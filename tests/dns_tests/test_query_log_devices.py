"""End-to-end test for the query-log device list endpoint.

Queries arrive over DoH with and without a device segment in the path
(`/dns-query/{profile_id}/{device_id}`); the collector batches log rows to
Mongo; GET /profiles/{id}/logs/devices returns the distinct non-empty device
ids with last-seen timestamps, sorted ascending. specRef: api-endpoint-behaviour #J5.
"""

import time

import pytest
from libs.dns_lib import DNSLib
from libs.settings import get_settings
from dns.rdatatype import A

import moddns.api_client as client
import moddns.api as api
import moddns.configuration as api_config
from moddns import (
    ApiCreateProfileBody,
    RequestsProfileUpdates,
    ModelProfileUpdate,
)

RESOLVED_DOMAIN = "test.com"  # pinned in testhosts

# Collector batch interval is 10s in this env; poll a little past it.
LOGS_POLL_TIMEOUT_S = 30
LOGS_POLL_STEP_S = 2


class TestQueryLogDevices:
    def setup_class(self):
        self.config = get_settings()
        self.api_config = api_config.Configuration(host=self.config.DNS_API_ADDR)
        self.dns_lib = DNSLib(self.config.DOH_ENDPOINT)

    def _patch(self, profiles_instance, profile_id, path, value):
        body = RequestsProfileUpdates(
            updates=[ModelProfileUpdate(operation="replace", path=path, value={"value": value})]
        )
        resp = profiles_instance.api_v1_profiles_id_patch_with_http_info(profile_id, body=body)
        assert resp.status_code == 200, f"PATCH {path} failed: {resp.status_code}"

    @pytest.mark.asyncio
    async def test_distinct_devices_listed_sorted_with_last_seen(self, create_account_and_login):
        """specRef: api-endpoint-behaviour #J5 — distinct non-empty device ids,
        sorted, with last_seen; device-less queries are excluded."""
        account, cookie = create_account_and_login
        with client.ApiClient(self.api_config) as api_client:
            profiles_instance = api.ProfileApi(api_client)
            profiles_instance.api_client.default_headers["Cookie"] = cookie
            logs_instance = api.QueryLogsApi(api_client)

            resp = profiles_instance.api_v1_profiles_post_with_http_info(
                body=ApiCreateProfileBody(name="devices")
            )
            assert resp.status_code == 201
            profile_id = resp.data.profile_id

            self._patch(profiles_instance, profile_id, "/settings/logs/enabled", True)

            # The DoH path carries the device id as an extra segment after the
            # profile id; the third query has no device segment at all.
            await self.dns_lib.send_doh_request(f"{profile_id}/laptop", RESOLVED_DOMAIN, A)
            await self.dns_lib.send_doh_request(f"{profile_id}/phone", RESOLVED_DOMAIN, A)
            await self.dns_lib.send_doh_request(profile_id, RESOLVED_DOMAIN, A)

            # Logs are batched (10s); poll until both device ids appear.
            deadline = time.monotonic() + LOGS_POLL_TIMEOUT_S
            devices = []
            while time.monotonic() < deadline:
                resp = logs_instance.api_v1_profiles_id_logs_devices_get_with_http_info(id=profile_id)
                assert resp.status_code == 200, f"devices fetch failed: {resp.status_code}"
                devices = resp.data or []
                if {d.device_id for d in devices} >= {"laptop", "phone"}:
                    break
                time.sleep(LOGS_POLL_STEP_S)

            ids = [d.device_id for d in devices]
            assert ids == sorted(ids), f"device ids not sorted: {ids}"
            assert set(ids) == {"laptop", "phone"}, (
                f"expected exactly laptop+phone (device-less query excluded), got {ids}"
            )
            for d in devices:
                assert d.last_seen is not None, f"last_seen missing for {d.device_id}"

            # The device-less query still landed in the logs themselves — its
            # exclusion above is the endpoint's doing, not a lost query.
            logs_resp = logs_instance.api_v1_profiles_id_logs_get_with_http_info(id=profile_id)
            assert logs_resp.status_code == 200
            assert len(logs_resp.data or []) >= 3
