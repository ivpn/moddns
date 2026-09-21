"""End-to-end check that DNS statistics are anonymous and service-wide.

Queries for a profile with the default settings must leave no per-profile
statistics document behind; the only counters written are one document per
PoP and hour in ``service_statistics`` with no profile, device or client
field.
specRef: proxy-statistics-behaviour #Y1 #Y5 #Y6 #Y7 #Y10.
"""

import os
import time

import pytest
from dns.rdatatype import A
from libs.constants import RESOLVABLE_TEST_DOMAIN
from libs.dns_lib import assert_not_blocked
from pymongo import MongoClient

MONGO_URI = os.getenv("MONGO_URI", "mongodb://admin:admin@localhost:27017/?authSource=admin")
MONGO_DB = os.getenv("MONGO_DB", "dns")

# Collector batch interval is 10s in this env (tests/config/proxy.env); poll past it.
STATS_POLL_TIMEOUT_S = 30
STATS_POLL_STEP_S = 2
QUERIES_TO_SEND = 5
EXPECTED_POP = "dev1"  # POP_NAME in tests/config/proxy.env
ALLOWED_FIELDS = {"_id", "timestamp", "pop", "queries"}
HOUR_S = 3600


@pytest.fixture(scope="module")
def mongo_db():
    client = MongoClient(MONGO_URI, serverSelectionTimeoutMS=5000)
    yield client[MONGO_DB]
    client.close()


def _service_total_since(db, since) -> int:
    agg = list(db.service_statistics.aggregate([
        {"$match": {"timestamp": {"$gte": since}}},
        {"$group": {"_id": None, "total": {"$sum": "$queries.total"}}},
    ]))
    return agg[0]["total"] if agg else 0


class TestServiceStatistics:
    @pytest.mark.asyncio
    async def test_queries_leave_no_per_profile_document(self, user, mongo_db):
        """specRef: proxy-statistics-behaviour #Y1 #Y5 #Y6 #Y7 — default
        settings: no document references the profile; service_statistics gains
        anonymous, PoP-labelled hourly counters in a single document."""
        pid = user.new_profile("service-stats")
        # One clock read: two reads straddle a sub-microsecond gap and round below the hour.
        now = time.time()
        hour_start = _utc(now - now % HOUR_S)
        since = hour_start
        baseline = _service_total_since(mongo_db, since)

        resp = await user.wait_for(pid, RESOLVABLE_TEST_DOMAIN, A, lambda m: len(m.answer) > 0)
        assert_not_blocked(resp)
        for _ in range(QUERIES_TO_SEND - 1):
            await user.resolve(pid, RESOLVABLE_TEST_DOMAIN, A)

        deadline = time.time() + STATS_POLL_TIMEOUT_S
        while time.time() < deadline:
            if _service_total_since(mongo_db, since) >= baseline + QUERIES_TO_SEND:
                break
            time.sleep(STATS_POLL_STEP_S)
        else:
            pytest.fail("service_statistics did not grow by the queries sent within the poll window")

        # Y1: nothing keyed by the profile, in the legacy collection or anywhere else.
        if "statistics" in mongo_db.list_collection_names():
            assert mongo_db.statistics.count_documents({"profile_id": pid}) == 0
        docs = list(mongo_db.service_statistics.find({"timestamp": {"$gte": since}}))
        # Y5 / Y6: one document per PoP and hour, keyed deterministically, hour start only.
        this_hour = [d for d in docs if d["pop"] == EXPECTED_POP and d["timestamp"].replace(tzinfo=None) == hour_start.replace(tzinfo=None)]
        assert len(this_hour) == 1, f"expected one document for {EXPECTED_POP} this hour, got {this_hour}"
        assert this_hour[0]["_id"] == f"{EXPECTED_POP}:{hour_start.strftime('%Y-%m-%dT%H')}"
        for doc in docs:
            # Y6 / Y7: shape and label.
            assert set(doc.keys()) == ALLOWED_FIELDS, f"unexpected fields in {doc}"
            assert set(doc["queries"].keys()) == {"total", "blocked", "dnssec"}
            assert doc["pop"] == EXPECTED_POP
            assert pid not in str(doc)
            ts = doc["timestamp"]
            assert (ts.minute, ts.second, ts.microsecond) == (0, 0, 0), f"sub-hour timestamp {ts}"

    def test_service_statistics_is_a_regular_collection_without_ttl(self, mongo_db):
        """specRef: proxy-statistics-behaviour #Y10 — a regular collection
        (upserts need it) with no TTL index."""
        info = list(mongo_db.list_collections(filter={"name": "service_statistics"}))
        assert len(info) == 1, "service_statistics is created by the first upsert"
        assert "timeseries" not in info[0].get("options", {}), "must stay a regular collection"
        assert info[0]["type"] == "collection"
        for index in mongo_db.service_statistics.list_indexes():
            assert "expireAfterSeconds" not in index, f"unexpected TTL index {index}"


def _utc(epoch: float):
    from datetime import datetime, timezone

    return datetime.fromtimestamp(epoch, tz=timezone.utc)
