import pytest
from dns.rdatatype import A

from libs.constants import BLOCKLISTED_DOMAIN, BLOCKLISTED_SUBDOMAIN
from libs.dns_lib import assert_blocked, assert_not_blocked, is_blocked, is_resolved


class TestSubdomainBlocking:
    """End-to-end tests for subdomain blocking behaviour in the DNS proxy.

    When a parent domain (e.g. example.com) is present in a blocklist the
    proxy should, by default, also block all its subdomains (sub.example.com,
    www.example.com, a.b.example.com, etc.).  A per-profile setting called
    ``blocklists_subdomains_rule`` controls this: ``"block"`` (default) means subdomains
    are blocked; ``"allow"`` means only the exact parent domain is blocked.
    """

    @pytest.mark.asyncio
    async def test_parent_domain_blocked(
        self, user, ensure_test_blocklisted
    ):
        """Verify that a domain explicitly present in the blocklist is blocked.

        This is the baseline: example.com is inserted into the blocklist via
        the ``ensure_test_blocklisted`` fixture and a DNS query for it must
        return 0.0.0.0.
        """
        profile_id = user.new_profile("subdomain")

        resp = await user.wait_for(profile_id, BLOCKLISTED_DOMAIN, A, is_blocked)
        assert_blocked(resp, BLOCKLISTED_DOMAIN)

    @pytest.mark.asyncio
    async def test_subdomain_blocked_by_default(
        self, user, ensure_test_blocklisted
    ):
        """Verify that subdomains are blocked when the parent is in the blocklist.

        This is the core regression test: sub.example.com is NOT inserted into
        the blocklist, yet it must be blocked because example.com is listed and
        the default blocklists_subdomains_rule is "block".
        """
        profile_id = user.new_profile("subdomain")

        resp = await user.wait_for(profile_id, BLOCKLISTED_SUBDOMAIN, A, is_blocked)
        assert_blocked(resp, BLOCKLISTED_SUBDOMAIN)

    @pytest.mark.asyncio
    async def test_www_subdomain_blocked(
        self, user, ensure_test_blocklisted
    ):
        """Verify that www.<parent> is blocked when the parent is in the blocklist.

        Browsers commonly prepend ``www.`` to domains.  The proxy must treat
        www.example.com as a subdomain of the blocklisted example.com.
        """
        profile_id = user.new_profile("subdomain")

        domain = f"www.{BLOCKLISTED_DOMAIN}"
        resp = await user.wait_for(profile_id, domain, A, is_blocked)
        assert_blocked(resp, domain)

    @pytest.mark.asyncio
    async def test_deep_subdomain_blocked(
        self, user, ensure_test_blocklisted
    ):
        """Verify that deeply-nested subdomains are blocked.

        a.b.example.com should still be blocked when example.com is in the
        blocklist and blocklists_subdomains_rule is "block" (default).
        """
        profile_id = user.new_profile("subdomain")

        domain = f"a.b.{BLOCKLISTED_DOMAIN}"
        resp = await user.wait_for(profile_id, domain, A, is_blocked)
        assert_blocked(resp, domain)

    @pytest.mark.asyncio
    async def test_subdomain_allowed_when_rule_disabled(
        self, user, ensure_test_blocklisted
    ):
        """Verify that subdomains pass through when blocklists_subdomains_rule is "allow".

        When the profile setting is changed to "allow", only the exact parent
        domain (example.com) should be blocked.  sub.example.com must not be
        intercepted by the proxy.
        """
        profile_id = user.new_profile("subdomain")

        user.patch_setting(
            profile_id, "/settings/privacy/blocklists_subdomains_rule", "allow"
        )

        resp = await user.wait_for(profile_id, BLOCKLISTED_SUBDOMAIN, A, is_resolved)
        assert_not_blocked(resp, BLOCKLISTED_SUBDOMAIN)

    @pytest.mark.asyncio
    async def test_subdomain_rule_toggle(
        self, user, ensure_test_blocklisted
    ):
        """Verify that toggling blocklists_subdomains_rule takes effect dynamically.

        Steps:
          1. Default (block) -- subdomain query returns 0.0.0.0
          2. Switch to "allow" -- subdomain query is no longer blocked
          3. Switch back to "block" -- subdomain query returns 0.0.0.0 again
        """
        profile_id = user.new_profile("subdomain")

        # Step 1: default setting is "block"
        resp1 = await user.wait_for(profile_id, BLOCKLISTED_SUBDOMAIN, A, is_blocked)
        assert_blocked(resp1, BLOCKLISTED_SUBDOMAIN)

        # Step 2: switch to "allow"
        user.patch_setting(
            profile_id, "/settings/privacy/blocklists_subdomains_rule", "allow"
        )
        resp2 = await user.wait_for(profile_id, BLOCKLISTED_SUBDOMAIN, A, is_resolved)
        assert_not_blocked(resp2, BLOCKLISTED_SUBDOMAIN)

        # Step 3: switch back to "block"
        user.patch_setting(
            profile_id, "/settings/privacy/blocklists_subdomains_rule", "block"
        )
        resp3 = await user.wait_for(profile_id, BLOCKLISTED_SUBDOMAIN, A, is_blocked)
        assert_blocked(resp3, BLOCKLISTED_SUBDOMAIN)

    @pytest.mark.asyncio
    async def test_unrelated_domain_not_blocked(
        self, user, ensure_test_blocklisted
    ):
        """Verify that domains NOT in the blocklist are not affected.

        facebook.com is a well-known domain that is not present in the test
        blocklist.  A DNS query for it must return a valid, non-blocked IP.
        """
        profile_id = user.new_profile("subdomain")

        # NOTE: negative assertion — cannot poll; may read pre-mutation state (see DNSLib.wait_until docstring)
        resp = await user.resolve(profile_id, "facebook.com", A)
        assert_not_blocked(resp, "facebook.com")

    @pytest.mark.asyncio
    @pytest.mark.parametrize(
        "subdomain",
        [
            BLOCKLISTED_SUBDOMAIN,
            f"www.{BLOCKLISTED_DOMAIN}",
            f"deep.sub.{BLOCKLISTED_DOMAIN}",
        ],
        ids=["one-level", "www-prefix", "two-levels"],
    )
    async def test_multiple_subdomain_levels_blocked(
        self, user, ensure_test_blocklisted, subdomain
    ):
        """Parametrized: various subdomain depths are all blocked.

        When example.com is in the blocklist and blocklists_subdomains_rule is "block"
        (default), every subdomain regardless of depth must return 0.0.0.0.
        """
        profile_id = user.new_profile("subdomain")

        resp = await user.wait_for(profile_id, subdomain, A, is_blocked)
        assert_blocked(resp, subdomain)
