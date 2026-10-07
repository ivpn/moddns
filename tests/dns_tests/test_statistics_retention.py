"""End-to-end checks for the per-profile statistics retention (#713).

specRef: api-endpoint-behaviour #G26 #J48 #J50 #J51 #J52 #J53; proxy-statistics-behaviour #Y20.
"""

from datetime import datetime, timedelta, timezone

import asyncio
import time

import pytest
import requests
from dns.rdatatype import A
from libs.constants import RESOLVABLE_TEST_DOMAIN
from libs.dns_lib import is_resolved
from libs.settings import get_settings
from libs.statistics_helpers import COUNTER_KEYS, DAY_TIERS, TIER_15MIN, TIER_1H, patch_stats, restart_proxy
from pymongo import MongoClient


@pytest.fixture(scope="module")
def mongo_db():
    c = MongoClient(get_settings().MONGO_URI, serverSelectionTimeoutMS=5000)
    yield c[get_settings().MONGO_DB]
    c.close()


def _patch_retention(user, pid, value):
    # Raw request: the generated clients learn the path in the API-sync commit.
    return requests.patch(
        f"{get_settings().DNS_API_ADDR}/api/v1/profiles/{pid}",
        json={"updates": [{"operation": "replace", "path": "/settings/statistics/retention", "value": value}]},
        headers={"Cookie": user.cookie}, timeout=15,
    )


def _set_retention(user, pid, value):
    resp = _patch_retention(user, pid, value)
    assert resp.status_code == 200, resp.text


def _retention(db, pid):
    # Read from Mongo: the generated clients learn the field in the API-sync commit.
    return db.profiles.find_one({"profile_id": pid})["settings"]["statistics"].get("retention")


def _count(db, pid, collection) -> int:
    return db[collection].count_documents({"meta.profile_id": pid})


def _docs(db, pid, collection):
    return {d["_id"]: d for d in db[collection].find({"meta.profile_id": pid})}


def _insert_day(db, pid, collection, day, total):
    doc = dict.fromkeys(COUNTER_KEYS, 0)
    doc.update({"bucket_start": day, "meta": {"profile_id": pid, "device_id": "laptop"}, "total": total, "proto_doh": total})
    return db[collection].insert_one(doc).inserted_id


def _month_total(user, pid) -> int:
    resp = requests.get(
        f"{get_settings().DNS_API_ADDR}/api/v1/profiles/{pid}/statistics",
        params={"timespan": "LAST_MONTH"}, headers={"Cookie": user.cookie}, timeout=15,
    )
    assert resp.status_code == 200, resp.text
    return resp.json()["totals"]["total"]


async def _enable(user, pid):
    """Turn statistics on, then restart the proxy so no cached settings predate the change."""
    patch_stats(user, pid, True)
    await restart_proxy(user)


async def _traffic(user, pid, n):
    for _ in range(n):
        assert is_resolved(await user.resolve(f"{pid}/laptop", RESOLVABLE_TEST_DOMAIN, A))


class TestStatisticsRetention:
    def test_new_profile_defaults_to_30d(self, user, mongo_db):
        """specRef: api-endpoint-behaviour #J48"""
        pid = user.new_profile("ret-default")
        assert _retention(mongo_db, pid) == "30d"

    def test_unknown_retention_is_rejected(self, user, mongo_db):
        """specRef: api-endpoint-behaviour #G26"""
        pid = user.new_profile("ret-invalid")
        for value in ("1m", "", "365d"):
            assert _patch_retention(user, pid, value).status_code == 400, value
        assert _retention(mongo_db, pid) == "30d"

    @pytest.mark.asyncio
    async def test_1y_routes_daily_documents_to_the_1y_collection(self, user, mongo_db):
        """specRef: api-endpoint-behaviour #G26 #J48; proxy-statistics-behaviour #Y20 —
        with 1y the day tier goes to statistics_1d_1y; the 15-minute and hourly tiers are unchanged."""
        pid = user.new_profile("ret-1y")
        _set_retention(user, pid, "1y")
        assert _retention(mongo_db, pid) == "1y"
        await _enable(user, pid)
        for _ in range(3):
            assert is_resolved(await user.resolve(f"{pid}/laptop", RESOLVABLE_TEST_DOMAIN, A))

        await restart_proxy(user)

        assert _count(mongo_db, pid, "statistics_1d_1y") > 0
        for other in DAY_TIERS:
            if other != "statistics_1d_1y":
                assert _count(mongo_db, pid, other) == 0, other
        assert _count(mongo_db, pid, TIER_15MIN) > 0
        assert _count(mongo_db, pid, TIER_1H) > 0

    @pytest.mark.asyncio
    async def test_lowering_to_30d_moves_recent_days_and_empties_the_1y_collection(self, user, mongo_db):
        """specRef: api-endpoint-behaviour #J50 — 1y → 30d: the last 30 days move unchanged into
        statistics_1d_30d, older days are dropped, statistics_1d_1y holds nothing for the profile."""
        pid = user.new_profile("ret-lower")
        _set_retention(user, pid, "1y")
        await _enable(user, pid)
        await _traffic(user, pid, 3)
        await restart_proxy(user)
        today = datetime.now(timezone.utc).replace(hour=0, minute=0, second=0, microsecond=0)
        old = _insert_day(mongo_db, pid, "statistics_1d_1y", today - timedelta(days=200), 50)
        recent = _insert_day(mongo_db, pid, "statistics_1d_1y", today - timedelta(days=10), 5)
        before = _docs(mongo_db, pid, "statistics_1d_1y")
        assert len(before) >= 3, {c: _count(mongo_db, pid, c) for c in (TIER_15MIN, TIER_1H, *DAY_TIERS)}

        _set_retention(user, pid, "30d")

        assert _docs(mongo_db, pid, "statistics_1d_1y") == {}
        after = _docs(mongo_db, pid, "statistics_1d_30d")
        assert old not in after
        for _id, doc in before.items():
            if _id != old:
                assert after.get(_id) == doc, "moved documents are unchanged"
        assert recent in after

    @pytest.mark.asyncio
    async def test_raising_after_traffic_keeps_the_chart_continuous(self, user, mongo_db):
        """specRef: api-endpoint-behaviour #J51 — 30d → 1y: earlier days stay in statistics_1d_30d,
        new days go to statistics_1d_1y, and the month view sums both."""
        pid = user.new_profile("ret-raise")
        await _enable(user, pid)
        await _traffic(user, pid, 2)
        await restart_proxy(user)
        assert _docs(mongo_db, pid, "statistics_1d_30d")
        first = _month_total(user, pid)
        assert first >= 2

        _set_retention(user, pid, "1y")
        # A restart drops the proxy's cached settings, so the next writes route by the new value.
        await restart_proxy(user)
        await _traffic(user, pid, 3)
        await restart_proxy(user)

        assert _docs(mongo_db, pid, "statistics_1d_30d"), "earlier days stay in the shorter collection"
        assert _docs(mongo_db, pid, "statistics_1d_1y"), "new days go to the longer collection"
        assert _month_total(user, pid) >= first + 3

    @pytest.mark.asyncio
    async def test_delete_history_empties_every_tier_and_keeps_the_setting(self, user, mongo_db):
        """specRef: api-endpoint-behaviour #J52 #J53 — DELETE removes every bucket incl. the current
        ones, statistics stay on with their retention, stragglers flushed afterwards into the deleted
        buckets are purged, and buckets after the delete survive."""
        pid = user.new_profile("ret-delete")
        _set_retention(user, pid, "90d")
        await _enable(user, pid)
        await _traffic(user, pid, 2)
        await restart_proxy(user)
        assert _count(mongo_db, pid, TIER_15MIN) > 0

        resp = requests.delete(
            f"{get_settings().DNS_API_ADDR}/api/v1/profiles/{pid}/statistics",
            headers={"Cookie": user.cookie}, timeout=15,
        )
        assert resp.status_code == 204, resp.text

        for c in (TIER_15MIN, TIER_1H, *DAY_TIERS):
            assert _count(mongo_db, pid, c) == 0, c
        stats = mongo_db.profiles.find_one({"profile_id": pid})["settings"]["statistics"]
        assert stats["enabled"] is True and stats["retention"] == "90d"
        assert stats.get("history_deleted_at") is not None
        assert _month_total(user, pid) == 0

        # A straggler the proxy flushes into the deleted quarter, and a bucket after the delete.
        deleted_at = stats["history_deleted_at"].replace(tzinfo=timezone.utc)
        quarter = deleted_at.replace(minute=deleted_at.minute - deleted_at.minute % 15, second=0, microsecond=0)
        straggler = _insert_day(mongo_db, pid, TIER_15MIN, quarter, 4)
        later = _insert_day(mongo_db, pid, TIER_15MIN, quarter + timedelta(minutes=15), 1)
        # STATISTICS_PURGE_INTERVAL is 5s in config/api.env.
        deadline = time.monotonic() + 60
        while time.monotonic() < deadline and _docs(mongo_db, pid, TIER_15MIN).get(straggler):
            await asyncio.sleep(2)
        remaining = _docs(mongo_db, pid, TIER_15MIN)
        assert straggler not in remaining, "the straggler in the deleted bucket is purged"
        assert later in remaining, "a bucket after the delete is kept"
