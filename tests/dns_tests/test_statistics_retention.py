"""End-to-end checks for the per-profile statistics retention (#713).

specRef: api-endpoint-behaviour #G26 #J48; proxy-statistics-behaviour #Y20.
"""

import pytest
from dns.rdatatype import A
from libs.constants import RESOLVABLE_TEST_DOMAIN
from libs.dns_lib import is_resolved
from libs.settings import get_settings
from libs.statistics_helpers import DAY_TIERS, TIER_15MIN, TIER_1H, enable_and_warm, restart_proxy
from moddns.exceptions import ApiException
from moddns.models.model_profile_update import ModelProfileUpdate
from moddns.models.requests_profile_updates import RequestsProfileUpdates
from pymongo import MongoClient


@pytest.fixture(scope="module")
def mongo_db():
    c = MongoClient(get_settings().MONGO_URI, serverSelectionTimeoutMS=5000)
    yield c[get_settings().MONGO_DB]
    c.close()


def _retention(db, pid):
    # Read from Mongo: the generated clients learn the field in the API-sync commit.
    return db.profiles.find_one({"profile_id": pid})["settings"]["statistics"].get("retention")


def _count(db, pid, collection) -> int:
    return db[collection].count_documents({"meta.profile_id": pid})


class TestStatisticsRetention:
    def test_new_profile_defaults_to_30d(self, user, mongo_db):
        """specRef: api-endpoint-behaviour #J48"""
        pid = user.new_profile("ret-default")
        assert _retention(mongo_db, pid) == "30d"

    def test_unknown_retention_is_rejected(self, user, mongo_db):
        """specRef: api-endpoint-behaviour #G26"""
        pid = user.new_profile("ret-invalid")
        with user.profiles_api() as p:
            body = RequestsProfileUpdates(updates=[
                ModelProfileUpdate(operation="replace", path="/settings/statistics/retention", value={"value": "1m"}),
            ])
            with pytest.raises(ApiException) as exc:
                p.api_v1_profiles_id_patch(pid, body=body)
        assert exc.value.status == 400
        assert _retention(mongo_db, pid) == "30d"

    @pytest.mark.asyncio
    async def test_1y_routes_daily_documents_to_the_1y_collection(self, user, mongo_db):
        """specRef: api-endpoint-behaviour #G26 #J48; proxy-statistics-behaviour #Y20 —
        with 1y the day tier goes to statistics_1d_1y; the 15-minute and hourly tiers are unchanged."""
        pid = user.new_profile("ret-1y")
        user.patch_setting(pid, "/settings/statistics/retention", "1y")
        assert _retention(mongo_db, pid) == "1y"
        await enable_and_warm(user, pid)
        for _ in range(3):
            assert is_resolved(await user.resolve(f"{pid}/laptop", RESOLVABLE_TEST_DOMAIN, A))

        await restart_proxy(user)

        assert _count(mongo_db, pid, "statistics_1d_1y") > 0
        for other in DAY_TIERS:
            if other != "statistics_1d_1y":
                assert _count(mongo_db, pid, other) == 0, other
        assert _count(mongo_db, pid, TIER_15MIN) > 0
        assert _count(mongo_db, pid, TIER_1H) > 0
