"""Withdrawing consent and erasing stored data in every subscription state.

specRef: api-endpoint-behaviour #G27 #J3 #J52 #J6 #J12; subscription-lifecycle-enforcement (always allowed).
"""

from datetime import datetime, timedelta, timezone

import pytest
import requests
from bson import ObjectId
from libs.session import ProfileSession
from libs.settings import get_settings
from libs.statistics_helpers import STATS_COLLECTIONS
from pymongo import MongoClient

API = f"{get_settings().DNS_API_ADDR}/api/v1"

# Subscription fields that resolve to each status (see api/model/subscription.go GetStatus).
STATES = {
    "limited_access": {"active_until": -5, "updated_at": -0.04, "tier": "Tier 2"},
    "inactive": {"active_until": 30, "updated_at": -0.04, "tier": "Standard"},
    "pending_delete": {"active_until": 30, "updated_at": -0.04, "tier": "Tier 2", "retired": True},
}


@pytest.fixture(scope="module")
def mongo_db():
    c = MongoClient(get_settings().MONGO_URI, serverSelectionTimeoutMS=5000)
    yield c[get_settings().MONGO_DB]
    c.close()


def _patch(user, pid, updates):
    return requests.patch(f"{API}/profiles/{pid}", json={"updates": updates}, headers={"Cookie": user.cookie}, timeout=15)


def _replace(path, value):
    return {"operation": "replace", "path": path, "value": value}


def _seed_data(db, pid, retention):
    now = datetime.now(timezone.utc)
    counters = {"total": 3, "blocked": 1, "dnssec": 0}
    for coll in ("statistics_15min", "statistics_1d_30d"):
        db[coll].insert_one({"bucket_start": now.replace(minute=0, second=0, microsecond=0),
                             "meta": {"profile_id": pid, "device_id": ""}, **counters})
    db[f"query_logs_{retention}"].insert_one({
        "timestamp": now, "profile_id": pid, "device_id": "", "status": "processed", "reasons": [],
        "outcome": "resolved", "protocol": "doh",
        "dns_request": {"domain": "seeded.moddns-test.com", "query_type": "A", "response_code": "NOERROR", "dnssec": False},
    })


def _stats_count(db, pid):
    return sum(db[n].count_documents({"meta.profile_id": pid}) for n in STATS_COLLECTIONS)


def _logs_count(db, pid):
    return sum(db[f"query_logs_{r}"].count_documents({"profile_id": pid}) for r in ("1h", "6h", "1d", "1w", "1m"))


def _move_to(db, account_id, state):
    now = datetime.now(timezone.utc)
    s = STATES[state]
    update = {"active_until": now + timedelta(days=s["active_until"]),
              "updated_at": now + timedelta(days=s["updated_at"]), "tier": s["tier"]}
    if s.get("retired"):
        update["deletion_scheduled_at"] = now + timedelta(hours=48)
    db.subscriptions.update_one({"account_id": ObjectId(account_id)}, {"$set": update})


@pytest.fixture(params=list(STATES))
def collecting_user(request, mongo_db):
    """A fresh account whose profile collects statistics and logs, then moved into the state."""
    user = ProfileSession.create()
    pid = user.default_profile_id
    user.patch_setting(pid, "/settings/statistics/enabled", True)
    user.patch_setting(pid, "/settings/logs/enabled", True)
    _seed_data(mongo_db, pid, user.get_profile(pid).settings.logs.retention.value)
    _move_to(mongo_db, user.account.id, request.param)
    status = requests.get(f"{API}/sub", headers={"Cookie": user.cookie}, timeout=15).json()["status"]
    assert status == request.param
    yield user, pid, request.param
    user.cleanup()


class TestDataCollectionInEveryState:
    def test_turning_off_purges_in_every_state(self, collecting_user, mongo_db):
        """specRef: api-endpoint-behaviour #G27 #J6 #J12 — a turn-off-only PATCH
        passes the subscription guard and purges what it turns off."""
        user, pid, state = collecting_user
        assert _stats_count(mongo_db, pid) > 0 and _logs_count(mongo_db, pid) > 0

        resp = _patch(user, pid, [_replace("/settings/statistics/enabled", False)])
        assert resp.status_code == 200, f"{state}: {resp.text}"
        assert _stats_count(mongo_db, pid) == 0
        assert _logs_count(mongo_db, pid) > 0, "turning statistics off leaves the logs"

        # The generated Python client wraps values as {"value": x}; the guard unwraps them too.
        resp = _patch(user, pid, [_replace("/settings/logs/enabled", {"value": False})])
        assert resp.status_code == 200, f"{state}: {resp.text}"
        assert _logs_count(mongo_db, pid) == 0
        doc = mongo_db.profiles.find_one({"profile_id": pid})["settings"]
        assert doc["statistics"]["enabled"] is False and doc["logs"]["enabled"] is False

    def test_deleting_stored_data_in_every_state(self, collecting_user, mongo_db):
        """specRef: api-endpoint-behaviour #J3 #J52 — both deletes and the profile list are allowed."""
        user, pid, state = collecting_user
        headers = {"Cookie": user.cookie}
        assert requests.get(f"{API}/profiles", headers=headers, timeout=15).status_code == 200
        assert requests.delete(f"{API}/profiles/{pid}/statistics", headers=headers, timeout=15).status_code == 204
        assert _stats_count(mongo_db, pid) == 0
        assert requests.delete(f"{API}/profiles/{pid}/logs", headers=headers, timeout=15).status_code == 204
        assert _logs_count(mongo_db, pid) == 0

    def test_turning_on_and_other_changes_stay_blocked(self, collecting_user, mongo_db):
        """specRef: api-endpoint-behaviour #G27 — anything but a turn-off is the guard's 403."""
        user, pid, state = collecting_user
        blocked = [
            [_replace("/settings/statistics/retention", "1y")],
            [_replace("/settings/logs/log_domains", False)],
            [_replace("/settings/statistics/enabled", False), _replace("/name", "renamed")],
            [_replace("/settings/statistics/enabled", False), _replace("/settings/logs/enabled", True)],
        ]
        for updates in blocked:
            resp = _patch(user, pid, updates)
            assert resp.status_code == 403, f"{state} {updates}: {resp.status_code}"
            assert resp.json()["status"] == state
        assert requests.delete(f"{API}/profiles/{pid}", headers={"Cookie": user.cookie}, timeout=15).status_code == 403
        # Nothing was applied: both sources are still on and the data is still there.
        doc = mongo_db.profiles.find_one({"profile_id": pid})
        assert doc["settings"]["statistics"]["enabled"] is True and doc["name"] != "renamed"
        assert _stats_count(mongo_db, pid) > 0

        # Off, then on again: turning on stays blocked.
        assert _patch(user, pid, [_replace("/settings/statistics/enabled", False)]).status_code == 200
        assert _patch(user, pid, [_replace("/settings/statistics/enabled", True)]).status_code == 403
