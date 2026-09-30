"""End-to-end checks for consented per-profile statistics.

The proxy writes one document per (profile, device, 15-minute UTC bucket) into
the retention time-series collections only while ``statistics.enabled`` is true,
and only after the bucket closes or the proxy shuts down (Y18). Waiting 15
minutes is not an option, so tests force the flush by restarting the proxy
container.
specRef: proxy-statistics-behaviour #Y1 #Y11 #Y12 #Y13 #Y14 #Y18 #Y19 #Y20;
api-endpoint-behaviour #G7 #J6 #J7 #J8 #J9 #J10 #J11.
"""

import asyncio
import time
from datetime import datetime, timedelta, timezone

import docker
import pytest
import redis
from dns.rdatatype import A
from dns.query import tls as query_tls
from dns import message
from libs.constants import BLOCKLISTED_DOMAIN, RESOLVABLE_TEST_DOMAIN
from libs.dns_lib import LOCAL_PROXY_HOST, _dev_ca_path, is_blocked, is_resolved
from libs.profile_helpers import SVC_GOOGLE_DOMAIN
from libs.settings import get_settings
from pymongo import MongoClient

import moddns.api as api
import moddns.api_client as client
import moddns.configuration as api_config
from moddns import ModelProfileUpdate, RequestsProfileUpdates

PROXY_CONTAINER = "dnsproxy"
RETENTION_COLLECTIONS = ("statistics_30d", "statistics_90d", "statistics_1y")
BUCKET = timedelta(minutes=15)
PURGE_QUEUE = "statistics:purge:queue"
# Sweeper ticks every minute (J10); STATISTICS_PURGE_DELAY is 5s in config/api.env.
SWEEP_TIMEOUT_S = 120
# Replica lag between the API's settings write and the proxy's read.
SETTINGS_SETTLE_S = 2
TS_OPTIONS = {
    "statistics_30d": 30 * 86400,
    "statistics_90d": 90 * 86400,
    "statistics_1y": 365 * 86400,
}
DOC_KEYS = {"_id", "bucket_start", "meta", "queries", "reasons", "protocols"}
REASON_KEYS = {"blocklist", "service", "custom_rule", "rebinding", "default_rule", "other"}
PROTOCOL_KEYS = {"doh", "dot", "doq"}


@pytest.fixture(scope="module")
def mongo_db():
    c = MongoClient(get_settings().MONGO_URI, serverSelectionTimeoutMS=5000)
    yield c[get_settings().MONGO_DB]
    c.close()


@pytest.fixture(scope="module")
def redis_conn():
    cfg = get_settings()
    r = redis.Redis(host=cfg.REDIS_HOST, port=cfg.REDIS_PORT, db=0)
    yield r
    r.close()


def _docs(db, pid):
    return [d for name in RETENTION_COLLECTIONS for d in db[name].find({"meta.profile_id": pid})]


def _queued(r, prefix: str) -> list:
    """Purge-queue members starting with ``prefix`` (J9: ``disable:<pid>:<cutoff>``, ``delete:<pid>``)."""
    return [m for m in r.zrange(PURGE_QUEUE, 0, -1) if m.decode().startswith(prefix)]


def _count(db, pid) -> int:
    return sum(db[n].count_documents({"meta.profile_id": pid}) for n in RETENTION_COLLECTIONS)


async def _restart_proxy(user, pid_for_readiness):
    """Graceful restart: the collector flushes open buckets on shutdown (Y18)."""
    d = docker.from_env()
    try:
        d.containers.get(PROXY_CONTAINER).restart(timeout=30)
    finally:
        d.close()
    resp = await user.wait_for(
        pid_for_readiness, RESOLVABLE_TEST_DOMAIN, A, is_resolved, timeout=60.0, interval=1.0
    )
    assert is_resolved(resp), "proxy did not come back after restart"


def _patch_stats(user, pid, value):
    user.patch_setting(pid, "/settings/statistics/enabled", value)


def _dot_query(pid, device, domain):
    """DoT SNI carries the device as {device}-{profile}.<server name> (clientid.go)."""
    q = message.make_query(domain, A)
    return query_tls(
        q, LOCAL_PROXY_HOST, port=853,
        server_hostname=f"{device}-{pid}.moddns.dev", verify=_dev_ca_path(),
    )


def _is_bucket_boundary(ts: datetime) -> bool:
    return ts.minute % 15 == 0 and ts.second == 0 and ts.microsecond == 0


async def _enable_and_warm(user, pid):
    """Warm the profile with statistics still off so warm-up queries are never counted."""
    resp = await user.wait_for(pid, RESOLVABLE_TEST_DOMAIN, A, is_resolved)
    assert is_resolved(resp)
    _patch_stats(user, pid, True)
    await asyncio.sleep(SETTINGS_SETTLE_S)


class TestConsentedStatistics:
    def test_retention_collections_are_timeseries_with_ttl(self, mongo_db):
        """specRef: api-endpoint-behaviour #J11 / proxy-statistics-behaviour #Y21 —
        migration 027 options; legacy `statistics` is gone."""
        specs = {s["name"]: s for s in mongo_db.list_collections()}
        for name, ttl in TS_OPTIONS.items():
            assert name in specs, f"{name} missing"
            opts = specs[name]["options"]
            assert specs[name]["type"] == "timeseries"
            ts = opts["timeseries"]
            assert ts["timeField"] == "bucket_start"
            assert ts["metaField"] == "meta"
            assert ts["granularity"] == "minutes"
            assert opts["expireAfterSeconds"] == ttl
        assert "statistics" not in specs, "legacy statistics collection must be dropped"

    @pytest.mark.asyncio
    async def test_statistics_off_writes_no_profile_documents(self, user, mongo_db):
        """specRef: proxy-statistics-behaviour #Y1 #Y11 — default (off): queries
        with a device label leave no per-profile document after a forced flush,
        while the anonymous service_statistics still counts them."""
        pid = user.new_profile("stats-off")
        since = datetime.now(timezone.utc) - timedelta(hours=1)
        before = _fleet_total(mongo_db, since)
        resp = await user.wait_for(f"{pid}/laptop", RESOLVABLE_TEST_DOMAIN, A, is_resolved)
        assert is_resolved(resp)
        for _ in range(4):
            await user.resolve(f"{pid}/laptop", RESOLVABLE_TEST_DOMAIN, A)

        await _restart_proxy(user, pid)

        assert _count(mongo_db, pid) == 0
        deadline = time.monotonic() + 30
        while time.monotonic() < deadline and _fleet_total(mongo_db, since) < before + 5:
            await asyncio.sleep(2)
        assert _fleet_total(mongo_db, since) >= before + 5, "service_statistics must still count"
        assert _count(mongo_db, pid) == 0

    @pytest.mark.asyncio
    async def test_consented_documents_split_per_device_then_purged_on_toggle_off(
        self, user, mongo_db, redis_conn, ensure_test_blocklisted
    ):
        """specRef: proxy-statistics-behaviour #Y11 #Y12 #Y13 #Y14 #Y18 #Y19 #Y20;
        api-endpoint-behaviour #G7 #J6 #J8 #J9 #J10 — one document per device
        with exact counters, reason and protocol maps; toggling off removes them
        immediately and the delayed pass removes a late flush."""
        pid = user.new_profile("stats-on")
        user.add_rule(pid, "block", SVC_GOOGLE_DOMAIN)
        # Sync on the rule before enabling so measured queries are fully classified.
        resp = await user.wait_for(pid, SVC_GOOGLE_DOMAIN, A, is_blocked)
        assert is_blocked(resp)
        await _enable_and_warm(user, pid)

        # laptop (DoH): 3 allowed, 2 blocked by the profile blocklist.
        for _ in range(3):
            assert is_resolved(await user.resolve(f"{pid}/laptop", RESOLVABLE_TEST_DOMAIN, A))
        for _ in range(2):
            assert is_blocked(await user.resolve(f"{pid}/laptop", BLOCKLISTED_DOMAIN, A))
        # phone (DoH): 1 allowed, 1 blocked by a custom rule.
        assert is_resolved(await user.resolve(f"{pid}/phone", RESOLVABLE_TEST_DOMAIN, A))
        assert is_blocked(await user.resolve(f"{pid}/phone", SVC_GOOGLE_DOMAIN, A))
        # dotbox (DoT): 2 allowed.
        for _ in range(2):
            assert _dot_query(pid, "dotbox", RESOLVABLE_TEST_DOMAIN).answer

        await _restart_proxy(user, pid)

        docs = _docs(mongo_db, pid)
        assert docs, "no consented documents after the shutdown flush"
        only = {n: mongo_db[n].count_documents({"meta.profile_id": pid}) for n in RETENTION_COLLECTIONS}
        assert only["statistics_30d"] == len(docs), f"default retention is 30d: {only}"

        per_device = {}
        for d in docs:
            assert set(d.keys()) == DOC_KEYS, f"unexpected fields: {sorted(d.keys())}"
            assert set(d["meta"].keys()) == {"profile_id", "device_id"}
            assert set(d["queries"].keys()) == {"total", "blocked", "dnssec"}
            assert set(d["reasons"].keys()) == REASON_KEYS
            assert set(d["protocols"].keys()) == PROTOCOL_KEYS
            bs = d["bucket_start"].replace(tzinfo=timezone.utc)
            assert _is_bucket_boundary(bs), f"bucket_start not on a 15m boundary: {bs}"
            agg = per_device.setdefault(
                d["meta"]["device_id"],
                {"total": 0, "blocked": 0, "reasons": dict.fromkeys(REASON_KEYS, 0),
                 "protocols": dict.fromkeys(PROTOCOL_KEYS, 0)},
            )
            agg["total"] += d["queries"]["total"]
            agg["blocked"] += d["queries"]["blocked"]
            for k in REASON_KEYS:
                agg["reasons"][k] += d["reasons"][k]
            for k in PROTOCOL_KEYS:
                agg["protocols"][k] += d["protocols"][k]

        assert set(per_device) == {"laptop", "phone", "dotbox"}, sorted(per_device)
        laptop, phone, dotbox = per_device["laptop"], per_device["phone"], per_device["dotbox"]
        assert (laptop["total"], laptop["blocked"]) == (5, 2)
        assert laptop["reasons"] == {**dict.fromkeys(REASON_KEYS, 0), "blocklist": 2}
        assert laptop["protocols"] == {"doh": 5, "dot": 0, "doq": 0}
        assert (phone["total"], phone["blocked"]) == (2, 1)
        assert phone["reasons"] == {**dict.fromkeys(REASON_KEYS, 0), "custom_rule": 1}
        assert phone["protocols"] == {"doh": 2, "dot": 0, "doq": 0}
        assert (dotbox["total"], dotbox["blocked"]) == (2, 0)
        assert sum(dotbox["reasons"].values()) == 0
        assert dotbox["protocols"] == {"doh": 0, "dot": 2, "doq": 0}

        # J6: immediate purge from every retention collection.
        _patch_stats(user, pid, False)
        assert _count(mongo_db, pid) == 0, "documents must be gone as soon as the PATCH returns"

        # J8/J9: the delayed pass is queued, then removes a late flush.
        prefix = f"disable:{pid}:"
        assert _queued(redis_conn, prefix), "delayed purge not queued"
        late = datetime.now(timezone.utc).replace(second=0, microsecond=0)
        late -= timedelta(minutes=late.minute % 15)
        mongo_db.statistics_30d.insert_one({
            "bucket_start": late,
            "meta": {"profile_id": pid, "device_id": "laptop"},
            "queries": {"total": 1, "blocked": 0, "dnssec": 0},
            "reasons": dict.fromkeys(REASON_KEYS, 0),
            "protocols": {"doh": 1, "dot": 0, "doq": 0},
        })
        deadline = time.monotonic() + SWEEP_TIMEOUT_S
        while time.monotonic() < deadline and (
            _count(mongo_db, pid) or _queued(redis_conn, prefix)
        ):
            await asyncio.sleep(3)
        assert _count(mongo_db, pid) == 0, "delayed pass did not remove the late document"
        assert not _queued(redis_conn, prefix), "sweeper left the member queued"

    @pytest.mark.asyncio
    async def test_profile_delete_purges_documents(self, user, mongo_db, redis_conn):
        """specRef: api-endpoint-behaviour #J7 — deleting a profile removes its
        statistics documents from every retention collection."""
        pid = user.new_profile("stats-delete")
        await _enable_and_warm(user, pid)
        for _ in range(3):
            await user.resolve(f"{pid}/laptop", RESOLVABLE_TEST_DOMAIN, A)

        await _restart_proxy(user, pid)
        assert _count(mongo_db, pid) > 0, "expected consented documents before delete"

        with user.profiles_api() as p:
            resp = p.api_v1_profiles_id_delete_with_http_info(id=pid)
            assert resp.status_code in (200, 204), f"delete failed: {resp.status_code}"
        assert _count(mongo_db, pid) == 0
        assert _queued(redis_conn, f"delete:{pid}"), "delayed purge not queued"


def _fleet_total(db, since) -> int:
    agg = list(db.service_statistics.aggregate([
        {"$match": {"timestamp": {"$gte": since}}},
        {"$group": {"_id": None, "total": {"$sum": "$queries.total"}}},
    ]))
    return agg[0]["total"] if agg else 0
