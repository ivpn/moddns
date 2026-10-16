"""End-to-end checks for consented per-profile statistics.

The proxy writes flat documents per (profile, device, bucket) into the tiered
time-series collections only while ``statistics.enabled`` is true: 15-minute
entries when a quarter closes, and at that point the hour's counts so far as an
additive 1-hour measurement and a 1-day one (stamped at the UTC day start).
Waiting for a quarter to close is not an option, so tests force the flush by
restarting the proxy container, which flushes partial buckets on shutdown (Y18).
specRef: proxy-statistics-behaviour #Y11 #Y12 #Y13 #Y14 #Y17 #Y18 #Y19 #Y20;
api-endpoint-behaviour #J6 #J7 #J8 #J9 #J11.
"""

import asyncio
import time
from datetime import datetime, timedelta, timezone

import pytest
from dns.rdatatype import A
from libs.constants import BLOCKLISTED_DOMAIN, RESOLVABLE_TEST_DOMAIN
from libs.dns_lib import is_blocked, is_resolved
from libs.profile_helpers import SVC_GOOGLE_DOMAIN
from libs.settings import get_settings
from libs.statistics_helpers import (
    COUNTER_KEYS,
    DAY_TIERS,
    STATS_COLLECTIONS,
    TIER_15MIN,
    TIER_1H,
    dot_query,
    enable_and_warm,
    parse_ts,
    patch_stats,
    restart_proxy,
)
from pymongo import MongoClient

# STATISTICS_PURGE_INTERVAL is 5s in config/api.env; leave room for Mongo latency.
PURGE_TIMEOUT_S = 60
TS_TTL = {
    "statistics_15min": 86400,
    "statistics_1h": 691200,
    "statistics_1d_30d": 2592000,
    "statistics_1d_90d": 7776000,
    "statistics_1d_1y": 31536000,
}
LEGACY_COLLECTIONS = ("statistics", "statistics_30d", "statistics_90d", "statistics_1y")
DOC_KEYS = {"_id", "bucket_start", "meta"} | COUNTER_KEYS
INDEX_NAME = "meta_profile_id_bucket_start"


@pytest.fixture(scope="module")
def mongo_db():
    c = MongoClient(get_settings().MONGO_URI, serverSelectionTimeoutMS=5000)
    yield c[get_settings().MONGO_DB]
    c.close()


def _docs(db, pid, collection):
    return list(db[collection].find({"meta.profile_id": pid}))


def _count(db, pid) -> int:
    return sum(db[n].count_documents({"meta.profile_id": pid}) for n in STATS_COLLECTIONS)


def _utc(ts: datetime) -> datetime:
    return ts.replace(tzinfo=timezone.utc)


def _floor15(ts: datetime) -> datetime:
    ts = _utc(ts).replace(second=0, microsecond=0)
    return ts - timedelta(minutes=ts.minute % 15)


def _floor_hour(ts: datetime) -> datetime:
    return _utc(ts).replace(minute=0, second=0, microsecond=0)


def _floor_day(ts: datetime) -> datetime:
    return _utc(ts).replace(hour=0, minute=0, second=0, microsecond=0)


def _insert_doc(db, pid, bucket_start, device="laptop", collection=TIER_15MIN):
    doc = dict.fromkeys(COUNTER_KEYS, 0)
    doc.update({
        "bucket_start": bucket_start,
        "meta": {"profile_id": pid, "device_id": device},
        "total": 1,
        "proto_doh": 1,
    })
    db[collection].insert_one(doc)


async def _wait_until(cond, timeout=PURGE_TIMEOUT_S, step=2):
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline and not cond():
        await asyncio.sleep(step)
    return cond()


def _sum_by_device(docs):
    """Sum the flat counters of ``docs`` per device id."""
    per_device = {}
    for d in docs:
        agg = per_device.setdefault(d["meta"]["device_id"], dict.fromkeys(COUNTER_KEYS, 0))
        for k in COUNTER_KEYS:
            agg[k] += d[k]
    return per_device


class TestConsentedStatistics:
    def test_tier_collections_are_timeseries_with_ttl_and_index(self, mongo_db):
        """specRef: api-endpoint-behaviour #J11 / proxy-statistics-behaviour #Y21 —
        migration 027: five time-series collections with fixed 1-day buckets, the
        per-tier TTLs and the profile index; every legacy collection is gone."""
        specs = {s["name"]: s for s in mongo_db.list_collections()}
        for name, ttl in TS_TTL.items():
            assert name in specs, f"{name} missing"
            assert specs[name]["type"] == "timeseries"
            opts = specs[name]["options"]
            ts = opts["timeseries"]
            assert ts["timeField"] == "bucket_start"
            assert ts["metaField"] == "meta"
            assert ts["bucketMaxSpanSeconds"] == 86400
            assert ts["bucketRoundingSeconds"] == 86400
            assert "granularity" not in ts
            assert opts["expireAfterSeconds"] == ttl
            indexes = {i["name"]: i for i in mongo_db[name].list_indexes()}
            assert INDEX_NAME in indexes, f"{name}: index {INDEX_NAME} missing"
            assert dict(indexes[INDEX_NAME]["key"]) == {"meta.profile_id": 1, "bucket_start": 1}
        for legacy in LEGACY_COLLECTIONS:
            assert legacy not in specs, f"legacy collection {legacy} must be absent"

    @pytest.mark.asyncio
    async def test_statistics_off_writes_no_profile_documents(self, user, mongo_db):
        """specRef: proxy-statistics-behaviour #Y1 #Y11 — default (off): queries
        with a device label leave no per-profile document in any tier after a
        forced flush, while the anonymous service_statistics still counts them."""
        pid = user.new_profile("stats-off")
        since = datetime.now(timezone.utc) - timedelta(hours=1)
        before = _fleet_total(mongo_db, since)
        resp = await user.wait_for(f"{pid}/laptop", RESOLVABLE_TEST_DOMAIN, A, is_resolved)
        assert is_resolved(resp)
        for _ in range(4):
            await user.resolve(f"{pid}/laptop", RESOLVABLE_TEST_DOMAIN, A)

        await restart_proxy(user)

        assert _count(mongo_db, pid) == 0
        deadline = time.monotonic() + 30
        while time.monotonic() < deadline and _fleet_total(mongo_db, since) < before + 5:
            await asyncio.sleep(2)
        assert _fleet_total(mongo_db, since) >= before + 5, "service_statistics must still count"
        assert _count(mongo_db, pid) == 0

    @pytest.mark.asyncio
    async def test_consented_documents_per_tier_then_purged_on_toggle_off(
        self, user, mongo_db, ensure_test_blocklisted
    ):
        """specRef: proxy-statistics-behaviour #Y11 #Y12 #Y13 #Y14 #Y17 #Y18 #Y19 #Y20;
        api-endpoint-behaviour #J6 #J8 — one flat document per device in each of the
        15-minute, 1-hour and 1-day (30d) tiers with exact counters; toggling off
        removes them from every tier at once and the statistics reconcile removes a late flush."""
        pid = user.new_profile("stats-on")
        user.add_rule(pid, "block", SVC_GOOGLE_DOMAIN)
        # Sync on the rule before enabling so measured queries are fully classified.
        resp = await user.wait_for(pid, SVC_GOOGLE_DOMAIN, A, is_blocked)
        assert is_blocked(resp)
        await enable_and_warm(user, pid)

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
            assert (await asyncio.to_thread(dot_query, pid, "dotbox", RESOLVABLE_TEST_DOMAIN)).answer

        await restart_proxy(user)

        tiers = {name: _docs(mongo_db, pid, name) for name in STATS_COLLECTIONS}
        for name in (TIER_15MIN, TIER_1H, "statistics_1d_30d"):
            assert tiers[name], f"no documents in {name} after the shutdown flush"
        # Retention routes the 1-day tier only; the default goes to the 30d collection.
        assert tiers["statistics_1d_90d"] == [] and tiers["statistics_1d_1y"] == []

        for name, docs in tiers.items():
            for d in docs:
                assert set(d.keys()) == DOC_KEYS, f"{name}: unexpected fields {sorted(d.keys())}"
                assert set(d["meta"].keys()) == {"profile_id", "device_id"}
                assert all(isinstance(d[k], int) for k in COUNTER_KEYS), f"{name}: counters must be ints"
        for d in tiers[TIER_15MIN]:
            assert _utc(d["bucket_start"]) == _floor15(d["bucket_start"]), "15min bucket_start"
        for d in tiers[TIER_1H]:
            assert _utc(d["bucket_start"]) == _floor_hour(d["bucket_start"]), "1h bucket_start"
        for d in tiers["statistics_1d_30d"]:
            assert _utc(d["bucket_start"]) == _floor_day(d["bucket_start"]), "1d bucket_start is the UTC day start"

        # One document per device per tier bucket; a boundary crossed during the test
        # legitimately adds one more per crossed boundary.
        per_device_15 = _sum_by_device(tiers[TIER_15MIN])
        for device in per_device_15:
            hours = {_floor_hour(d["bucket_start"]) for d in tiers[TIER_15MIN] if d["meta"]["device_id"] == device}
            days = {_floor_day(d["bucket_start"]) for d in tiers[TIER_15MIN] if d["meta"]["device_id"] == device}
            assert sum(d["meta"]["device_id"] == device for d in tiers[TIER_1H]) == len(hours), device
            assert sum(d["meta"]["device_id"] == device for d in tiers["statistics_1d_30d"]) == len(days), device

        # Every tier carries the same totals.
        expected = {
            "laptop": {"total": 5, "blocked": 2, "reason_blocklist": 2, "proto_doh": 5},
            "phone": {"total": 2, "blocked": 1, "reason_custom_rule": 1, "proto_doh": 2},
            "dotbox": {"total": 2, "blocked": 0, "proto_dot": 2},
        }
        # dnssec counts answers carrying the AD flag (Y4), so it depends on the
        # recursor: assert it is consistent across tiers and within bounds.
        dnssec_15 = {dev: agg["dnssec"] for dev, agg in per_device_15.items()}
        for name in (TIER_15MIN, TIER_1H, "statistics_1d_30d"):
            per_device = _sum_by_device(tiers[name])
            assert set(per_device) == set(expected), (name, sorted(per_device))
            for device, want in expected.items():
                got = dict(per_device[device])
                assert got.pop("dnssec") == dnssec_15[device], (name, device)
                assert 0 <= dnssec_15[device] <= want["total"] - want["blocked"], (name, device)
                assert got == {**dict.fromkeys(COUNTER_KEYS - {"dnssec"}, 0), **want}, (name, device)

        # J6: immediate purge from every tier.
        patch_stats(user, pid, False)
        assert _count(mongo_db, pid) == 0, "documents must be gone as soon as the PATCH returns"

        # J8: a flush landing after the immediate purge is removed by the statistics reconcile.
        now = datetime.now(timezone.utc)
        _insert_doc(mongo_db, pid, _floor15(now), collection=TIER_15MIN)
        _insert_doc(mongo_db, pid, _floor_hour(now), collection=TIER_1H)
        _insert_doc(mongo_db, pid, _floor_day(now), collection="statistics_1d_90d")
        assert await _wait_until(lambda: _count(mongo_db, pid) == 0), (
            "statistics reconcile did not remove the late documents of a statistics-off profile"
        )

    @pytest.mark.asyncio
    async def test_reconcile_keeps_only_buckets_since_reenable(self, user, mongo_db):
        """specRef: api-endpoint-behaviour #J8 #J9 — for a statistics-on profile the
        statistics reconcile deletes, per tier, buckets older than enabled_at floored to that
        tier's width (15 min / 1 h / 1 day) and keeps buckets at or after it."""
        pid = user.new_profile("stats-reenable")
        await enable_and_warm(user, pid)
        patch_stats(user, pid, False)
        patch_stats(user, pid, True)
        raw = user.get_profile(pid).settings.statistics.enabled_at
        assert raw is not None
        enabled_at = parse_ts(raw)
        bounds = (
            (TIER_15MIN, _floor15(enabled_at), timedelta(minutes=15)),
            (TIER_1H, _floor_hour(enabled_at), timedelta(hours=1)),
            (DAY_TIERS[0], _floor_day(enabled_at), timedelta(days=1)),
        )

        # Inserted after the re-enable so the profile is never statistics-off
        # while the statistics reconcile can see these documents.
        for tier, cutoff, width in bounds:
            _insert_doc(mongo_db, pid, cutoff - width, device="old", collection=tier)
            _insert_doc(mongo_db, pid, cutoff, device="new", collection=tier)

        def old_gone():
            return sum(
                mongo_db[t].count_documents({"meta.profile_id": pid, "meta.device_id": "old"})
                for t, _, _ in bounds
            ) == 0

        assert await _wait_until(old_gone), "a bucket before its tier's cutoff survived"
        for tier, _, _ in bounds:
            assert mongo_db[tier].count_documents(
                {"meta.profile_id": pid, "meta.device_id": "new"}) == 1, (
                f"{tier}: bucket at the tier-floored enabled_at must be kept"
            )

    @pytest.mark.asyncio
    async def test_reconcile_keeps_flushed_hour_and_day_tier_docs_of_an_enabled_profile(
        self, user, mongo_db
    ):
        """specRef: api-endpoint-behaviour #J8 #J9 / proxy-statistics-behaviour #Y17 #Y18 —
        a profile that keeps statistics on keeps the 1-hour and 1-day documents of the
        current period across purge runs (their bucket_start is the hour / day start,
        earlier than floor15(enabled_at))."""
        pid = user.new_profile("stats-keep-tiers")
        await enable_and_warm(user, pid)
        for _ in range(3):
            await user.resolve(f"{pid}/laptop", RESOLVABLE_TEST_DOMAIN, A)
        await restart_proxy(user)
        before = {n: _docs(mongo_db, pid, n) for n in (TIER_15MIN, TIER_1H, "statistics_1d_30d")}
        assert all(before.values()), {n: len(d) for n, d in before.items()}

        # Several purge runs (5s interval) later nothing may be gone.
        await asyncio.sleep(20)
        after = {n: _docs(mongo_db, pid, n) for n in before}
        assert {n: len(d) for n, d in after.items()} == {n: len(d) for n, d in before.items()}

    @pytest.mark.asyncio
    async def test_profile_delete_purges_documents(self, user, mongo_db):
        """specRef: api-endpoint-behaviour #J7 #J8 — deleting a profile removes its
        statistics from every tier at once; the statistics reconcile removes a late flush."""
        pid = user.new_profile("stats-delete")
        await enable_and_warm(user, pid)
        for _ in range(3):
            await user.resolve(f"{pid}/laptop", RESOLVABLE_TEST_DOMAIN, A)

        await restart_proxy(user)
        for name in (TIER_15MIN, TIER_1H, "statistics_1d_30d"):
            assert _docs(mongo_db, pid, name), f"expected consented documents in {name} before delete"

        with user.profiles_api() as p:
            resp = p.api_v1_profiles_id_delete_with_http_info(id=pid)
            assert resp.status_code in (200, 204), f"delete failed: {resp.status_code}"
        assert _count(mongo_db, pid) == 0

        now = datetime.now(timezone.utc)
        _insert_doc(mongo_db, pid, _floor15(now), collection=TIER_15MIN)
        _insert_doc(mongo_db, pid, _floor_day(now), collection="statistics_1d_1y")
        assert await _wait_until(lambda: _count(mongo_db, pid) == 0), (
            "statistics reconcile did not remove the late documents of a deleted profile"
        )


def _fleet_total(db, since) -> int:
    agg = list(db.service_statistics.aggregate([
        {"$match": {"timestamp": {"$gte": since}}},
        {"$group": {"_id": None, "total": {"$sum": "$queries.total"}}},
    ]))
    return agg[0]["total"] if agg else 0
