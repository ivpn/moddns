"""End-to-end tests for query-log resolution outcomes.

Validates the proxy-computed `outcome` field on query-log entries over the real
DoH + logs-API path: the proxy classifies each answer at log emission
(`classifyOutcome`), the collector batches it to Mongo (10s in this env), and
the logs endpoint returns it.

Covered rows (docs/specs/query-log-outcomes-behaviour.md): O1 resolved,
O2 nodata, O3 nxdomain, O4 blocked. Transport rows (O7 timeout / O8
network_error) are unit-tested only — they cannot be produced deterministically
in the compose stack. specRef: O1, O2, O3, O4.
"""

import time

import pytest
from libs.dns_lib import DNSLib
from libs.settings import get_settings
from dns.rdatatype import A, AAAA

import moddns.api_client as client
import moddns.api as api
import moddns.configuration as api_config
from moddns import (
    ApiCreateProfileBody,
    RequestsProfileUpdates,
    ModelProfileUpdate,
)

# Pinned in config/testhosts.txt + knot local-data.
BLOCKED_DOMAIN = "rebinding-private-v4.com"   # A -> 192.168.0.10 (blocked when rebinding on)
RESOLVED_DOMAIN = "test.com"                  # A -> 104.18.74.230
# RFC 6761 reserved TLD -> deterministic NXDOMAIN from any recursor.
NXDOMAIN_DOMAIN = "definitely-missing.invalid"

# Collector batch interval is 10s in this env; poll a little past it.
LOGS_POLL_TIMEOUT_S = 30
LOGS_POLL_STEP_S = 2


class TestQueryLogOutcomes:
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

    def _fetch_outcomes(self, logs_instance, profile_id):
        """Return {(domain, qtype): outcome} for the profile's current logs."""
        resp = logs_instance.api_v1_profiles_id_logs_get_with_http_info(id=profile_id)
        assert resp.status_code == 200, f"logs fetch failed: {resp.status_code}"
        out = {}
        for entry in resp.data or []:
            req = entry.dns_request
            domain = (req.domain or "").rstrip(".") if req else ""
            qtype = req.query_type if req else ""
            out[(domain, qtype)] = entry.outcome
        return out

    @pytest.mark.asyncio
    async def test_outcomes_recorded_per_answer_state(self, create_account_and_login):
        """specRef: O1, O2, O3, O4 — resolved/nodata/nxdomain/blocked outcomes
        land on the matching query-log entries."""
        account, cookie = create_account_and_login
        with client.ApiClient(self.api_config) as api_client:
            profiles_instance = api.ProfileApi(api_client)
            profiles_instance.api_client.default_headers["Cookie"] = cookie
            logs_instance = api.QueryLogsApi(api_client)

            body = ApiCreateProfileBody(name="outcomes")
            resp = profiles_instance.api_v1_profiles_post_with_http_info(body=body)
            assert resp.status_code == 201
            profile_id = resp.data.profile_id

            self._patch(profiles_instance, profile_id, "/settings/logs/enabled", True)
            self._patch(
                profiles_instance, profile_id,
                "/settings/security/rebinding_protection/enabled", True,
            )

            # Give the 1ms-TTL settings cache + Redis replica a beat, then fire
            # one query per expected outcome row.
            expected = {
                (BLOCKED_DOMAIN, "A"): "blocked",       # O4
                (BLOCKED_DOMAIN, "AAAA"): "nodata",     # O2 — testhosts pins A only
                (RESOLVED_DOMAIN, "A"): "resolved",     # O1
                (NXDOMAIN_DOMAIN, "A"): "nxdomain",     # O3
            }
            await self.dns_lib.send_doh_request(profile_id, BLOCKED_DOMAIN, A)
            await self.dns_lib.send_doh_request(profile_id, BLOCKED_DOMAIN, AAAA)
            await self.dns_lib.send_doh_request(profile_id, RESOLVED_DOMAIN, A)
            await self.dns_lib.send_doh_request(profile_id, NXDOMAIN_DOMAIN, A)

            # Logs are batched (10s); poll until all four entries are present.
            deadline = time.monotonic() + LOGS_POLL_TIMEOUT_S
            outcomes = {}
            while time.monotonic() < deadline:
                outcomes = self._fetch_outcomes(logs_instance, profile_id)
                if all(key in outcomes for key in expected):
                    break
                time.sleep(LOGS_POLL_STEP_S)

            for key, want in expected.items():
                assert key in outcomes, f"log entry for {key} never appeared; got {outcomes}"
                assert outcomes[key] == want, (
                    f"outcome mismatch for {key}: want {want!r}, got {outcomes[key]!r}"
                )
