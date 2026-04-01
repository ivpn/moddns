"""Cross-phase DNS filtering integration tests.

Tests interactions between domain-phase (pre-resolve) and IP-phase
(post-resolve) filters, covering scenarios from the behaviour table
in docs/proxy-filtering-behaviour.md.

Test domains (controlled via testhosts.txt -> sdns hostsfile):
  - test.com             -> 104.18.74.230  (Cloudflare AS13335, NOT in catalog)
  - svctest-google.com   -> 8.8.8.8       (Google AS15169, in services catalog)
"""

import pytest
from libs.dns_lib import DNSLib
from libs.settings import get_settings
from libs.profile_helpers import (
    ProfileHelpers,
    extract_ip,
    services_available,
    SVC_GOOGLE_DOMAIN,
    SVC_GOOGLE_IP,
    SVC_GOOGLE_ID,
    TEST_DOMAIN,
    TEST_IP,
)
from dns.rdatatype import A

import moddns.api_client as client
import moddns.api as api
import moddns.configuration as api_config


# ===================================================================
# Unified cross-phase aggregation — domain allow overrides IP blocks
# ===================================================================
class TestCrossPhaseAggregation(ProfileHelpers):
    """Domain-phase custom Allow (T200) overrides IP-phase blocks
    through unified cross-phase aggregation.

    Custom Allow rules are user-authored exceptions and always win,
    following the global aggregation rule: any Allow present wins.
    """

    def setup_class(self):
        self.config = get_settings()
        self.api_config = api_config.Configuration(host=self.config.DNS_API_ADDR)
        self.dns_lib = DNSLib(self.config.DOH_ENDPOINT)

    @pytest.mark.asyncio
    async def test_domain_allow_overrides_services_block(
        self, create_account_and_login
    ):
        """Domain custom allow + services block -> Processed.
        Domain Allow (T200) overrides services block (T100) through
        unified cross-phase aggregation. Behaviour table #8."""
        account, cookie = create_account_and_login
        with client.ApiClient(self.api_config) as api_client:
            p = api.ProfileApi(api_client)
            p.api_client.default_headers["Cookie"] = cookie

            if not await services_available(self.dns_lib, p, cookie):
                pytest.skip("Services/ASN blocking not available")

            profile_id = self._create_profile(p, "cross_phase_8")
            self._create_custom_rule(
                p, profile_id, "allow", SVC_GOOGLE_DOMAIN
            )
            self._block_service(p, profile_id, [SVC_GOOGLE_ID])

            resp = await self.dns_lib.send_doh_request(
                profile_id, SVC_GOOGLE_DOMAIN, A
            )
            ip_str = extract_ip(resp)
            assert ip_str != "0.0.0.0", (
                f"#8: Domain allow for {SVC_GOOGLE_DOMAIN} should override "
                f"services block; got {ip_str}"
            )

    @pytest.mark.asyncio
    async def test_domain_allow_overrides_ip_block(
        self, create_account_and_login
    ):
        """Domain custom allow + IP custom block -> Processed.
        Domain Allow (T200) overrides IP custom block (T200) — Allow
        always wins. Behaviour table #9."""
        account, cookie = create_account_and_login
        with client.ApiClient(self.api_config) as api_client:
            p = api.ProfileApi(api_client)
            p.api_client.default_headers["Cookie"] = cookie
            profile_id = self._create_profile(p, "cross_phase_9")

            self._create_custom_rule(p, profile_id, "allow", TEST_DOMAIN)
            self._create_custom_rule(p, profile_id, "block", TEST_IP)

            resp = await self.dns_lib.send_doh_request(profile_id, TEST_DOMAIN, A)
            ip_str = extract_ip(resp)
            assert ip_str != "0.0.0.0", (
                f"#9: Domain allow should override IP block; got {ip_str}"
            )

    @pytest.mark.asyncio
    async def test_domain_allow_overrides_blocklist_and_ip_block(
        self, create_account_and_login, ensure_domain_blocklisted
    ):
        """BL block + domain CR allow + IP CR block -> Processed.
        Domain Allow (T200) overrides both blocklist (T100) and IP
        custom block (T200). Behaviour table #15."""
        account, cookie = create_account_and_login
        ensure_domain_blocklisted(TEST_DOMAIN)
        with client.ApiClient(self.api_config) as api_client:
            p = api.ProfileApi(api_client)
            p.api_client.default_headers["Cookie"] = cookie
            profile_id = self._create_profile(p, "cross_phase_15")
            # Default blocklist (TEST_BLOCKLIST_ID) is already enabled on new profiles.
            self._create_custom_rule(p, profile_id, "allow", TEST_DOMAIN)
            self._create_custom_rule(p, profile_id, "block", TEST_IP)

            resp = await self.dns_lib.send_doh_request(profile_id, TEST_DOMAIN, A)
            ip_str = extract_ip(resp)
            assert ip_str != "0.0.0.0", (
                f"#15: Domain allow should override BL block + IP block; "
                f"got {ip_str}"
            )

    @pytest.mark.asyncio
    async def test_domain_allow_overrides_blocklist_and_services_block(
        self, create_account_and_login, ensure_domain_blocklisted
    ):
        """BL block + domain CR allow + services block -> Processed.
        Domain Allow (T200) overrides both blocklist (T100) and services
        block (T100). Behaviour table #14."""
        account, cookie = create_account_and_login
        ensure_domain_blocklisted(SVC_GOOGLE_DOMAIN)
        with client.ApiClient(self.api_config) as api_client:
            p = api.ProfileApi(api_client)
            p.api_client.default_headers["Cookie"] = cookie

            if not await services_available(self.dns_lib, p, cookie):
                pytest.skip("Services/ASN blocking not available")

            profile_id = self._create_profile(p, "cross_phase_14")
            # Default blocklist (TEST_BLOCKLIST_ID) is already enabled on new profiles.
            self._create_custom_rule(
                p, profile_id, "allow", SVC_GOOGLE_DOMAIN
            )
            self._block_service(p, profile_id, [SVC_GOOGLE_ID])

            resp = await self.dns_lib.send_doh_request(
                profile_id, SVC_GOOGLE_DOMAIN, A
            )
            ip_str = extract_ip(resp)
            assert ip_str != "0.0.0.0", (
                f"#14: Domain allow should override BL block + services block; "
                f"got {ip_str}"
            )

    @pytest.mark.asyncio
    async def test_ip_allow_overrides_services_with_domain_allow(
        self, create_account_and_login
    ):
        """Domain allow + services block + IP allow -> Processed.
        Both domain and IP allow, services blocked. Table #12."""
        account, cookie = create_account_and_login
        with client.ApiClient(self.api_config) as api_client:
            p = api.ProfileApi(api_client)
            p.api_client.default_headers["Cookie"] = cookie

            if not await services_available(self.dns_lib, p, cookie):
                pytest.skip("Services/ASN blocking not available")

            profile_id = self._create_profile(p, "ip_allow_svc_12")
            self._create_custom_rule(
                p, profile_id, "allow", SVC_GOOGLE_DOMAIN
            )
            self._block_service(p, profile_id, [SVC_GOOGLE_ID])
            self._create_custom_rule(p, profile_id, "allow", SVC_GOOGLE_IP)

            resp = await self.dns_lib.send_doh_request(
                profile_id, SVC_GOOGLE_DOMAIN, A
            )
            ip_str = extract_ip(resp)
            assert ip_str != "0.0.0.0", (
                f"#12: Domain allow + IP allow should override services block; "
                f"got {ip_str}"
            )


# ===================================================================
# Domain block is terminal — IP phase is skipped entirely
# ===================================================================
class TestDomainBlockTerminal(ProfileHelpers):
    """When the domain phase blocks, the IP phase is skipped entirely.
    Configured IP allow rules are inert."""

    def setup_class(self):
        self.config = get_settings()
        self.api_config = api_config.Configuration(host=self.config.DNS_API_ADDR)
        self.dns_lib = DNSLib(self.config.DOH_ENDPOINT)

    @pytest.mark.asyncio
    async def test_domain_block_ignores_ip_allow(self, create_account_and_login):
        """Domain CR block + IP CR allow -> Blocked.
        IP allow can't fire because domain block prevents upstream resolution
        (no response IPs to match). Table #24."""
        account, cookie = create_account_and_login
        with client.ApiClient(self.api_config) as api_client:
            p = api.ProfileApi(api_client)
            p.api_client.default_headers["Cookie"] = cookie
            profile_id = self._create_profile(p, "terminal_24")

            self._create_custom_rule(p, profile_id, "block", TEST_DOMAIN)
            self._create_custom_rule(p, profile_id, "allow", TEST_IP)

            resp = await self.dns_lib.send_doh_request(profile_id, TEST_DOMAIN, A)
            ip_str = extract_ip(resp)
            assert ip_str == "0.0.0.0", (
                f"#24: Domain block must be terminal -- IP allow should be "
                f"inert; got {ip_str}"
            )

    @pytest.mark.asyncio
    async def test_blocklist_block_ignores_ip_allow(
        self, create_account_and_login, ensure_domain_blocklisted
    ):
        """BL block (no domain CR allow to override) + IP CR allow -> Blocked.
        Table #19 variant with IP allow configured."""
        account, cookie = create_account_and_login
        ensure_domain_blocklisted(TEST_DOMAIN)
        with client.ApiClient(self.api_config) as api_client:
            p = api.ProfileApi(api_client)
            p.api_client.default_headers["Cookie"] = cookie
            profile_id = self._create_profile(p, "terminal_bl_19")
            # Default blocklist (TEST_BLOCKLIST_ID) is already enabled on new profiles.
            self._create_custom_rule(p, profile_id, "allow", TEST_IP)

            resp = await self.dns_lib.send_doh_request(profile_id, TEST_DOMAIN, A)
            ip_str = extract_ip(resp)
            assert ip_str == "0.0.0.0", (
                f"#19 variant: Blocklist block must be terminal -- IP allow "
                f"should be inert; got {ip_str}"
            )

    @pytest.mark.asyncio
    async def test_default_block_ignores_ip_allow(self, create_account_and_login):
        """default_rule=block + IP CR allow -> Blocked.
        Default rule blocks at domain phase, IP allow never evaluated."""
        account, cookie = create_account_and_login
        with client.ApiClient(self.api_config) as api_client:
            p = api.ProfileApi(api_client)
            p.api_client.default_headers["Cookie"] = cookie
            profile_id = self._create_profile(p, "terminal_default")

            from moddns import RequestsProfileUpdates, ModelProfileUpdate

            p.api_v1_profiles_id_patch_with_http_info(
                id=profile_id,
                body=RequestsProfileUpdates(
                    updates=[
                        ModelProfileUpdate(
                            operation="replace",
                            path="/settings/privacy/default_rule",
                            value={"value": "block"},
                        )
                    ]
                ),
            )
            self._create_custom_rule(p, profile_id, "allow", TEST_IP)

            resp = await self.dns_lib.send_doh_request(profile_id, TEST_DOMAIN, A)
            ip_str = extract_ip(resp)
            assert ip_str == "0.0.0.0", (
                f"Default block must be terminal -- IP allow should be inert; "
                f"got {ip_str}"
            )
