"""End-to-end checks for changing the query-logs retention (#748): logs follow the statistics model.

specRef: api-endpoint-behaviour #G11 #J1 #J13 #J14 #J54 #J55 #J56.
"""

import time
from datetime import datetime, timedelta, timezone

import pytest
import requests
from bson import ObjectId
from libs.settings import get_settings
from pymongo import MongoClient

API = f"{get_settings().DNS_API_ADDR}/api/v1"
RETENTIONS = ("1h", "6h", "1d", "1w", "1m")


@pytest.fixture(scope="module")
def mongo_db():
    c = MongoClient(get_settings().MONGO_URI, serverSelectionTimeoutMS=5000)
    yield c[get_settings().MONGO_DB]
    c.close()


def _insert(db, pid, retention, age, domain):
    """One query-log row as the proxy writes it, `age` old, straight into a retention collection."""
    return db[f"query_logs_{retention}"].insert_one({
        "_id": ObjectId(), "timestamp": datetime.now(timezone.utc) - age, "profile_id": pid, "device_id": "",
        "status": "processed", "reasons": [], "outcome": "resolved", "protocol": "doh", "client_ip": "",
        "dns_request": {"domain": domain, "query_type": "A", "response_code": "NOERROR", "dnssec": False},
    }).inserted_id


def _ids(db, pid, retention):
    return {d["_id"] for d in db[f"query_logs_{retention}"].find({"profile_id": pid}, {"_id": 1})}


def _listed_domains(user, pid):
    resp = requests.get(f"{API}/profiles/{pid}/logs", params={"page": 1, "limit": 100, "timespan": "LAST_MONTH"},
                        headers={"Cookie": user.cookie}, timeout=15)
    assert resp.status_code == 200, resp.text
    return [row["dns_request"]["domain"] for row in resp.json()]


def _logs_on(user, pid, retention):
    user.patch_setting(pid, "/settings/logs/retention", retention)
    user.patch_setting(pid, "/settings/logs/enabled", True)


def _wait(predicate, timeout=30.0):
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        if predicate():
            return True
        time.sleep(0.5)
    return predicate()


class TestQueryLogsRetention:
    def test_raising_keeps_existing_logs_listed(self, user, mongo_db):
        """specRef: api-endpoint-behaviour #J55 #J56 #J1 — 1h → 1m: rows written under 1h stay listed,
        newest first together with rows of the new retention."""
        pid = user.new_profile("ql-raise")
        _logs_on(user, pid, "1h")
        _insert(mongo_db, pid, "1h", timedelta(minutes=10), "before-raise.moddns-test.com")

        user.patch_setting(pid, "/settings/logs/retention", "1m")
        _insert(mongo_db, pid, "1m", timedelta(minutes=1), "after-raise.moddns-test.com")

        assert _listed_domains(user, pid) == ["after-raise.moddns-test.com", "before-raise.moddns-test.com"]

    def test_lowering_deletes_older_rows_and_keeps_recent_ones_in_place(self, user, mongo_db):
        """specRef: api-endpoint-behaviour #J54 #J56 — 1m → 1h: the PATCH deletes the rows older than an
        hour from every longer collection; rows from the last hour stay where they were written and are listed."""
        pid = user.new_profile("ql-lower")
        _logs_on(user, pid, "1m")
        recent = _insert(mongo_db, pid, "1m", timedelta(minutes=20), "recent.moddns-test.com")
        _insert(mongo_db, pid, "1m", timedelta(days=3), "old.moddns-test.com")
        recent_week = _insert(mongo_db, pid, "1w", timedelta(minutes=30), "recent-week.moddns-test.com")
        _insert(mongo_db, pid, "1d", timedelta(hours=5), "old-day.moddns-test.com")

        user.patch_setting(pid, "/settings/logs/retention", "1h")

        stored = {r: _ids(mongo_db, pid, r) for r in RETENTIONS}
        assert stored == {"1h": set(), "6h": set(), "1d": set(), "1w": {recent_week}, "1m": {recent}}, stored
        assert sorted(_listed_domains(user, pid)) == ["recent-week.moddns-test.com", "recent.moddns-test.com"]

    def test_turning_logs_off_while_lowering_purges_everything(self, user, mongo_db):
        """specRef: api-endpoint-behaviour #J12 #J54 — a PATCH that lowers the retention and turns logs
        off deletes every row, the recent ones included."""
        pid = user.new_profile("ql-lower-off")
        _logs_on(user, pid, "1m")
        _insert(mongo_db, pid, "1m", timedelta(minutes=20), "recent.moddns-test.com")

        resp = requests.patch(f"{API}/profiles/{pid}", headers={"Cookie": user.cookie}, timeout=15, json={"updates": [
            {"operation": "replace", "path": "/settings/logs/retention", "value": "1h"},
            {"operation": "replace", "path": "/settings/logs/enabled", "value": False},
        ]})
        assert resp.status_code == 200, resp.text
        assert _wait(lambda: all(not _ids(mongo_db, pid, r) for r in RETENTIONS))

    def test_purge_deletes_rows_older_than_the_retention_left_in_a_longer_collection(self, user, mongo_db):
        """specRef: api-endpoint-behaviour #J13 #J14 #J56 — rows that reach a longer collection after the
        retention changed (the proxy's settings cache lags the switch) stay there while inside the retention;
        the purge deletes the ones older than it, which reads never list. QUERY_LOGS_PURGE_INTERVAL is 5s
        in config/api.env."""
        pid = user.new_profile("ql-purge")
        _logs_on(user, pid, "1d")
        late = _insert(mongo_db, pid, "1w", timedelta(minutes=5), "late.moddns-test.com")
        _insert(mongo_db, pid, "1w", timedelta(days=2), "stale.moddns-test.com")

        assert _listed_domains(user, pid) == ["late.moddns-test.com"]
        assert _wait(lambda: _ids(mongo_db, pid, "1w") == {late}), {r: _ids(mongo_db, pid, r) for r in RETENTIONS}
        assert _listed_domains(user, pid) == ["late.moddns-test.com"]
