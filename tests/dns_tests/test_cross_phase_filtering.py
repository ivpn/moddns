"""Cross-phase DNS filtering backend E2E tests.

Tests interactions between domain-phase (pre-resolve) and IP-phase
(post-resolve) filters, covering scenarios from the behaviour table
in docs/specs/proxy-filtering-behaviour.md.

Test domains (controlled via testhosts.txt -> sdns hostsfile):
  - test.com             -> 104.18.74.230  (Cloudflare AS13335, NOT in catalog)
  - svctest-google.com   -> 8.8.8.8       (Google AS15169, in services catalog)
"""

import pytest
from libs.dns_lib import is_blocked
from libs.constants import RESOLVABLE_TEST_DOMAIN, RESOLVABLE_TEST_IP
from libs.profile_helpers import (
    extract_ip,
    services_available,
    SVC_GOOGLE_DOMAIN,
    SVC_GOOGLE_IP,
    SVC_GOOGLE_ID,
)
from dns.rdatatype import A


# ===================================================================
# Unified cross-phase aggregation — domain allow overrides IP blocks
# ===================================================================
class TestCrossPhaseAggregation:
    """Domain-phase custom Allow (T200) overrides IP-phase blocks
    through unified cross-phase aggregation.

    Custom Allow rules are user-authored exceptions and always win,
    following the global aggregation rule: any Allow present wins.
    """

    @pytest.mark.asyncio
    async def test_domain_allow_overrides_services_block(self, user):
        """Domain custom allow + services block -> Processed.
        Domain Allow (T200) overrides services block (T100) through
        unified cross-phase aggregation. tableRef: #8."""
        with user.profiles_api() as p:
            if not await services_available(user.dns, p, user.cookie):
                pytest.skip("Services/ASN blocking not available")

        profile_id = user.new_profile("cross_phase_8")
        user.add_rule(profile_id, "allow", SVC_GOOGLE_DOMAIN)
        user.block_services(profile_id, [SVC_GOOGLE_ID])

        # NOTE: negative assertion — cannot poll; may read pre-mutation state (see DNSLib.wait_until docstring)
        resp = await user.resolve(profile_id, SVC_GOOGLE_DOMAIN, A)
        ip_str = extract_ip(resp)
        assert ip_str != "0.0.0.0", (
            f"#8: Domain allow for {SVC_GOOGLE_DOMAIN} should override "
            f"services block; got {ip_str}"
        )

    @pytest.mark.asyncio
    async def test_domain_allow_overrides_ip_block(self, user):
        """Domain custom allow + IP custom block -> Processed.
        Domain Allow (T200) overrides IP custom block (T200) — Allow
        always wins. tableRef: #9."""
        profile_id = user.new_profile("cross_phase_9")

        user.add_rule(profile_id, "allow", RESOLVABLE_TEST_DOMAIN)
        user.add_rule(profile_id, "block", RESOLVABLE_TEST_IP)

        # NOTE: negative assertion — cannot poll; may read pre-mutation state (see DNSLib.wait_until docstring)
        resp = await user.resolve(profile_id, RESOLVABLE_TEST_DOMAIN, A)
        ip_str = extract_ip(resp)
        assert ip_str != "0.0.0.0", (
            f"#9: Domain allow should override IP block; got {ip_str}"
        )

    @pytest.mark.asyncio
    async def test_domain_allow_overrides_blocklist_and_ip_block(
        self, user, ensure_domain_blocklisted
    ):
        """BL block + domain CR allow + IP CR block -> Processed.
        Domain Allow (T200) overrides both blocklist (T100) and IP
        custom block (T200). tableRef: #15."""
        ensure_domain_blocklisted(RESOLVABLE_TEST_DOMAIN)
        profile_id = user.new_profile("cross_phase_15")
        # Default blocklist (TEST_BLOCKLIST_ID) is already enabled on new profiles.
        user.add_rule(profile_id, "allow", RESOLVABLE_TEST_DOMAIN)
        user.add_rule(profile_id, "block", RESOLVABLE_TEST_IP)

        # NOTE: negative assertion — cannot poll; may read pre-mutation state (see DNSLib.wait_until docstring)
        resp = await user.resolve(profile_id, RESOLVABLE_TEST_DOMAIN, A)
        ip_str = extract_ip(resp)
        assert ip_str != "0.0.0.0", (
            f"#15: Domain allow should override BL block + IP block; "
            f"got {ip_str}"
        )

    @pytest.mark.asyncio
    async def test_domain_allow_overrides_blocklist_and_services_block(
        self, user, ensure_domain_blocklisted
    ):
        """BL block + domain CR allow + services block -> Processed.
        Domain Allow (T200) overrides both blocklist (T100) and services
        block (T100). tableRef: #14."""
        ensure_domain_blocklisted(SVC_GOOGLE_DOMAIN)
        with user.profiles_api() as p:
            if not await services_available(user.dns, p, user.cookie):
                pytest.skip("Services/ASN blocking not available")

        profile_id = user.new_profile("cross_phase_14")
        # Default blocklist (TEST_BLOCKLIST_ID) is already enabled on new profiles.
        user.add_rule(profile_id, "allow", SVC_GOOGLE_DOMAIN)
        user.block_services(profile_id, [SVC_GOOGLE_ID])

        # NOTE: negative assertion — cannot poll; may read pre-mutation state (see DNSLib.wait_until docstring)
        resp = await user.resolve(profile_id, SVC_GOOGLE_DOMAIN, A)
        ip_str = extract_ip(resp)
        assert ip_str != "0.0.0.0", (
            f"#14: Domain allow should override BL block + services block; "
            f"got {ip_str}"
        )

    @pytest.mark.asyncio
    async def test_ip_allow_overrides_services_with_domain_allow(self, user):
        """Domain allow + services block + IP allow -> Processed.
        Both domain and IP allow, services blocked. tableRef: #12."""
        with user.profiles_api() as p:
            if not await services_available(user.dns, p, user.cookie):
                pytest.skip("Services/ASN blocking not available")

        profile_id = user.new_profile("ip_allow_svc_12")
        user.add_rule(profile_id, "allow", SVC_GOOGLE_DOMAIN)
        user.block_services(profile_id, [SVC_GOOGLE_ID])
        user.add_rule(profile_id, "allow", SVC_GOOGLE_IP)

        # NOTE: negative assertion — cannot poll; may read pre-mutation state (see DNSLib.wait_until docstring)
        resp = await user.resolve(profile_id, SVC_GOOGLE_DOMAIN, A)
        ip_str = extract_ip(resp)
        assert ip_str != "0.0.0.0", (
            f"#12: Domain allow + IP allow should override services block; "
            f"got {ip_str}"
        )


# ===================================================================
# Domain block is terminal — IP phase is skipped entirely
# ===================================================================
class TestDomainBlockTerminal:
    """When the domain phase blocks, the IP phase is skipped entirely.
    Configured IP allow rules are inert."""

    @pytest.mark.asyncio
    async def test_domain_block_ignores_ip_allow(self, user):
        """Domain CR block + IP CR allow -> Blocked.
        IP allow can't fire because domain block prevents upstream resolution
        (no response IPs to match). tableRef: #24."""
        profile_id = user.new_profile("terminal_24")

        user.add_rule(profile_id, "block", RESOLVABLE_TEST_DOMAIN)
        user.add_rule(profile_id, "allow", RESOLVABLE_TEST_IP)

        resp = await user.wait_for(profile_id, RESOLVABLE_TEST_DOMAIN, A, is_blocked)
        ip_str = extract_ip(resp)
        assert ip_str == "0.0.0.0", (
            f"#24: Domain block must be terminal -- IP allow should be "
            f"inert; got {ip_str}"
        )

    @pytest.mark.asyncio
    async def test_blocklist_block_ignores_ip_allow(
        self, user, ensure_domain_blocklisted
    ):
        """BL block (no domain CR allow to override) + IP CR allow -> Blocked.
        tableRef: #19 variant with IP allow configured."""
        ensure_domain_blocklisted(RESOLVABLE_TEST_DOMAIN)
        profile_id = user.new_profile("terminal_bl_19")
        # Default blocklist (TEST_BLOCKLIST_ID) is already enabled on new profiles.
        user.add_rule(profile_id, "allow", RESOLVABLE_TEST_IP)

        resp = await user.wait_for(profile_id, RESOLVABLE_TEST_DOMAIN, A, is_blocked)
        ip_str = extract_ip(resp)
        assert ip_str == "0.0.0.0", (
            f"#19 variant: Blocklist block must be terminal -- IP allow "
            f"should be inert; got {ip_str}"
        )

    @pytest.mark.asyncio
    async def test_default_block_ignores_ip_allow(self, user):
        """default_rule=block + IP CR allow -> Blocked.
        Default rule blocks at domain phase, IP allow never evaluated."""
        profile_id = user.new_profile("terminal_default")

        user.patch_setting(profile_id, "/settings/privacy/default_rule", "block")
        user.add_rule(profile_id, "allow", RESOLVABLE_TEST_IP)

        resp = await user.wait_for(profile_id, RESOLVABLE_TEST_DOMAIN, A, is_blocked)
        ip_str = extract_ip(resp)
        assert ip_str == "0.0.0.0", (
            f"Default block must be terminal -- IP allow should be inert; "
            f"got {ip_str}"
        )
