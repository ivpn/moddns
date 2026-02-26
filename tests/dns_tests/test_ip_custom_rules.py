"""End-to-end tests for IP-based custom rules.

IP custom rules are evaluated *after* DNS resolution (post-resolve), unlike
domain rules which are evaluated before.  The proxy inspects the A / AAAA
records in the upstream response and matches them against IP rules.

Test domains and their known IPs (resolved by sdns upstream):
  - test.com       → 104.18.74.230  (A)
  - ipv6-test.com  → 2001:41d0:701:1100::29c8  (AAAA)

These IPs are used in the existing test_custom_rules.py parametrization
and are assumed stable for the CI environment.
"""

from ipaddress import ip_address

import pytest
from libs.dns_lib import DNSLib
from libs.settings import get_settings
from dns.rdatatype import A, AAAA

import moddns.api_client as client
import moddns.api as api
import moddns.configuration as api_config
from moddns import (
    RequestsCreateProfileCustomRuleBody,
    ApiCreateProfileBody,
)

# Known IPs that the test domains resolve to via sdns.
TEST_IPV4 = "104.18.74.230"
TEST_IPV4_DOMAIN = "test.com"
TEST_IPV6 = "2001:41d0:701:1100::29c8"
TEST_IPV6_DOMAIN = "ipv6-test.com"
# RFC 5737 TEST-NET address — guaranteed to not appear in any real DNS response.
NONEXISTENT_IPV4 = "192.0.2.1"


class TestIPCustomRules:
    """Dedicated test suite for IP-based custom rule filtering."""

    def setup_class(self):
        self.config = get_settings()
        self.api_config = api_config.Configuration(host=self.config.DNS_API_ADDR)
        self.dns_lib = DNSLib(self.config.DOH_ENDPOINT)

    def _create_profile(self, profiles_instance, name):
        body = ApiCreateProfileBody(name=name)
        resp = profiles_instance.api_v1_profiles_post_with_http_info(body=body)
        assert resp.status_code == 201, (
            f"Profile creation failed with status code: {resp.status_code}"
        )
        return resp.data.profile_id

    def _create_custom_rule(self, profiles_instance, profile_id, action, value):
        body = RequestsCreateProfileCustomRuleBody(action=action, value=value)
        resp = profiles_instance.api_v1_profiles_id_custom_rules_post_with_http_info(
            id=profile_id, body=body
        )
        assert resp.status_code == 201, (
            f"Custom rule creation failed for {value} with status code: {resp.status_code}"
        )
        return resp

    # ------------------------------------------------------------------
    # IPv4 block
    # ------------------------------------------------------------------

    @pytest.mark.asyncio
    async def test_block_matching_ipv4(self, create_account_and_login):
        """An IP block rule for an IPv4 that appears in the A response should
        cause the proxy to return 0.0.0.0."""
        account, cookie = create_account_and_login
        with client.ApiClient(self.api_config) as api_client:
            p = api.ProfileApi(api_client)
            p.api_client.default_headers["Cookie"] = cookie
            profile_id = self._create_profile(p, "ip_block_ipv4")

            self._create_custom_rule(p, profile_id, "block", TEST_IPV4)

            resp = await self.dns_lib.send_doh_request(
                profile_id, TEST_IPV4_DOMAIN, A
            )
            assert resp.answer, f"Expected a blocked answer for {TEST_IPV4_DOMAIN}"
            ip_addr = resp.answer[0].to_text().split(" ")[-1]
            assert ip_addr == "0.0.0.0", (
                f"IP block rule for {TEST_IPV4} did not block {TEST_IPV4_DOMAIN}; "
                f"got {ip_addr}"
            )

    # ------------------------------------------------------------------
    # IPv6 block
    # ------------------------------------------------------------------

    @pytest.mark.asyncio
    async def test_block_matching_ipv6(self, create_account_and_login):
        """An IP block rule for an IPv6 that appears in the AAAA response
        should cause the proxy to return ::."""
        account, cookie = create_account_and_login
        with client.ApiClient(self.api_config) as api_client:
            p = api.ProfileApi(api_client)
            p.api_client.default_headers["Cookie"] = cookie
            profile_id = self._create_profile(p, "ip_block_ipv6")

            self._create_custom_rule(p, profile_id, "block", TEST_IPV6)

            resp = await self.dns_lib.send_doh_request(
                profile_id, TEST_IPV6_DOMAIN, AAAA
            )
            assert resp.answer, f"Expected a blocked answer for {TEST_IPV6_DOMAIN}"
            ip_addr = resp.answer[0].to_text().split(" ")[-1]
            assert ip_address(ip_addr) == ip_address("::"), (
                f"IP block rule for {TEST_IPV6} did not block {TEST_IPV6_DOMAIN}; "
                f"got {ip_addr}"
            )

    # ------------------------------------------------------------------
    # Non-matching IP block (should NOT block)
    # ------------------------------------------------------------------

    @pytest.mark.asyncio
    async def test_block_nonmatching_ip_does_not_block(
        self, create_account_and_login
    ):
        """An IP block rule for an address that does NOT appear in the DNS
        response must not interfere with normal resolution."""
        account, cookie = create_account_and_login
        with client.ApiClient(self.api_config) as api_client:
            p = api.ProfileApi(api_client)
            p.api_client.default_headers["Cookie"] = cookie
            profile_id = self._create_profile(p, "ip_block_nonmatch")

            # Block an IP from TEST-NET that no real domain resolves to.
            self._create_custom_rule(p, profile_id, "block", NONEXISTENT_IPV4)

            resp = await self.dns_lib.send_doh_request(
                profile_id, "google.com", A
            )
            assert resp.answer, "Expected an answer for google.com"
            ip_addr = resp.answer[0].to_text().split(" ")[-1]
            assert ip_addr != "0.0.0.0", (
                f"Non-matching IP block rule for {NONEXISTENT_IPV4} should not "
                f"block google.com; got {ip_addr}"
            )
            assert ip_address(ip_addr), f"Expected a valid IP, got {ip_addr}"

    # ------------------------------------------------------------------
    # IP block does not affect unrelated domains
    # ------------------------------------------------------------------

    @pytest.mark.asyncio
    async def test_ip_block_does_not_affect_unrelated_domain(
        self, create_account_and_login
    ):
        """Blocking an IP that test.com resolves to must not block google.com
        (which resolves to a different IP)."""
        account, cookie = create_account_and_login
        with client.ApiClient(self.api_config) as api_client:
            p = api.ProfileApi(api_client)
            p.api_client.default_headers["Cookie"] = cookie
            profile_id = self._create_profile(p, "ip_block_unrelated")

            self._create_custom_rule(p, profile_id, "block", TEST_IPV4)

            resp = await self.dns_lib.send_doh_request(
                profile_id, "google.com", A
            )
            assert resp.answer, "Expected an answer for google.com"
            ip_addr = resp.answer[0].to_text().split(" ")[-1]
            assert ip_addr != "0.0.0.0", (
                f"IP block rule for {TEST_IPV4} should not block google.com; "
                f"got {ip_addr}"
            )

    # ------------------------------------------------------------------
    # IPv4 allow (should not block)
    # ------------------------------------------------------------------

    @pytest.mark.asyncio
    async def test_allow_matching_ipv4(self, create_account_and_login):
        """An IP allow rule for an IPv4 that appears in the A response should
        let the domain resolve normally (not 0.0.0.0)."""
        account, cookie = create_account_and_login
        with client.ApiClient(self.api_config) as api_client:
            p = api.ProfileApi(api_client)
            p.api_client.default_headers["Cookie"] = cookie
            profile_id = self._create_profile(p, "ip_allow_ipv4")

            self._create_custom_rule(p, profile_id, "allow", TEST_IPV4)

            resp = await self.dns_lib.send_doh_request(
                profile_id, TEST_IPV4_DOMAIN, A
            )
            assert resp.answer, f"Expected an answer for {TEST_IPV4_DOMAIN}"
            ip_addr = resp.answer[0].to_text().split(" ")[-1]
            assert ip_addr != "0.0.0.0", (
                f"IP allow rule for {TEST_IPV4} should not block {TEST_IPV4_DOMAIN}; "
                f"got {ip_addr}"
            )

    # ------------------------------------------------------------------
    # IP block + domain allow coexistence
    # ------------------------------------------------------------------

    @pytest.mark.asyncio
    async def test_ip_block_overrides_domain_allow(
        self, create_account_and_login
    ):
        """When a domain allow rule and an IP block rule both match, the IP
        block should still block the response.

        Domain filtering (pre-resolve) runs first and allows the query through.
        IP filtering (post-resolve) then sees the response IPs and blocks.
        """
        account, cookie = create_account_and_login
        with client.ApiClient(self.api_config) as api_client:
            p = api.ProfileApi(api_client)
            p.api_client.default_headers["Cookie"] = cookie
            profile_id = self._create_profile(p, "ip_block_domain_allow")

            # Allow the domain explicitly.
            self._create_custom_rule(p, profile_id, "allow", TEST_IPV4_DOMAIN)
            # Block the IP it resolves to.
            self._create_custom_rule(p, profile_id, "block", TEST_IPV4)

            resp = await self.dns_lib.send_doh_request(
                profile_id, TEST_IPV4_DOMAIN, A
            )
            assert resp.answer, f"Expected a blocked answer for {TEST_IPV4_DOMAIN}"
            ip_addr = resp.answer[0].to_text().split(" ")[-1]
            assert ip_addr == "0.0.0.0", (
                f"IP block rule should override domain allow for {TEST_IPV4_DOMAIN}; "
                f"got {ip_addr}"
            )
