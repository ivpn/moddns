"""End-to-end checks for the statistics read endpoints.

GET /profiles/{id}/statistics aggregates the consented per-profile tiers;
GET /profiles/{id}/logs/top and /logs/clients aggregate the query logs behind
the logs.enabled + log_domains / log_clients_ips gates. Traffic is real: DoH
and DoT queries through the proxy, with a proxy restart forcing the bucket
flush (see test_consented_statistics.py). Plain ``requests`` with the session
cookie is used so error statuses can be asserted directly.
specRef: api-endpoint-behaviour #J4 #J40 #J41 #J42 #J43 #J44 #J45 #J46
(statistics read), #J20 #J21 #J22 #J23 (logs top / clients).
"""

import asyncio
import ipaddress
import math
import time
from datetime import datetime, timezone

import pytest
import requests
from dns.rdatatype import A
from libs.constants import BLOCKLISTED_DOMAIN, RESOLVABLE_TEST_DOMAIN
from libs.dns_lib import is_blocked, is_resolved
from libs.profile_helpers import SVC_GOOGLE_DOMAIN
from libs.settings import get_settings
from libs.statistics_helpers import (
    PROTOCOL_KEYS,
    REASON_KEYS,
    SETTINGS_SETTLE_S,
    dot_query,
    enable_and_warm,
    parse_ts,
    patch_stats,
    restart_proxy,
)
from pymongo import MongoClient

TIMESPAN_BUCKET_SECONDS = {
    "LAST_3_HOURS": 900,
    "LAST_6_HOURS": 900,
    "LAST_1_DAY": 3600,
    "LAST_7_DAYS": 3600,
    "LAST_MONTH": 86400,
    "LAST_3_MONTHS": 86400,
    "LAST_YEAR": 86400,
}
# Retention is 30d until the retention setting exists, so longer windows clamp to it.
RETENTION_DAYS = 30
CLAMPED_TIMESPANS = {"LAST_3_MONTHS", "LAST_YEAR"}
LOGS_POLL_TIMEOUT_S = 40
LOGS_POLL_STEP_S = 2
LOG_TIMESPAN = "LAST_1_HOUR"
# Collector batch interval is 10s; reads are cached for 5 min (J23) and a partial
# first read would be pinned, so let one full batch tick pass before reading.
LOGS_FLUSH_WAIT_S = 12

BLOCK_A = "stats-read-a.moddns-test.com"
BLOCK_B = "stats-read-b.moddns-test.com"
BLOCK_C = "stats-read-c.moddns-test.com"


def _get(user, path: str, **params):
    """GET with the session cookie; returns the raw response."""
    return requests.get(
        f"{get_settings().DNS_API_ADDR}/api/v1/profiles/{path}",
        params={k: v for k, v in params.items() if v is not None},
        headers={"Cookie": user.cookie},
        timeout=15,
    )


def _get_json(user, path: str, **params) -> dict:
    resp = _get(user, path, **params)
    assert resp.status_code == 200, f"GET {path} {params}: {resp.status_code} {resp.text}"
    return resp.json()


def _assert_zero_stats(body: dict, enabled: bool) -> None:
    assert body["enabled"] is enabled
    assert body["totals"] == {"total": 0, "blocked": 0, "dnssec": 0}
    assert body["reasons"] == dict.fromkeys(REASON_KEYS, 0)
    assert body["protocols"] == dict.fromkeys(PROTOCOL_KEYS, 0)
    assert body["devices"] == []
    assert sum(p["total"] for p in body["series"]) == 0


@pytest.fixture(scope="module")
def mongo_db():
    c = MongoClient(get_settings().MONGO_URI, serverSelectionTimeoutMS=5000)
    yield c[get_settings().MONGO_DB]
    c.close()


class TestStatisticsRead:
    @pytest.mark.asyncio
    async def test_statistics_off_returns_empty_shape(self, user):
        """specRef: api-endpoint-behaviour #J42 — statistics off: 200 with enabled
        false, null enabled_at, empty series and devices, zero maps, and the
        window fields still set."""
        pid = user.new_profile("read-off")
        body = _get_json(user, f"{pid}/statistics")
        _assert_zero_stats(body, enabled=False)
        assert body["series"] == []
        assert body["enabled_at"] is None
        assert body["timespan"] == "LAST_7_DAYS", "LAST_7_DAYS is the default"
        assert body["bucket_seconds"] == 3600
        assert body["retention"] == "30d"
        assert body["from"] and body["to"]

    @pytest.mark.parametrize("timespan", sorted(TIMESPAN_BUCKET_SECONDS))
    def test_every_timespan_is_accepted_with_its_bucket_width(self, user, timespan):
        """specRef: api-endpoint-behaviour #J40 — the seven statistics timespans and
        the point width of the tier each one reads."""
        pid = user.new_profile("read-spans")
        body = _get_json(user, f"{pid}/statistics", timespan=timespan)
        assert body["timespan"] == timespan
        assert body["bucket_seconds"] == TIMESPAN_BUCKET_SECONDS[timespan]

    @pytest.mark.parametrize(
        "timespan", ["LAST_1_HOUR", "LAST_12_HOURS", "LAST_2_DAYS", "last_7_days"]
    )
    def test_invalid_timespan_is_rejected(self, user, timespan):
        """specRef: api-endpoint-behaviour #J40 — values outside the statistics enum,
        including the logs-only LAST_1_HOUR / LAST_12_HOURS, are 400."""
        pid = user.new_profile("read-badspan")
        assert _get(user, f"{pid}/statistics", timespan=timespan).status_code == 400

    @pytest.mark.asyncio
    async def test_aggregates_real_traffic_then_toggle_leaves_no_stale_counts(
        self, user, ensure_test_blocklisted
    ):
        """specRef: api-endpoint-behaviour #J4 #J41 #J43 #J44 #J45 #J46 — totals,
        zero-filled series, reason, protocol and device breakdown of real traffic
        for every timespan; the 60s cache never serves counts after a disable/enable
        cycle."""
        pid = user.new_profile("read-on")
        user.add_rule(pid, "block", SVC_GOOGLE_DOMAIN)
        resp = await user.wait_for(pid, SVC_GOOGLE_DOMAIN, A, is_blocked)
        assert is_blocked(resp)
        await enable_and_warm(user, pid)

        # laptop (DoH): 3 allowed, 2 blocklist. phone (DoH): 1 allowed, 1 custom rule.
        for _ in range(3):
            assert is_resolved(await user.resolve(f"{pid}/laptop", RESOLVABLE_TEST_DOMAIN, A))
        for _ in range(2):
            assert is_blocked(await user.resolve(f"{pid}/laptop", BLOCKLISTED_DOMAIN, A))
        assert is_resolved(await user.resolve(f"{pid}/phone", RESOLVABLE_TEST_DOMAIN, A))
        assert is_blocked(await user.resolve(f"{pid}/phone", SVC_GOOGLE_DOMAIN, A))
        # dotbox (DoT): 2 allowed.
        for _ in range(2):
            assert (await asyncio.to_thread(dot_query, pid, "dotbox", RESOLVABLE_TEST_DOMAIN)).answer
        await restart_proxy(user)

        default = _get_json(user, f"{pid}/statistics")
        assert default["timespan"] == "LAST_7_DAYS", "LAST_7_DAYS is the default"
        assert default["totals"]["total"] == 9, "the 1-hour tier must still hold the flushed hour"

        for timespan, width in TIMESPAN_BUCKET_SECONDS.items():
            body = _get_json(user, f"{pid}/statistics", timespan=timespan)
            assert body["enabled"] is True
            assert body["enabled_at"] is not None
            assert body["retention"] == "30d"
            assert body["timespan"] == timespan
            assert body["bucket_seconds"] == width
            # dnssec counts AD-flagged answers (Y4): recursor-dependent, so only bounded.
            assert 0 <= body["totals"]["dnssec"] <= 6
            assert {k: v for k, v in body["totals"].items() if k != "dnssec"} == {"total": 9, "blocked": 3}
            assert body["totals"]["dnssec"] == default["totals"]["dnssec"]
            assert body["reasons"] == {**dict.fromkeys(REASON_KEYS, 0), "blocklist": 2, "custom_rule": 1}
            assert body["protocols"] == {"doh": 7, "dot": 2, "doq": 0}

            # Window: from is floored to the point width; 3m/1y clamp to retention.
            start, end = parse_ts(body["from"]), parse_ts(body["to"])
            assert int(start.timestamp()) % width == 0, "from is floored to the point width"
            if timespan in CLAMPED_TIMESPANS:
                assert (end - start).days in (RETENTION_DAYS, RETENTION_DAYS + 1)

            # Series: zero-filled, oldest first, evenly spaced, ceil((to-from)/width)
            # points, adds up to the totals.
            series = body["series"]
            stamps = [parse_ts(p["ts"]) for p in series]
            assert stamps == sorted(stamps)
            assert len(series) == math.ceil((end - start).total_seconds() / width)
            assert stamps[0] == start
            assert {(b - a).total_seconds() for a, b in zip(stamps, stamps[1:])} == {width}
            assert sum(p["total"] for p in series) == 9
            assert sum(p["blocked"] for p in series) == 3
            assert any(p["total"] == 0 for p in series), "empty points must be zero-filled"

            devices = body["devices"]
            keys = [(-d["total"], d["device_id"]) for d in devices]
            assert keys == sorted(keys), "devices sort by total desc, then id asc"
            assert {d["device_id"]: (d["total"], d["blocked"]) for d in devices} == {
                "laptop": (5, 2),
                "phone": (2, 1),
                "dotbox": (2, 0),
            }

        # J41: a seven-day read spans 168 or 169 points depending on grid alignment.
        assert len(default["series"]) in (168, 169)

        # Cache: a repeat inside 60s is identical.
        assert _get_json(user, f"{pid}/statistics") == default

        # Disable: no stale counts despite the cached response.
        patch_stats(user, pid, False)
        for timespan in TIMESPAN_BUCKET_SECONDS:
            _assert_zero_stats(_get_json(user, f"{pid}/statistics", timespan=timespan), enabled=False)
        # Enable again: the purged history stays gone and the cache holds nothing old.
        patch_stats(user, pid, True)
        await asyncio.sleep(SETTINGS_SETTLE_S)
        for timespan in TIMESPAN_BUCKET_SECONDS:
            again = _get_json(user, f"{pid}/statistics", timespan=timespan)
            _assert_zero_stats(again, enabled=True)
            assert again["enabled_at"] is not None


class TestLogsTopAndClients:
    @staticmethod
    async def _poll(user, path, predicate, **params):
        """Poll until ``predicate(body)`` holds and return the body."""
        deadline = time.monotonic() + LOGS_POLL_TIMEOUT_S
        body = _get_json(user, path, **params)
        while time.monotonic() < deadline and not predicate(body):
            await asyncio.sleep(LOGS_POLL_STEP_S)
            body = _get_json(user, path, **params)
        return body

    def test_top_parameters_are_validated(self, user):
        """specRef: api-endpoint-behaviour #J20 — kind is required and limited to
        blocked|resolved; limit is 1..50; timespan must be in the logs enum."""
        pid = user.new_profile("read-top-params")
        assert _get(user, f"{pid}/logs/top").status_code == 400
        assert _get(user, f"{pid}/logs/top", kind="allowed").status_code == 400
        for limit in (0, 51, -1):
            assert _get(user, f"{pid}/logs/top", kind="blocked", limit=limit).status_code == 400
        assert _get(user, f"{pid}/logs/top", kind="blocked", timespan="LAST_YEAR").status_code == 400
        for limit in (1, 50):
            assert _get(user, f"{pid}/logs/top", kind="blocked", limit=limit).status_code == 200
        # Default timespan is LAST_1_DAY; the gate is closed here so the body is the closed shape.
        assert _get_json(user, f"{pid}/logs/top", kind="resolved") == {"enabled": False, "items": []}
        assert _get(user, f"{pid}/logs/clients", limit=0).status_code == 400

    def test_non_numeric_limit_is_rejected(self, user):
        """specRef: api-endpoint-behaviour #J20 #J21 — anything but an integer in
        1..50 is 400, including a non-numeric value."""
        pid = user.new_profile("read-limit-x")
        assert _get(user, f"{pid}/logs/top", kind="blocked", limit="x").status_code == 400
        assert _get(user, f"{pid}/logs/clients", limit="x").status_code == 400

    @pytest.mark.asyncio
    async def test_top_domains_and_clients_follow_the_log_gates(self, user, mongo_db):
        """specRef: api-endpoint-behaviour #J20 #J21 #J22 #J23 — counts per domain
        (blocked vs resolved) and per client IP ordered by count and limited; each
        endpoint is gated by logs.enabled plus log_domains / log_clients_ips; GeoIP
        enrichment of a public IP, nulls for private ones."""
        pid = user.new_profile("read-logs")
        for domain in (BLOCK_A, BLOCK_B, BLOCK_C):
            user.add_rule(pid, "block", domain)
        # Sync on the rules before logging starts so warm-up queries are not logged.
        assert is_blocked(await user.wait_for(pid, BLOCK_C, A, is_blocked))
        assert is_resolved(await user.wait_for(pid, RESOLVABLE_TEST_DOMAIN, A, is_resolved))

        # Gates closed by default: logging is off.
        assert _get_json(user, f"{pid}/logs/top", timespan=LOG_TIMESPAN, kind="blocked") == {
            "enabled": False, "items": []}
        assert _get_json(user, f"{pid}/logs/clients", timespan=LOG_TIMESPAN)["enabled"] is False

        user.patch_setting(pid, "/settings/logs/enabled", True)
        user.patch_setting(pid, "/settings/logs/log_domains", True)
        user.patch_setting(pid, "/settings/logs/log_clients_ips", True)
        await asyncio.sleep(SETTINGS_SETTLE_S)

        # Blocked: A x3, B x2, C x1. Resolved: test.com x2.
        for domain, n in ((BLOCK_A, 3), (BLOCK_B, 2), (BLOCK_C, 1)):
            for _ in range(n):
                assert is_blocked(await user.resolve(pid, domain, A))
        for _ in range(2):
            assert is_resolved(await user.resolve(pid, RESOLVABLE_TEST_DOMAIN, A))
        real_logged = 3 + 2 + 1 + 2

        # Three log rows from a public client IP the GeoIP stubs know (8.8.8.8).
        retention = user.get_profile(pid).settings.logs.retention
        for _ in range(3):
            mongo_db[f"query_logs_{retention}"].insert_one({
                "timestamp": datetime.now(timezone.utc),
                "profile_id": pid,
                "device_id": "",
                "status": "processed",
                "reasons": [],
                "outcome": "resolved",
                "dns_request": {
                    "domain": "seeded.moddns-test.com", "query_type": "A",
                    "response_code": "NOERROR", "dnssec": False,
                },
                "client_ip": "8.8.8.8",
                "protocol": "doh",
            })

        # Reads are cached for 5 min and a partial first answer would be pinned.
        await asyncio.sleep(LOGS_FLUSH_WAIT_S)

        blocked = await self._poll(
            user, f"{pid}/logs/top",
            lambda b: [i["count"] for i in b.get("items", [])] == [3, 2, 1],
            timespan=LOG_TIMESPAN, kind="blocked",
        )
        assert blocked["enabled"] is True
        assert [(i["domain"].rstrip("."), i["count"]) for i in blocked["items"]] == [
            (BLOCK_A, 3), (BLOCK_B, 2), (BLOCK_C, 1)]

        limited = _get_json(user, f"{pid}/logs/top", timespan=LOG_TIMESPAN, kind="blocked", limit=2)
        assert [(i["domain"].rstrip("."), i["count"]) for i in limited["items"]] == [
            (BLOCK_A, 3), (BLOCK_B, 2)]

        resolved = await self._poll(
            user, f"{pid}/logs/top",
            lambda b: any(i["domain"].rstrip(".") == RESOLVABLE_TEST_DOMAIN and i["count"] == 2
                          for i in b.get("items", [])),
            timespan=LOG_TIMESPAN, kind="resolved",
        )
        counts = {i["domain"].rstrip("."): i["count"] for i in resolved["items"]}
        assert counts[RESOLVABLE_TEST_DOMAIN] == 2
        assert counts["seeded.moddns-test.com"] == 3
        assert BLOCK_A not in counts
        keys = [(-i["count"], i["domain"]) for i in resolved["items"]]
        assert keys == sorted(keys), "count desc, then domain asc"

        clients = await self._poll(
            user, f"{pid}/logs/clients",
            lambda b: sum(i["count"] for i in b.get("items", [])) == real_logged + 3,
            timespan=LOG_TIMESPAN,
        )
        assert clients["enabled"] is True
        by_ip = {i["ip"]: i for i in clients["items"]}
        for item in clients["items"]:
            ipaddress.ip_address(item["ip"])
            assert {"asn", "as_org", "country"} <= item.keys()
        # Enrichment: the stub databases know 8.8.8.8; private client addresses get nulls.
        google = by_ip["8.8.8.8"]
        assert google["count"] == 3
        assert (google["asn"], google["as_org"], google["country"]) == (15169, "GOOGLE", "US")
        private = [i for i in clients["items"] if ipaddress.ip_address(i["ip"]).is_private]
        assert private and sum(i["count"] for i in private) == real_logged
        for item in private:
            assert (item["asn"], item["as_org"], item["country"]) == (None, None, None)
        keys = [(-i["count"], i["ip"]) for i in clients["items"]]
        assert keys == sorted(keys), "count desc, then ip asc"
        assert len(_get_json(user, f"{pid}/logs/clients", timespan=LOG_TIMESPAN, limit=1)["items"]) == 1

        # Gates: each endpoint closes on its own toggle.
        user.patch_setting(pid, "/settings/logs/log_domains", False)
        await asyncio.sleep(SETTINGS_SETTLE_S)
        assert _get_json(user, f"{pid}/logs/top", timespan=LOG_TIMESPAN, kind="blocked") == {
            "enabled": False, "items": []}
        assert _get_json(user, f"{pid}/logs/clients", timespan=LOG_TIMESPAN)["enabled"] is True

        user.patch_setting(pid, "/settings/logs/log_clients_ips", False)
        await asyncio.sleep(SETTINGS_SETTLE_S)
        assert _get_json(user, f"{pid}/logs/clients", timespan=LOG_TIMESPAN) == {
            "enabled": False, "items": []}
