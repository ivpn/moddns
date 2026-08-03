import pytest
from dns.rdatatype import A

from libs.constants import (
    BLOCKLISTED_DOMAIN,
    BLOCKLISTED_SUBDOMAIN,
    RESOLVABLE_TEST_DOMAIN,
)
from libs.dns_lib import assert_blocked, assert_not_blocked, is_blocked


def _mixed(domain: str) -> str:
    """Alternate the case of a domain, e.g. example.com -> ExAmPlE.cOm."""
    return "".join(c.upper() if i % 2 == 0 else c.lower() for i, c in enumerate(domain))


class TestBlocklistCaseNormalisation:
    """End-to-end tests: blocklist matching is case-insensitive.

    DNS preserves query-name case on the wire (RFC 1035 2.3.3) while name
    comparison is case-insensitive (RFC 4343). Blocklist members are stored
    lowercased by the ingest pipeline
    (``blocklists/internal/extractor/normalize.go``) and the proxy's lookup is a
    byte-exact Redis ``SISMEMBER``, so the queried name must be normalised first.

    These exercise the full stack -- real Redis membership, real proxy filtering --
    which the unit tests (which mock the cache) cannot.

    tableRef: #N1, #N2, #N3, #N4, #N6
    (docs/specs/proxy-filtering-behaviour.md -- Section G: Name Normalisation)
    """

    @pytest.mark.asyncio
    async def test_blocklisted_domain_blocked_lowercase(
        self, user, ensure_test_blocklisted
    ):
        """Control: the domain is blocked in its stored (lowercase) form.

        Establishes filtering is active for this profile, so a mixed-case miss
        below is attributable to case handling alone. tableRef: #N1.
        """
        profile_id = user.new_profile("case-norm")

        resp = await user.wait_for(profile_id, BLOCKLISTED_DOMAIN, A, is_blocked)
        assert_blocked(resp, BLOCKLISTED_DOMAIN)

    @pytest.mark.asyncio
    @pytest.mark.parametrize(
        "transform,label",
        [
            (str.upper, "uppercase"),
            (_mixed, "mixed-case"),
        ],
    )
    async def test_blocklisted_domain_blocked_regardless_of_case(
        self, user, ensure_test_blocklisted, transform, label
    ):
        """The same blocklisted domain is blocked in any case. tableRef: #N2, #N3."""
        profile_id = user.new_profile("case-norm")
        domain = transform(BLOCKLISTED_DOMAIN)

        resp = await user.wait_for(profile_id, domain, A, is_blocked)
        assert_blocked(resp, domain)

    @pytest.mark.asyncio
    @pytest.mark.parametrize(
        "transform,label",
        [
            (str.upper, "uppercase"),
            (_mixed, "mixed-case"),
        ],
    )
    async def test_subdomain_blocked_regardless_of_case(
        self, user, ensure_test_blocklisted, transform, label
    ):
        """Parent-walk matching is case-insensitive too. tableRef: #N4.

        ``sub.example.com`` is not itself listed; it is blocked because
        ``example.com`` is, under the default ``blocklists_subdomains_rule``.
        The walk splits the name, so it must split the normalised form.
        """
        profile_id = user.new_profile("case-norm")
        domain = transform(BLOCKLISTED_SUBDOMAIN)

        resp = await user.wait_for(profile_id, domain, A, is_blocked)
        assert_blocked(resp, domain)

    @pytest.mark.asyncio
    @pytest.mark.parametrize(
        "transform,label",
        [
            (str.upper, "uppercase"),
            (_mixed, "mixed-case"),
        ],
    )
    async def test_non_blocklisted_domain_not_blocked_in_any_case(
        self, user, ensure_test_blocklisted, transform, label
    ):
        """Normalisation must not over-block. tableRef: #N6."""
        profile_id = user.new_profile("case-norm")
        domain = transform(RESOLVABLE_TEST_DOMAIN)

        # NOTE: negative assertion — cannot poll; may read pre-mutation state
        # (see DNSLib.wait_until docstring)
        resp = await user.resolve(profile_id, domain, A)
        assert_not_blocked(resp, domain)
