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

import pytest
from dns.rdatatype import A, AAAA

from libs.constants import RESOLVABLE_TEST_DOMAIN, RESOLVABLE_TEST_IP
from libs.dns_lib import assert_blocked, assert_not_blocked, is_blocked

# Known IPv6 target the test domain resolves to via sdns.
TEST_IPV6 = "2001:41d0:701:1100::29c8"
TEST_IPV6_DOMAIN = "ipv6-test.com"
# RFC 5737 TEST-NET address — guaranteed to not appear in any real DNS response.
NONEXISTENT_IPV4 = "192.0.2.1"
# Pinned to 8.8.8.8 in config/testhosts.txt — resolves deterministically and
# shares no IP with RESOLVABLE_TEST_DOMAIN, so "unrelated domain" tests need no
# live DNS.
UNRELATED_PINNED_DOMAIN = "svctest-google.com"


class TestIPCustomRules:
    """Dedicated test suite for IP-based custom rule filtering."""

    # ------------------------------------------------------------------
    # IPv4 block
    # ------------------------------------------------------------------

    @pytest.mark.asyncio
    async def test_block_matching_ipv4(self, user):
        """An IP block rule for an IPv4 that appears in the A response should
        cause the proxy to return 0.0.0.0."""
        profile_id = user.new_profile("ip_block_ipv4")
        user.add_rule(profile_id, "block", RESOLVABLE_TEST_IP)

        resp = await user.wait_for(profile_id, RESOLVABLE_TEST_DOMAIN, A, is_blocked)
        assert_blocked(resp, RESOLVABLE_TEST_DOMAIN)

    # ------------------------------------------------------------------
    # IPv6 block
    # ------------------------------------------------------------------

    @pytest.mark.asyncio
    async def test_block_matching_ipv6(self, user):
        """An IP block rule for an IPv6 that appears in the AAAA response
        should cause the proxy to return ::."""
        profile_id = user.new_profile("ip_block_ipv6")
        user.add_rule(profile_id, "block", TEST_IPV6)

        resp = await user.wait_for(profile_id, TEST_IPV6_DOMAIN, AAAA, is_blocked)
        assert_blocked(resp, TEST_IPV6_DOMAIN)

    # ------------------------------------------------------------------
    # Non-matching IP block (should NOT block)
    # ------------------------------------------------------------------

    @pytest.mark.asyncio
    async def test_block_nonmatching_ip_does_not_block(self, user):
        """An IP block rule for an address that does NOT appear in the DNS
        response must not interfere with normal resolution."""
        profile_id = user.new_profile("ip_block_nonmatch")
        # Block an IP from TEST-NET that no real domain resolves to.
        user.add_rule(profile_id, "block", NONEXISTENT_IPV4)

        # NOTE: negative assertion — cannot poll; may read pre-mutation state (see DNSLib.wait_until docstring)
        resp = await user.resolve(profile_id, RESOLVABLE_TEST_DOMAIN, A)
        assert_not_blocked(resp, RESOLVABLE_TEST_DOMAIN)

    # ------------------------------------------------------------------
    # IP block does not affect unrelated domains
    # ------------------------------------------------------------------

    @pytest.mark.asyncio
    async def test_ip_block_does_not_affect_unrelated_domain(self, user):
        """Blocking an IP that test.com resolves to must not block an
        unrelated pinned domain that resolves to a different IP."""
        profile_id = user.new_profile("ip_block_unrelated")
        user.add_rule(profile_id, "block", RESOLVABLE_TEST_IP)

        # NOTE: negative assertion — cannot poll; may read pre-mutation state (see DNSLib.wait_until docstring)
        resp = await user.resolve(profile_id, UNRELATED_PINNED_DOMAIN, A)
        assert_not_blocked(resp, UNRELATED_PINNED_DOMAIN)

    # ------------------------------------------------------------------
    # IPv4 allow (should not block)
    # ------------------------------------------------------------------

    @pytest.mark.asyncio
    async def test_allow_matching_ipv4(self, user):
        """An IP allow rule for an IPv4 that appears in the A response should
        let the domain resolve normally (not 0.0.0.0)."""
        profile_id = user.new_profile("ip_allow_ipv4")
        user.add_rule(profile_id, "allow", RESOLVABLE_TEST_IP)

        # NOTE: negative assertion — cannot poll; may read pre-mutation state (see DNSLib.wait_until docstring)
        resp = await user.resolve(profile_id, RESOLVABLE_TEST_DOMAIN, A)
        assert_not_blocked(resp, RESOLVABLE_TEST_DOMAIN)

    # ------------------------------------------------------------------
    # Domain allow + IP block — allow wins (unified cross-phase aggregation)
    # ------------------------------------------------------------------

    @pytest.mark.asyncio
    async def test_domain_allow_overrides_ip_block(self, user):
        """When a domain allow rule and an IP block rule both match, the
        domain allow wins through unified cross-phase aggregation.

        Domain Allow (T200) overrides IP custom block (T200) — any Allow
        present wins. tableRef: #9.
        """
        profile_id = user.new_profile("domain_allow_ip_block")
        # Allow the domain explicitly.
        user.add_rule(profile_id, "allow", RESOLVABLE_TEST_DOMAIN)
        # Block the IP it resolves to.
        user.add_rule(profile_id, "block", RESOLVABLE_TEST_IP)

        # NOTE: negative assertion — cannot poll; may read pre-mutation state (see DNSLib.wait_until docstring)
        resp = await user.resolve(profile_id, RESOLVABLE_TEST_DOMAIN, A)
        assert_not_blocked(resp, RESOLVABLE_TEST_DOMAIN)
