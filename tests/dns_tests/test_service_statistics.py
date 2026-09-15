"""End-to-end check that DNS statistics are anonymous and service-wide.

Queries for a profile with the default settings must leave no per-profile
statistics document behind; the only counters written are per-PoP documents
in ``service_statistics`` (one per collector flush) with no profile, device
or client field.
specRef: proxy-statistics-behaviour #Y1 #Y6 #Y7 #Y10.
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
        """specRef: proxy-statistics-behaviour #Y1 #Y6 #Y7 — default
        settings: no document references the profile; service_statistics gains
        anonymous, PoP-labelled counters."""
        pid = user.new_profile("service-stats")
        since = _utc(time.time() - 60)
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
        for doc in mongo_db.service_statistics.find({"timestamp": {"$gte": since}}):
            # Y6 / Y7: shape and label.
            assert set(doc.keys()) == ALLOWED_FIELDS, f"unexpected fields in {doc}"
            assert set(doc["queries"].keys()) == {"total", "blocked", "dnssec"}
            assert doc["pop"] == EXPECTED_POP
            assert pid not in str(doc)

    def test_service_statistics_is_a_pop_keyed_time_series_without_ttl(self, mongo_db):
        """specRef: proxy-statistics-behaviour #Y10 — collection options."""
        info = list(mongo_db.list_collections(filter={"name": "service_statistics"}))
        assert len(info) == 1, "service_statistics is created by the proxy at startup"
        options = info[0]["options"]
        assert options["timeseries"]["timeField"] == "timestamp"
        assert options["timeseries"]["metaField"] == "pop"
        assert options["timeseries"]["granularity"] == "minutes"
        assert "expireAfterSeconds" not in options


def _utc(epoch: float):
    from datetime import datetime, timezone

    return datetime.fromtimestamp(epoch, tz=timezone.utc)
