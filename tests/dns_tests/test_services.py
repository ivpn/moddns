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
from libs.dns_lib import is_blocked, is_resolved, assert_blocked, assert_not_blocked
from libs.constants import RESOLVABLE_TEST_DOMAIN
from libs.profile_helpers import (
    services_available,
    SVC_GOOGLE_DOMAIN,
    SVC_GOOGLE_IP,
    SVC_GOOGLE_ID,
    SVC_GOOGLE_ALIAS_ID,
    SVC_APPLE_DOMAIN,
    SVC_APPLE_ID,
    SVC_MICROSOFT_DOMAIN,
    SVC_MICROSOFT_ID,
    REAL_GOOGLE_DOMAIN,
    REAL_HTTPS_HINTS_DOMAIN,
)
from dns.rdatatype import A, HTTPS
import dns.rcode


# ===================================================================
# Services blocking (ASN-based, via catalog)
#
# Covers both the canonical service ID and its catalog *alias*: the
# alias (``google-legacy``) exercises the zero-downtime service-ID
# rename mechanism — the proxy's FindByID resolves an alias to the
# underlying service, so blocking the alias must yield exactly the same
# ASN blocking as the canonical ID. This is what keeps blocking from
# failing open while profiles are migrated off an old ID.
# ===================================================================
class TestServicesBlocking:
    """End-to-end tests for ASN-based services blocking."""

    @pytest.mark.asyncio
    @pytest.mark.parametrize(
        "service_id, domain",
        [
            pytest.param(SVC_GOOGLE_ID, SVC_GOOGLE_DOMAIN, id="google"),
            pytest.param(SVC_GOOGLE_ALIAS_ID, SVC_GOOGLE_DOMAIN, id="alias"),
            pytest.param(
                SVC_APPLE_ID,
                SVC_APPLE_DOMAIN,
                marks=pytest.mark.xfail(
                    strict=False,
                    reason="Depends on apple.com resolving to Apple ASN (external DNS)",
                ),
                id="apple",
            ),
            pytest.param(
                SVC_MICROSOFT_ID,
                SVC_MICROSOFT_DOMAIN,
                marks=pytest.mark.xfail(
                    strict=False,
                    reason="Depends on microsoft.com resolving to Microsoft ASN (external DNS)",
                ),
                id="microsoft",
            ),
        ],
    )
    async def test_services_block_by_asn(self, user, service_id, domain):
        """Blocking a service blocks every domain resolving into its ASN set.
        tableRef: #2. Each parametrized service resolves to an IP in the
        service's ASN and must come back as the block sentinel (0.0.0.0):

        - google: svctest-google.com -> 8.8.8.8 (AS15169), pinned/deterministic.
        - alias (google-legacy): a catalog *alias* of 'google'. The proxy's
          FindByID resolves the alias to the underlying 'google' service, so
          blocking the alias yields identical ASN blocking to the canonical ID.
        - apple: apple.com -> AS714/AS6185 (live external DNS, xfail).
        - microsoft: microsoft.com -> AS8068-AS8075 (live external DNS, xfail).
        """
        with user.profiles_api() as p:
            if not await services_available(user.dns, p, user.cookie):
                pytest.skip("Services/ASN blocking not available (GeoIP DB missing?)")

        profile_id = user.new_profile("svc_block")
        user.block_services(profile_id, [service_id])

        resp = await user.wait_for(profile_id, domain, A, is_blocked)
        assert_blocked(resp, domain)

    @pytest.mark.asyncio
    @pytest.mark.parametrize(
        "service_id",
        [
            pytest.param(SVC_GOOGLE_ID, id="google"),
            pytest.param(SVC_GOOGLE_ALIAS_ID, id="alias"),
        ],
    )
    async def test_services_block_does_not_affect_other_asn(self, user, service_id):
        """Blocking the google service (by canonical ID or alias) must NOT
        over-block: test.com (Cloudflare AS13335) stays resolvable — a
        different ASN is unaffected.
        tableRef: #1 (no rules matched in IP phase)."""
        with user.profiles_api() as p:
            if not await services_available(user.dns, p, user.cookie):
                pytest.skip("Services/ASN blocking not available")

        profile_id = user.new_profile("svc_other_asn")
        user.block_services(profile_id, [service_id])

        # NOTE: negative assertion — cannot poll; may read pre-mutation state (see DNSLib.wait_until docstring)
        resp = await user.resolve(profile_id, RESOLVABLE_TEST_DOMAIN, A)
        assert_not_blocked(resp, RESOLVABLE_TEST_DOMAIN)

    @pytest.mark.asyncio
    @pytest.mark.parametrize(
        "service_id",
        [
            pytest.param(SVC_GOOGLE_ID, id="google"),
            pytest.param(SVC_GOOGLE_ALIAS_ID, id="alias"),
        ],
    )
    async def test_services_unblock_restores_resolution(self, user, service_id):
        """After unblocking the service (by canonical ID or alias), the domain
        resolves normally again — the alias round-trips through enable/disable
        exactly like a canonical service ID."""
        with user.profiles_api() as p:
            if not await services_available(user.dns, p, user.cookie):
                pytest.skip("Services/ASN blocking not available")

        profile_id = user.new_profile("svc_unblock")
        user.block_services(profile_id, [service_id])

        # Verify blocked first.
        resp = await user.wait_for(profile_id, SVC_GOOGLE_DOMAIN, A, is_blocked)
        assert_blocked(resp, SVC_GOOGLE_DOMAIN)

        # Unblock.
        user.unblock_services(profile_id, [service_id])

        resp = await user.wait_for(profile_id, SVC_GOOGLE_DOMAIN, A, is_resolved)
        assert_not_blocked(resp, SVC_GOOGLE_DOMAIN)


# ===================================================================
# IP allow overrides services block (intra-IP-phase, T200 > T100)
# ===================================================================
class TestIPAllowOverridesServices:
    """IP custom allow (T200) should override services block (T100)
    within the IP phase."""

    @pytest.mark.asyncio
    async def test_ip_allow_overrides_services_block(self, user):
        """Services block + IP allow for the resolved IP -> Processed.
        IP custom rule (T200) overrides services (T100). tableRef: #6."""
        with user.profiles_api() as p:
            if not await services_available(user.dns, p, user.cookie):
                pytest.skip("Services/ASN blocking not available")

        profile_id = user.new_profile("ip_allow_svc_6")
        user.block_services(profile_id, [SVC_GOOGLE_ID])
        # Allow the specific IP that svctest-google.com resolves to.
        user.add_rule(profile_id, "allow", SVC_GOOGLE_IP)

        # NOTE: negative assertion — cannot poll; may read pre-mutation state (see DNSLib.wait_until docstring)
        resp = await user.resolve(profile_id, SVC_GOOGLE_DOMAIN, A)
        assert_not_blocked(resp, SVC_GOOGLE_DOMAIN)


# ===================================================================
# ASN custom rules (IP phase)
# ===================================================================
class TestASNCustomRules:
    """ASN-based custom rules created via the API and evaluated in
    the IP phase (post-resolve)."""

    @pytest.mark.asyncio
    async def test_asn_custom_block(self, user):
        """Block ASN 15169 (Google) -> svctest-google.com should return 0.0.0.0.
        tableRef: #3 variant (IP CR block via ASN syntax)."""
        profile_id = user.new_profile("asn_block")
        user.add_rule(profile_id, "block", "AS15169")

        resp = await user.wait_for(profile_id, SVC_GOOGLE_DOMAIN, A, is_blocked)
        assert_blocked(resp, SVC_GOOGLE_DOMAIN)

    @pytest.mark.asyncio
    async def test_asn_custom_block_does_not_affect_other_asn(self, user):
        """Block ASN 15169 should NOT block test.com (Cloudflare AS13335)."""
        profile_id = user.new_profile("asn_block_other")
        user.add_rule(profile_id, "block", "AS15169")

        # NOTE: negative assertion — cannot poll; may read pre-mutation state (see DNSLib.wait_until docstring)
        resp = await user.resolve(profile_id, RESOLVABLE_TEST_DOMAIN, A)
        assert_not_blocked(resp, RESOLVABLE_TEST_DOMAIN)

    @pytest.mark.asyncio
    async def test_asn_allow_overrides_services_block(self, user):
        """Services block + ASN allow -> Processed.
        ASN custom allow (T200) overrides services block (T100). tableRef: #6 variant."""
        with user.profiles_api() as p:
            if not await services_available(user.dns, p, user.cookie):
                pytest.skip("Services/ASN blocking not available")

        profile_id = user.new_profile("asn_allow_svc")
        user.block_services(profile_id, [SVC_GOOGLE_ID])
        user.add_rule(profile_id, "allow", "AS15169")

        # NOTE: negative assertion — cannot poll; may read pre-mutation state (see DNSLib.wait_until docstring)
        resp = await user.resolve(profile_id, SVC_GOOGLE_DOMAIN, A)
        assert_not_blocked(resp, SVC_GOOGLE_DOMAIN)


# ===================================================================
# HTTPS record blocking (real domain)
# ===================================================================
class TestServicesHTTPSBlocking:
    """Verify that HTTPS (type 65) queries for blocked services don't
    leak information that would let browsers bypass A/AAAA blocking.

    Uses google.com (a real domain) because HTTPS records are only
    returned by real authoritative servers.  These tests require the
    recursor to have internet access.
    """

    @pytest.mark.asyncio
    @pytest.mark.xfail(
        strict=False,
        reason="depends on live external DNS (google.com HTTPS records)",
    )
    async def test_services_block_https_query_no_ip_hints(self, user):
        """When a service is blocked, HTTPS records must not contain
        ipv4hint or ipv6hint parameters that would leak IP addresses
        to browsers. The response is either NODATA (empty answer) when
        hints were present and matched, or contains only hint-free
        HTTPS records (e.g. alpn-only)."""
        with user.profiles_api() as p:
            if not await services_available(user.dns, p, user.cookie):
                pytest.skip("Services/ASN blocking not available")

        profile_id = user.new_profile("svc_https_hints")
        user.block_services(profile_id, [SVC_GOOGLE_ID])

        resp = await user.wait_for(
            profile_id, REAL_GOOGLE_DOMAIN, HTTPS, lambda r: bool(r.answer)
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
    @pytest.mark.xfail(
        strict=False,
        reason="depends on live external DNS (google.com HTTPS records)",
    )
    async def test_services_no_block_real_domain_https_query(self, user):
        """When Google service is NOT blocked, HTTPS query should return
        answer records (proves the recursor returns HTTPS records and the
        blocking test above is meaningful)."""
        with user.profiles_api() as p:
            if not await services_available(user.dns, p, user.cookie):
                pytest.skip("Services/ASN blocking not available")

        profile_id = user.new_profile("svc_real_https_noblock")
        # Do NOT block any service.

        resp = await user.wait_for(
            profile_id, REAL_GOOGLE_DOMAIN, HTTPS, lambda r: bool(r.answer)
        )

        assert resp.answer, (
            f"HTTPS query for {REAL_GOOGLE_DOMAIN} without blocking "
            f"should return HTTPS records; got empty answer. "
            f"Recursor may not have internet access."
        )


# ===================================================================
# HTTPS record IP hints extraction (real domain with ipv4hint/ipv6hint)
# ===================================================================
class TestHTTPSRecordIPHints:
    """Verify that the proxy inspects ipv4hint/ipv6hint inside HTTPS
    records when evaluating IP-phase filters (custom ASN rules).

    Uses cloudflare.com because it serves HTTPS records with real
    ipv4hint and ipv6hint parameters (AS13335).  These tests depend
    on Cloudflare's live authoritative DNS — they are marked
    xfail(strict=False) so a change in upstream record format produces
    a warning instead of a hard CI failure.
    """

    @pytest.mark.asyncio
    @pytest.mark.xfail(
        reason="Depends on cloudflare.com serving HTTPS records with ipv4hint/ipv6hint (external DNS)",
        strict=False,
    )
    async def test_https_hints_precondition(self, user):
        """Precondition: cloudflare.com HTTPS record contains ipv4hint.

        If this fails, Cloudflare changed their HTTPS record format and
        the other tests in this class are not meaningful."""
        with user.profiles_api() as p:
            if not await services_available(user.dns, p, user.cookie):
                pytest.skip("Services/ASN blocking not available")

        profile_id = user.new_profile("https_hints_pre")

        resp = await user.wait_for(
            profile_id, REAL_HTTPS_HINTS_DOMAIN, HTTPS, lambda r: bool(r.answer)
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
    async def test_asn_block_catches_https_ipv4hint(self, user):
        """A custom ASN-block rule for AS13335 (Cloudflare) should block
        an HTTPS query whose ipv4hint IPs belong to that ASN.

        This verifies extractIPsFromSVCB feeds hint IPs into the ASN
        matcher in the IP-phase filter."""
        with user.profiles_api() as p:
            if not await services_available(user.dns, p, user.cookie):
                pytest.skip("Services/ASN blocking not available")

        profile_id = user.new_profile("https_hints_asn")
        user.add_rule(profile_id, "block", "AS13335")

        resp = await user.wait_for(
            profile_id, REAL_HTTPS_HINTS_DOMAIN, HTTPS,
            lambda r: r.rcode() == dns.rcode.NOERROR and not r.answer,
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
    async def test_asn_block_also_blocks_a_record(self, user):
        """Sanity check: the same AS13335 block rule also blocks the A
        query (standard post-resolve IP filtering)."""
        with user.profiles_api() as p:
            if not await services_available(user.dns, p, user.cookie):
                pytest.skip("Services/ASN blocking not available")

        profile_id = user.new_profile("https_hints_a")
        user.add_rule(profile_id, "block", "AS13335")

        resp = await user.wait_for(
            profile_id, REAL_HTTPS_HINTS_DOMAIN, A, is_blocked
        )
        assert_blocked(resp, REAL_HTTPS_HINTS_DOMAIN)
