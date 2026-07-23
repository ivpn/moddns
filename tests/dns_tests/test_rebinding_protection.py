"""End-to-end integration tests for DNS rebinding protection.

These tests validate the *integration seam* that the Go unit tests
(`proxy/filter/rebinding_test.go`, rows R1-R12) cannot exercise: the API client
PATCHes `/settings/security/rebinding_protection/enabled`, the API writes the
`settings:<id>:security:rebinding_protection` Redis hash, the proxy batch-fetch
reads it into the request context, and the IP-phase filter (TierRebinding, T150)
produces the right answer over the real DoH wire path.

Determinism comes from `config/testhosts.txt`, which maps public-looking names to
private IPs via the sdns hostsfile (sdns returns them unmodified). The proxy master
switch defaults ON in the test env (`REBINDING_PROTECTION_ENABLED` unset), and
`PROFILE_SETTINGS_CACHE_TTL=1ms` makes per-profile toggles visible on the next query.

Deliberately out of scope here (covered by Go unit tests, not drivable through the
IPv4-only hostsfile / build-time env): IPv6 ranges (::1, fc00::/7, fe80::/10), the
::ffff: IPv4-mapped unwrap, HTTPS/SVCB ipv4hint, and the env-gated CGNAT (100.64/10)
and NAT64 (64:ff9b::/96) ranges.

specRef rows refer to docs/specs/proxy-filtering-behaviour.md Section E (R1-R12).
"""

import pytest
from libs.dns_lib import DNSLib
from libs.settings import get_settings
from dns.rdatatype import A

import moddns.api_client as client
import moddns.api as api
import moddns.configuration as api_config
from moddns import (
    RequestsProfileUpdates,
    ModelProfileUpdate,
    RequestsCreateProfileCustomRuleBody,
    ApiCreateProfileBody,
)

# testhosts.txt mappings (public name -> private IP) used by these tests.
PRIVATE_V4_DOMAIN = "rebinding-private-v4.com"
PRIVATE_V4_IP = "192.168.0.10"
LOOPBACK_DOMAIN = "rebinding-loopback.com"
ALLOW_RULE_DOMAIN = "rebinding-allow-rule.com"
ALLOW_RULE_IP = "192.168.0.20"
ALLOW_SUFFIX_DOMAIN = "router.local"  # ends in .local -> operator allow-suffix
ALLOW_SUFFIX_IP = "192.168.0.30"
PUBLIC_DOMAIN = "svctest-google.com"  # -> 8.8.8.8 (public), already in testhosts.txt
PUBLIC_IP = "8.8.8.8"

BLOCKED_A = "0.0.0.0"


class TestRebindingProtection:
    """End-to-end tests for the per-profile DNS rebinding protection toggle.

    Each test creates an isolated profile to avoid cross-test interference.
    """

    def setup_class(self):
        self.config = get_settings()
        self.api_config = api_config.Configuration(host=self.config.DNS_API_ADDR)
        self.dns_lib = DNSLib(self.config.DOH_ENDPOINT)

    def _create_profile(self, profiles_instance, name):
        """Create a new profile and return its ID."""
        body = ApiCreateProfileBody(name=name)
        resp = profiles_instance.api_v1_profiles_post_with_http_info(body=body)
        assert (
            resp.status_code == 201
        ), f"Failed to create profile with status code: {resp.status_code}"
        return resp.data.profile_id

    def _create_custom_rule(self, profiles_instance, profile_id, action, value):
        """Create a custom rule (action: 'allow' | 'block') on a profile."""
        custom_rule_body = RequestsCreateProfileCustomRuleBody(
            action=action, value=value
        )
        resp = profiles_instance.api_v1_profiles_id_custom_rules_post_with_http_info(
            id=profile_id, body=custom_rule_body
        )
        assert (
            resp.status_code == 201
        ), f"Custom rule creation failed for {value} with status code: {resp.status_code}"
        return resp

    def _set_rebinding_protection(self, profiles_instance, profile_id, enabled: bool):
        """Toggle settings.security.rebinding_protection.enabled via PATCH."""
        update_request = RequestsProfileUpdates(
            updates=[
                ModelProfileUpdate(
                    operation="replace",
                    path="/settings/security/rebinding_protection/enabled",
                    # Dict[string, Any] is an openapi-cli-gen limitation — the Go
                    # 'interface{}' type is generated as Dict[string, Any].
                    value={"value": enabled},
                )
            ]
        )
        resp = profiles_instance.api_v1_profiles_id_patch_with_http_info(
            profile_id, body=update_request
        )
        assert (
            resp.status_code == 200
        ), f"Profile rebinding_protection update failed with status code: {resp.status_code}"
        return resp

    @staticmethod
    def _answer_ip(resp):
        """Return the first A-answer IP as a string."""
        assert resp.answer, "Expected a DNS answer section"
        return resp.answer[0].to_text().split(" ")[-1]

    @pytest.mark.asyncio
    async def test_default_off_passes_private_ip(self, create_account_and_login):
        """specRef: R5 — opt-in default OFF: a fresh profile resolves a private IP
        normally (rebinding protection is not enabled)."""
        account, cookie = create_account_and_login
        with client.ApiClient(self.api_config) as api_client:
            profiles_instance = api.ProfileApi(api_client)
            profiles_instance.api_client.default_headers["Cookie"] = cookie
            profile_id = self._create_profile(
                profiles_instance, "rebinding_default_off"
            )

            resp = await self.dns_lib.send_doh_request(profile_id, PRIVATE_V4_DOMAIN, A)
            ip = self._answer_ip(resp)
            assert ip == PRIVATE_V4_IP, (
                f"Default-off profile should resolve {PRIVATE_V4_DOMAIN} to "
                f"{PRIVATE_V4_IP}, got {ip}"
            )

    @pytest.mark.asyncio
    async def test_enabled_blocks_private_192168(self, create_account_and_login):
        """specRef: R1 — enabled: a public name resolving to 192.168.x is blocked
        (A -> 0.0.0.0)."""
        account, cookie = create_account_and_login
        with client.ApiClient(self.api_config) as api_client:
            profiles_instance = api.ProfileApi(api_client)
            profiles_instance.api_client.default_headers["Cookie"] = cookie
            profile_id = self._create_profile(
                profiles_instance, "rebinding_block_192168"
            )
            self._set_rebinding_protection(profiles_instance, profile_id, True)

            resp = await self.dns_lib.send_doh_request(profile_id, PRIVATE_V4_DOMAIN, A)
            ip = self._answer_ip(resp)
            assert ip == BLOCKED_A, (
                f"Expected {PRIVATE_V4_DOMAIN} blocked to {BLOCKED_A}, got {ip}"
            )

    @pytest.mark.asyncio
    async def test_enabled_blocks_loopback(self, create_account_and_login):
        """specRef: R1 — enabled: a public name resolving to 127.0.0.1 is blocked."""
        account, cookie = create_account_and_login
        with client.ApiClient(self.api_config) as api_client:
            profiles_instance = api.ProfileApi(api_client)
            profiles_instance.api_client.default_headers["Cookie"] = cookie
            profile_id = self._create_profile(
                profiles_instance, "rebinding_block_loopback"
            )
            self._set_rebinding_protection(profiles_instance, profile_id, True)

            resp = await self.dns_lib.send_doh_request(profile_id, LOOPBACK_DOMAIN, A)
            ip = self._answer_ip(resp)
            assert ip == BLOCKED_A, (
                f"Expected {LOOPBACK_DOMAIN} blocked to {BLOCKED_A}, got {ip}"
            )

    @pytest.mark.asyncio
    async def test_disable_restores_resolution(self, create_account_and_login):
        """specRef: R5 / seam — toggling the setting off restores normal resolution
        (relies on PROFILE_SETTINGS_CACHE_TTL=1ms for immediate visibility)."""
        account, cookie = create_account_and_login
        with client.ApiClient(self.api_config) as api_client:
            profiles_instance = api.ProfileApi(api_client)
            profiles_instance.api_client.default_headers["Cookie"] = cookie
            profile_id = self._create_profile(
                profiles_instance, "rebinding_toggle"
            )

            self._set_rebinding_protection(profiles_instance, profile_id, True)
            resp_blocked = await self.dns_lib.send_doh_request(
                profile_id, PRIVATE_V4_DOMAIN, A
            )
            assert self._answer_ip(resp_blocked) == BLOCKED_A, "Expected block when enabled"

            self._set_rebinding_protection(profiles_instance, profile_id, False)
            resp_open = await self.dns_lib.send_doh_request(
                profile_id, PRIVATE_V4_DOMAIN, A
            )
            ip = self._answer_ip(resp_open)
            assert ip == PRIVATE_V4_IP, (
                f"Expected resolution restored to {PRIVATE_V4_IP} after disable, got {ip}"
            )

    @pytest.mark.asyncio
    async def test_custom_allow_overrides_rebinding(self, create_account_and_login):
        """specRef: R12 — a user custom Allow rule (tier 200) overrides the rebinding
        block (tier 150) via Allow-wins cross-phase aggregation."""
        account, cookie = create_account_and_login
        with client.ApiClient(self.api_config) as api_client:
            profiles_instance = api.ProfileApi(api_client)
            profiles_instance.api_client.default_headers["Cookie"] = cookie
            profile_id = self._create_profile(
                profiles_instance, "rebinding_custom_allow"
            )
            self._set_rebinding_protection(profiles_instance, profile_id, True)

            resp_blocked = await self.dns_lib.send_doh_request(
                profile_id, ALLOW_RULE_DOMAIN, A
            )
            assert self._answer_ip(resp_blocked) == BLOCKED_A, (
                "Expected block before the custom allow rule"
            )

            self._create_custom_rule(
                profiles_instance, profile_id, "allow", ALLOW_RULE_DOMAIN
            )
            resp = await self.dns_lib.send_doh_request(profile_id, ALLOW_RULE_DOMAIN, A)
            ip = self._answer_ip(resp)
            assert ip == ALLOW_RULE_IP, (
                f"Custom allow rule should override rebinding block; expected "
                f"{ALLOW_RULE_IP}, got {ip}"
            )

    @pytest.mark.asyncio
    async def test_allow_suffix_bypass(self, create_account_and_login):
        """specRef: R9 — names matching an operator allow-suffix (.local) resolve to a
        private IP even when rebinding protection is enabled."""
        account, cookie = create_account_and_login
        with client.ApiClient(self.api_config) as api_client:
            profiles_instance = api.ProfileApi(api_client)
            profiles_instance.api_client.default_headers["Cookie"] = cookie
            profile_id = self._create_profile(
                profiles_instance, "rebinding_allow_suffix"
            )
            self._set_rebinding_protection(profiles_instance, profile_id, True)

            resp = await self.dns_lib.send_doh_request(profile_id, ALLOW_SUFFIX_DOMAIN, A)
            ip = self._answer_ip(resp)
            assert ip == ALLOW_SUFFIX_IP, (
                f"Allow-suffix domain {ALLOW_SUFFIX_DOMAIN} should resolve to "
                f"{ALLOW_SUFFIX_IP}, got {ip}"
            )

    @pytest.mark.asyncio
    async def test_public_ip_not_blocked(self, create_account_and_login):
        """specRef: R4 — regression: a public IP answer is never blocked, so rebinding
        protection does not break normal resolution."""
        account, cookie = create_account_and_login
        with client.ApiClient(self.api_config) as api_client:
            profiles_instance = api.ProfileApi(api_client)
            profiles_instance.api_client.default_headers["Cookie"] = cookie
            profile_id = self._create_profile(
                profiles_instance, "rebinding_public_ok"
            )
            self._set_rebinding_protection(profiles_instance, profile_id, True)

            resp = await self.dns_lib.send_doh_request(profile_id, PUBLIC_DOMAIN, A)
            ip = self._answer_ip(resp)
            assert ip == PUBLIC_IP, (
                f"Public domain {PUBLIC_DOMAIN} should resolve to {PUBLIC_IP} with "
                f"rebinding enabled, got {ip}"
            )
