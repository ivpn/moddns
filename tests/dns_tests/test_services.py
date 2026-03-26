"""End-to-end tests for services/ASN blocking (IP phase only).

Tests ASN-based service blocking and ASN custom rules evaluated in the
post-resolve (IP) phase.  No domain-phase rules are involved — these are
pure single-phase tests.

Test domains (controlled via testhosts.txt -> sdns hostsfile):
  - svctest-google.com -> 8.8.8.8  (Google AS15169, in services catalog)
  - test.com           -> 104.18.74.230  (Cloudflare AS13335, NOT in catalog)

Requirements:
  - Services catalog mounted at /opt/services/catalog.yml
  - GeoLite2-ASN.mmdb mounted at /opt/geo/GeoLite2-ASN.mmdb
  Tests skip gracefully if the infrastructure is unavailable.
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
    REAL_GOOGLE_DOMAIN,
    REAL_HTTPS_HINTS_DOMAIN,
    TEST_DOMAIN,
)
from dns.rdatatype import A, HTTPS
import dns.rcode

import moddns.api_client as client
import moddns.api as api
import moddns.configuration as api_config


# ===================================================================
# Services blocking (ASN-based, via catalog)
# ===================================================================
class TestServicesBlocking(ProfileHelpers):
    """End-to-end tests for ASN-based services blocking."""

    def setup_class(self):
        self.config = get_settings()
        self.api_config = api_config.Configuration(host=self.config.DNS_API_ADDR)
        self.dns_lib = DNSLib(self.config.DOH_ENDPOINT)

    @pytest.mark.asyncio
    async def test_services_block_by_asn(self, create_account_and_login):
        """Blocking the 'google' service should cause svctest-google.com
        (which resolves to 8.8.8.8, AS15169) to return 0.0.0.0.
        Behaviour table #2."""
        account, cookie = create_account_and_login
        with client.ApiClient(self.api_config) as api_client:
            p = api.ProfileApi(api_client)
            p.api_client.default_headers["Cookie"] = cookie

            if not await services_available(self.dns_lib, p, cookie):
                pytest.skip("Services/ASN blocking not available (GeoIP DB missing?)")

            profile_id = self._create_profile(p, "svc_block")
            self._block_service(p, profile_id, [SVC_GOOGLE_ID])

            resp = await self.dns_lib.send_doh_request(profile_id, SVC_GOOGLE_DOMAIN, A)
            ip_str = extract_ip(resp)
            assert ip_str == "0.0.0.0", (
                f"Services block for {SVC_GOOGLE_ID} did not block "
                f"{SVC_GOOGLE_DOMAIN}; got {ip_str}"
            )

    @pytest.mark.asyncio
    async def test_services_block_does_not_affect_other_asn(
        self, create_account_and_login
    ):
        """Blocking 'google' service must NOT block test.com (Cloudflare AS13335).
        Behaviour table #1 (no rules matched in IP phase)."""
        account, cookie = create_account_and_login
        with client.ApiClient(self.api_config) as api_client:
            p = api.ProfileApi(api_client)
            p.api_client.default_headers["Cookie"] = cookie

            if not await services_available(self.dns_lib, p, cookie):
                pytest.skip("Services/ASN blocking not available")

            profile_id = self._create_profile(p, "svc_other_asn")
            self._block_service(p, profile_id, [SVC_GOOGLE_ID])

            resp = await self.dns_lib.send_doh_request(profile_id, TEST_DOMAIN, A)
            ip_str = extract_ip(resp)
            assert ip_str != "0.0.0.0", (
                f"Blocking {SVC_GOOGLE_ID} should not affect {TEST_DOMAIN} "
                f"(different ASN); got {ip_str}"
            )

    @pytest.mark.asyncio
    async def test_services_unblock_restores_resolution(self, create_account_and_login):
        """After unblocking a service, the domain should resolve normally again."""
        account, cookie = create_account_and_login
        with client.ApiClient(self.api_config) as api_client:
            p = api.ProfileApi(api_client)
            p.api_client.default_headers["Cookie"] = cookie

            if not await services_available(self.dns_lib, p, cookie):
                pytest.skip("Services/ASN blocking not available")

            profile_id = self._create_profile(p, "svc_unblock")
            self._block_service(p, profile_id, [SVC_GOOGLE_ID])

            # Verify blocked first.
            resp = await self.dns_lib.send_doh_request(profile_id, SVC_GOOGLE_DOMAIN, A)
            assert extract_ip(resp) == "0.0.0.0", "Expected blocked before unblock"

            # Unblock.
            self._unblock_service(p, profile_id, [SVC_GOOGLE_ID])

            resp = await self.dns_lib.send_doh_request(profile_id, SVC_GOOGLE_DOMAIN, A)
            ip_str = extract_ip(resp)
            assert ip_str != "0.0.0.0", (
                f"After unblocking {SVC_GOOGLE_ID}, {SVC_GOOGLE_DOMAIN} should "
                f"resolve normally; got {ip_str}"
            )


# ===================================================================
# IP allow overrides services block (intra-IP-phase, T200 > T100)
# ===================================================================
class TestIPAllowOverridesServices(ProfileHelpers):
    """IP custom allow (T200) should override services block (T100)
    within the IP phase."""

    def setup_class(self):
        self.config = get_settings()
        self.api_config = api_config.Configuration(host=self.config.DNS_API_ADDR)
        self.dns_lib = DNSLib(self.config.DOH_ENDPOINT)

    @pytest.mark.asyncio
    async def test_ip_allow_overrides_services_block(self, create_account_and_login):
        """Services block + IP allow for the resolved IP -> Processed.
        IP custom rule (T200) overrides services (T100). Table #6."""
        account, cookie = create_account_and_login
        with client.ApiClient(self.api_config) as api_client:
            p = api.ProfileApi(api_client)
            p.api_client.default_headers["Cookie"] = cookie

            if not await services_available(self.dns_lib, p, cookie):
                pytest.skip("Services/ASN blocking not available")

            profile_id = self._create_profile(p, "ip_allow_svc_6")
            self._block_service(p, profile_id, [SVC_GOOGLE_ID])
            # Allow the specific IP that svctest-google.com resolves to.
            self._create_custom_rule(p, profile_id, "allow", SVC_GOOGLE_IP)

            resp = await self.dns_lib.send_doh_request(profile_id, SVC_GOOGLE_DOMAIN, A)
            ip_str = extract_ip(resp)
            assert ip_str != "0.0.0.0", (
                f"#6: IP allow for {SVC_GOOGLE_IP} should override services "
                f"block; got {ip_str}"
            )


# ===================================================================
# ASN custom rules (IP phase)
# ===================================================================
class TestASNCustomRules(ProfileHelpers):
    """ASN-based custom rules created via the API and evaluated in
    the IP phase (post-resolve)."""

    def setup_class(self):
        self.config = get_settings()
        self.api_config = api_config.Configuration(host=self.config.DNS_API_ADDR)
        self.dns_lib = DNSLib(self.config.DOH_ENDPOINT)

    @pytest.mark.asyncio
    async def test_asn_custom_block(self, create_account_and_login):
        """Block ASN 15169 (Google) -> svctest-google.com should return 0.0.0.0.
        Table #3 variant (IP CR block via ASN syntax)."""
        account, cookie = create_account_and_login
        with client.ApiClient(self.api_config) as api_client:
            p = api.ProfileApi(api_client)
            p.api_client.default_headers["Cookie"] = cookie
            profile_id = self._create_profile(p, "asn_block")

            self._create_custom_rule(p, profile_id, "block", "AS15169")

            resp = await self.dns_lib.send_doh_request(profile_id, SVC_GOOGLE_DOMAIN, A)
            ip_str = extract_ip(resp)
            assert ip_str == "0.0.0.0", (
                f"ASN block for AS15169 did not block {SVC_GOOGLE_DOMAIN}; "
                f"got {ip_str}"
            )

    @pytest.mark.asyncio
    async def test_asn_custom_block_does_not_affect_other_asn(
        self, create_account_and_login
    ):
        """Block ASN 15169 should NOT block test.com (Cloudflare AS13335)."""
        account, cookie = create_account_and_login
        with client.ApiClient(self.api_config) as api_client:
            p = api.ProfileApi(api_client)
            p.api_client.default_headers["Cookie"] = cookie
            profile_id = self._create_profile(p, "asn_block_other")

            self._create_custom_rule(p, profile_id, "block", "AS15169")

            resp = await self.dns_lib.send_doh_request(profile_id, TEST_DOMAIN, A)
            ip_str = extract_ip(resp)
            assert ip_str != "0.0.0.0", (
                f"ASN block for AS15169 should not affect {TEST_DOMAIN} "
                f"(AS13335); got {ip_str}"
            )

    @pytest.mark.asyncio
    async def test_asn_allow_overrides_services_block(self, create_account_and_login):
        """Services block + ASN allow -> Processed.
        ASN custom allow (T200) overrides services block (T100). Table #6 variant."""
        account, cookie = create_account_and_login
        with client.ApiClient(self.api_config) as api_client:
            p = api.ProfileApi(api_client)
            p.api_client.default_headers["Cookie"] = cookie

            if not await services_available(self.dns_lib, p, cookie):
                pytest.skip("Services/ASN blocking not available")

            profile_id = self._create_profile(p, "asn_allow_svc")
            self._block_service(p, profile_id, [SVC_GOOGLE_ID])
            self._create_custom_rule(p, profile_id, "allow", "AS15169")

            resp = await self.dns_lib.send_doh_request(profile_id, SVC_GOOGLE_DOMAIN, A)
            ip_str = extract_ip(resp)
            assert ip_str != "0.0.0.0", (
                f"ASN allow for AS15169 should override services block; "
                f"got {ip_str}"
            )


# ===================================================================
# HTTPS record blocking (real domain)
# ===================================================================
class TestServicesHTTPSBlocking(ProfileHelpers):
    """Verify that HTTPS (type 65) queries for blocked services don't
    leak information that would let browsers bypass A/AAAA blocking.

    Uses google.com (a real domain) because HTTPS records are only
    returned by real authoritative servers.  These tests require the
    recursor to have internet access.
    """

    def setup_class(self):
        self.config = get_settings()
        self.api_config = api_config.Configuration(host=self.config.DNS_API_ADDR)
        self.dns_lib = DNSLib(self.config.DOH_ENDPOINT)

    @pytest.mark.asyncio
    async def test_services_block_https_query_no_ip_hints(
        self, create_account_and_login
    ):
        """When a service is blocked, HTTPS records must not contain
        ipv4hint or ipv6hint parameters that would leak IP addresses
        to browsers. The response is either NODATA (empty answer) when
        hints were present and matched, or contains only hint-free
        HTTPS records (e.g. alpn-only)."""
        account, cookie = create_account_and_login
        with client.ApiClient(self.api_config) as api_client:
            p = api.ProfileApi(api_client)
            p.api_client.default_headers["Cookie"] = cookie

            if not await services_available(self.dns_lib, p, cookie):
                pytest.skip("Services/ASN blocking not available")

            profile_id = self._create_profile(p, "svc_https_hints")
            self._block_service(p, profile_id, [SVC_GOOGLE_ID])

            resp = await self.dns_lib.send_doh_request(
                profile_id, REAL_GOOGLE_DOMAIN, HTTPS
            )

            # HTTPS records without IP hints (e.g. alpn-only) are safe
            # to pass through. Verify none leak ipv4hint/ipv6hint.
            for rrset in resp.answer:
                for rdata in rrset:
                    rdata_text = rdata.to_text()
                    assert "ipv4hint" not in rdata_text, (
                        f"HTTPS record for blocked service leaks ipv4hint: "
                        f"{rdata_text}"
                    )
                    assert "ipv6hint" not in rdata_text, (
                        f"HTTPS record for blocked service leaks ipv6hint: "
                        f"{rdata_text}"
                    )

    @pytest.mark.asyncio
    async def test_services_no_block_real_domain_https_query(
        self, create_account_and_login
    ):
        """When Google service is NOT blocked, HTTPS query should return
        answer records (proves the recursor returns HTTPS records and the
        blocking test above is meaningful)."""
        account, cookie = create_account_and_login
        with client.ApiClient(self.api_config) as api_client:
            p = api.ProfileApi(api_client)
            p.api_client.default_headers["Cookie"] = cookie

            if not await services_available(self.dns_lib, p, cookie):
                pytest.skip("Services/ASN blocking not available")

            profile_id = self._create_profile(p, "svc_real_https_noblock")
            # Do NOT block any service.

            resp = await self.dns_lib.send_doh_request(
                profile_id, REAL_GOOGLE_DOMAIN, HTTPS
            )

            assert resp.answer, (
                f"HTTPS query for {REAL_GOOGLE_DOMAIN} without blocking "
                f"should return HTTPS records; got empty answer. "
                f"Recursor may not have internet access."
            )


# ===================================================================
# HTTPS record IP hints extraction (real domain with ipv4hint/ipv6hint)
# ===================================================================
class TestHTTPSRecordIPHints(ProfileHelpers):
    """Verify that the proxy inspects ipv4hint/ipv6hint inside HTTPS
    records when evaluating IP-phase filters (custom ASN rules).

    Uses cloudflare.com because it serves HTTPS records with real
    ipv4hint and ipv6hint parameters (AS13335).  These tests depend
    on Cloudflare's live authoritative DNS — they are marked
    xfail(strict=False) so a change in upstream record format produces
    a warning instead of a hard CI failure.
    """

    def setup_class(self):
        self.config = get_settings()
        self.api_config = api_config.Configuration(host=self.config.DNS_API_ADDR)
        self.dns_lib = DNSLib(self.config.DOH_ENDPOINT)

    @pytest.mark.asyncio
    @pytest.mark.xfail(
        reason="Depends on cloudflare.com serving HTTPS records with ipv4hint/ipv6hint (external DNS)",
        strict=False,
    )
    async def test_https_hints_precondition(self, create_account_and_login):
        """Precondition: cloudflare.com HTTPS record contains ipv4hint.

        If this fails, Cloudflare changed their HTTPS record format and
        the other tests in this class are not meaningful."""
        account, cookie = create_account_and_login
        with client.ApiClient(self.api_config) as api_client:
            p = api.ProfileApi(api_client)
            p.api_client.default_headers["Cookie"] = cookie

            if not await services_available(self.dns_lib, p, cookie):
                pytest.skip("Services/ASN blocking not available")

            profile_id = self._create_profile(p, "https_hints_pre")

            resp = await self.dns_lib.send_doh_request(
                profile_id, REAL_HTTPS_HINTS_DOMAIN, HTTPS
            )
            assert resp.answer, (
                f"HTTPS query for {REAL_HTTPS_HINTS_DOMAIN} returned empty answer"
            )
            full_answer = " ".join(
                rdata.to_text() for rrset in resp.answer for rdata in rrset
            )
            assert "ipv4hint" in full_answer, (
                f"{REAL_HTTPS_HINTS_DOMAIN} HTTPS record has no ipv4hint; "
                f"got: {full_answer}"
            )

    @pytest.mark.asyncio
    @pytest.mark.xfail(
        reason="Depends on cloudflare.com serving HTTPS records with ipv4hint/ipv6hint (external DNS)",
        strict=False,
    )
    async def test_asn_block_catches_https_ipv4hint(self, create_account_and_login):
        """A custom ASN-block rule for AS13335 (Cloudflare) should block
        an HTTPS query whose ipv4hint IPs belong to that ASN.

        This verifies extractIPsFromSVCB feeds hint IPs into the ASN
        matcher in the IP-phase filter."""
        account, cookie = create_account_and_login
        with client.ApiClient(self.api_config) as api_client:
            p = api.ProfileApi(api_client)
            p.api_client.default_headers["Cookie"] = cookie

            if not await services_available(self.dns_lib, p, cookie):
                pytest.skip("Services/ASN blocking not available")

            profile_id = self._create_profile(p, "https_hints_asn")
            self._create_custom_rule(p, profile_id, "block", "AS13335")

            resp = await self.dns_lib.send_doh_request(
                profile_id, REAL_HTTPS_HINTS_DOMAIN, HTTPS
            )
            # When the proxy extracts ipv4hint IPs from the HTTPS record
            # and matches them against the ASN custom rule, the query
            # should be blocked.  A blocked HTTPS query returns NODATA:
            # RCODE=NOERROR with an empty answer section.
            assert resp.rcode() == dns.rcode.NOERROR, (
                f"HTTPS query for {REAL_HTTPS_HINTS_DOMAIN} with AS13335 "
                f"blocked should return NOERROR (NODATA); "
                f"got rcode {dns.rcode.to_text(resp.rcode())}"
            )
            assert not resp.answer, (
                f"HTTPS query for {REAL_HTTPS_HINTS_DOMAIN} with AS13335 "
                f"blocked should return empty answer (NODATA); "
                f"got: {resp.answer}"
            )

    @pytest.mark.asyncio
    @pytest.mark.xfail(
        reason="Depends on cloudflare.com serving HTTPS records with ipv4hint/ipv6hint (external DNS)",
        strict=False,
    )
    async def test_asn_block_also_blocks_a_record(self, create_account_and_login):
        """Sanity check: the same AS13335 block rule also blocks the A
        query (standard post-resolve IP filtering)."""
        account, cookie = create_account_and_login
        with client.ApiClient(self.api_config) as api_client:
            p = api.ProfileApi(api_client)
            p.api_client.default_headers["Cookie"] = cookie

            if not await services_available(self.dns_lib, p, cookie):
                pytest.skip("Services/ASN blocking not available")

            profile_id = self._create_profile(p, "https_hints_a")
            self._create_custom_rule(p, profile_id, "block", "AS13335")

            resp = await self.dns_lib.send_doh_request(
                profile_id, REAL_HTTPS_HINTS_DOMAIN, A
            )
            ip_str = extract_ip(resp)
            assert ip_str == "0.0.0.0", (
                f"A query for {REAL_HTTPS_HINTS_DOMAIN} with AS13335 blocked "
                f"should return 0.0.0.0; got {ip_str}"
            )
