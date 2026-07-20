import pytest
from dns.rdatatype import A

from libs.constants import (
    BLOCKLISTED_DOMAIN,
    BLOCKLISTED_SUBDOMAIN,
)
from libs.dns_lib import (
    assert_blocked,
    assert_not_blocked,
    is_blocked,
    is_resolved,
)


class TestCustomRulesPrecedence:
    """
    Backend E2E tests verifying that custom rules take precedence
    over blocklist blocking and default_rule settings.

    The DNS proxy evaluates filtering tiers in priority order:
        CustomRules (tier 200) > Blocklists (tier 100) > DefaultRule (tier 0)

    Each test creates an isolated profile to avoid cross-test interference.
    """

    @pytest.mark.asyncio
    async def test_custom_allow_overrides_blocklist_block(
        self, user, ensure_test_blocklisted
    ):
        """Verify that a custom 'allow' rule overrides a blocklist 'block' for the same domain.

        Setup:
            - example.com is present in the blocklist (via ensure_test_blocklisted fixture).
            - A custom allow rule is created for example.com on a fresh profile.

        Expected:
            - The DNS query for example.com returns a valid IP (not 0.0.0.0)
              because CustomRules tier (200) takes precedence over Blocklists tier (100).
        """
        profile_id = user.new_profile("test_allow_overrides_blocklist")

        # Confirm the domain is blocked by the blocklist before adding the custom rule
        resp_blocked = await user.wait_for(
            profile_id, BLOCKLISTED_DOMAIN, A, is_blocked
        )
        assert_blocked(resp_blocked, BLOCKLISTED_DOMAIN)

        # Create custom allow rule for the blocklisted domain
        user.add_rule(profile_id, "allow", BLOCKLISTED_DOMAIN)

        # Query again -- custom allow should override blocklist block
        resp = await user.wait_for(profile_id, BLOCKLISTED_DOMAIN, A, is_resolved)
        assert_not_blocked(resp, BLOCKLISTED_DOMAIN)

    @pytest.mark.asyncio
    async def test_custom_allow_overrides_subdomain_blocklist_block(
        self, user, ensure_test_blocklisted
    ):
        """Verify that a custom 'allow' rule for a subdomain overrides inherited blocklist blocking.

        Setup:
            - example.com is in the blocklist; subdomain matching means sub.example.com
              is also blocked.
            - A custom allow rule is created for the exact subdomain sub.example.com.

        Expected:
            - The DNS query for sub.example.com returns a valid IP (not 0.0.0.0)
              because the exact custom allow rule overrides the inherited blocklist match.
        """
        profile_id = user.new_profile("test_allow_overrides_subdomain_blocklist")

        # Confirm subdomain is blocked by inherited blocklist rule
        resp_blocked = await user.wait_for(
            profile_id, BLOCKLISTED_SUBDOMAIN, A, is_blocked
        )
        assert_blocked(resp_blocked, BLOCKLISTED_SUBDOMAIN)

        # Create custom allow rule for the exact subdomain
        user.add_rule(profile_id, "allow", BLOCKLISTED_SUBDOMAIN)

        # Query again -- custom allow should override subdomain blocklist match.
        # Note: sub.example.com may not exist in DNS (NXDOMAIN / empty answer),
        # which is fine -- we only verify it's not actively blocked (0.0.0.0).
        # NOTE: negative assertion — cannot poll; may read pre-mutation state (see DNSLib.wait_until docstring)
        resp = await user.resolve(profile_id, BLOCKLISTED_SUBDOMAIN, A)
        if resp.answer:
            ip_addr = resp.answer[0].to_text().split(" ")[-1]
            assert ip_addr != "0.0.0.0", (
                f"Custom allow rule did not override subdomain blocklist block for "
                f"{BLOCKLISTED_SUBDOMAIN}; got {ip_addr}"
            )

    @pytest.mark.asyncio
    async def test_custom_wildcard_allow_overrides_blocklist(
        self, user, ensure_test_blocklisted
    ):
        """Verify that a wildcard custom 'allow' rule overrides blocklist blocking for subdomains.

        Setup:
            - example.com is in the blocklist (sub.example.com is blocked by inheritance).
            - A custom allow rule is created for *.example.com (wildcard).

        Expected:
            - The DNS query for sub.example.com returns a valid IP (not 0.0.0.0)
              because the wildcard custom allow rule matches and overrides the blocklist.
        """
        profile_id = user.new_profile("test_wildcard_allow_overrides_blocklist")

        # Confirm subdomain is blocked before adding wildcard allow
        resp_blocked = await user.wait_for(
            profile_id, BLOCKLISTED_SUBDOMAIN, A, is_blocked
        )
        assert_blocked(resp_blocked, BLOCKLISTED_SUBDOMAIN)

        # Create wildcard custom allow rule
        user.add_rule(profile_id, "allow", f"*.{BLOCKLISTED_DOMAIN}")

        # Query subdomain -- wildcard allow should override blocklist.
        # Note: sub.example.com may not exist in DNS (NXDOMAIN / empty answer),
        # which is fine -- we only verify it's not actively blocked (0.0.0.0).
        # NOTE: negative assertion — cannot poll; may read pre-mutation state (see DNSLib.wait_until docstring)
        resp = await user.resolve(profile_id, BLOCKLISTED_SUBDOMAIN, A)
        if resp.answer:
            ip_addr = resp.answer[0].to_text().split(" ")[-1]
            assert ip_addr != "0.0.0.0", (
                f"Wildcard custom allow rule did not override blocklist block for "
                f"{BLOCKLISTED_SUBDOMAIN}; got {ip_addr}"
            )

    @pytest.mark.asyncio
    async def test_custom_block_on_non_blocklisted_domain(self, user):
        """Verify that a custom 'block' rule blocks a domain that is not in any blocklist.

        Setup:
            - A new profile with default_rule = 'allow' (default behavior).
            - A custom block rule is created for facebook.com.

        Expected:
            - The DNS query for facebook.com returns 0.0.0.0 (blocked by custom rule),
              independent of any blocklist configuration.
        """
        profile_id = user.new_profile("test_custom_block_non_blocklisted")

        # Create custom block rule for a domain not in any blocklist
        user.add_rule(profile_id, "block", "facebook.com")

        resp = await user.wait_for(profile_id, "facebook.com", A, is_blocked)
        assert_blocked(resp, "facebook.com")

    @pytest.mark.asyncio
    async def test_default_block_rule_blocks_all(self, user):
        """Verify that setting default_rule to 'block' blocks all domains.

        Setup:
            - A new profile with default_rule set to 'block' via PATCH API.

        Expected:
            - Any DNS query (e.g., google.com) returns 0.0.0.0
              because the default rule blocks everything.
        """
        profile_id = user.new_profile("test_default_block_all")

        # Set default_rule to block
        user.patch_setting(profile_id, "/settings/privacy/default_rule", "block")

        resp = await user.wait_for(profile_id, "google.com", A, is_blocked)
        assert_blocked(resp, "google.com")

    @pytest.mark.asyncio
    async def test_custom_allow_overrides_default_block(self, user):
        """Verify that a custom 'allow' rule overrides a default_rule of 'block'.

        Setup:
            - A new profile with default_rule set to 'block'.
            - A custom allow rule is created for facebook.com.

        Expected:
            - The DNS query for facebook.com returns a valid IP (not 0.0.0.0)
              because the custom allow rule (tier 200) overrides the default block rule (tier 0).
        """
        profile_id = user.new_profile("test_allow_overrides_default_block")

        # Set default_rule to block
        user.patch_setting(profile_id, "/settings/privacy/default_rule", "block")

        # Confirm facebook.com is blocked by default rule
        resp_blocked = await user.wait_for(profile_id, "facebook.com", A, is_blocked)
        assert_blocked(resp_blocked, "facebook.com")

        # Create custom allow rule for facebook.com
        user.add_rule(profile_id, "allow", "facebook.com")

        # Query again -- custom allow should override default block
        resp = await user.wait_for(profile_id, "facebook.com", A, is_resolved)
        assert_not_blocked(resp, "facebook.com")

    @pytest.mark.asyncio
    async def test_blocklist_block_with_default_block(
        self, user, ensure_test_blocklisted
    ):
        """Verify blocking when both blocklist and default_rule agree on blocking.

        Setup:
            - example.com is in the blocklist (via ensure_test_blocklisted fixture).
            - A new profile with default_rule set to 'block'.

        Expected:
            - The DNS query for example.com returns 0.0.0.0 (both blocklist and default agree).
            - The DNS query for a non-blocklisted domain (e.g., google.com) also returns
              0.0.0.0 (blocked by default rule even though not in any blocklist).
        """
        profile_id = user.new_profile("test_blocklist_and_default_block")

        # Set default_rule to block
        user.patch_setting(profile_id, "/settings/privacy/default_rule", "block")

        # Blocklisted domain should be blocked (both blocklist and default rule)
        resp_blocklisted = await user.wait_for(
            profile_id, BLOCKLISTED_DOMAIN, A, is_blocked
        )
        assert_blocked(resp_blocklisted, BLOCKLISTED_DOMAIN)

        # Non-blocklisted domain should also be blocked (by default rule)
        resp_non_blocklisted = await user.wait_for(
            profile_id, "google.com", A, is_blocked
        )
        assert_blocked(resp_non_blocklisted, "google.com")

    # ------------------------------------------------------------------
    # Custom rule subdomain matching tests
    # ------------------------------------------------------------------

    @pytest.mark.asyncio
    async def test_exact_custom_block_does_not_block_www_subdomain(self, user):
        """Verify that an exact custom block rule does NOT block www.<domain>.

        When custom_rules_subdomains_rule is set to "exact", a rule for
        "facebook.com" (no wildcard) must only block the exact domain,
        not www.facebook.com.

        Wildcards (*.facebook.com or .facebook.com) are required to also
        cover subdomains when using exact mode.
        """
        profile_id = user.new_profile("test_exact_block_no_www")

        # Set custom_rules_subdomains_rule to "exact" so plain domains are not auto-expanded
        user.patch_setting(
            profile_id, "/settings/privacy/custom_rules_subdomains_rule", "exact"
        )

        # Create exact block rule for facebook.com
        user.add_rule(profile_id, "block", "facebook.com")

        # facebook.com itself should be blocked
        resp_exact = await user.wait_for(profile_id, "facebook.com", A, is_blocked)
        assert_blocked(resp_exact, "facebook.com")

        # www.facebook.com should NOT be blocked (exact match only)
        resp_www = await user.resolve(profile_id, "www.facebook.com", A)
        assert_not_blocked(resp_www, "www.facebook.com")

    @pytest.mark.asyncio
    async def test_wildcard_custom_block_blocks_www_subdomain(self, user):
        """Verify that a wildcard custom block rule *.facebook.com blocks www.facebook.com.

        Unlike exact rules, the "*.facebook.com" pattern matches the root domain
        AND all subdomains (www.facebook.com, ads.facebook.com, etc.).
        """
        profile_id = user.new_profile("test_wildcard_block_www")

        # Create wildcard block rule
        user.add_rule(profile_id, "block", "*.facebook.com")

        # facebook.com itself should be blocked
        resp_root = await user.wait_for(profile_id, "facebook.com", A, is_blocked)
        assert_blocked(resp_root, "facebook.com")

        # www.facebook.com should also be blocked
        resp_www = await user.resolve(profile_id, "www.facebook.com", A)
        assert_blocked(resp_www, "www.facebook.com")

    @pytest.mark.asyncio
    async def test_dot_prefix_custom_block_blocks_www_subdomain(self, user):
        """Verify that the dot-prefix syntax .facebook.com blocks www.facebook.com.

        The ".facebook.com" syntax is equivalent to "*.facebook.com" -- it blocks
        the root domain and all subdomains.
        """
        profile_id = user.new_profile("test_dot_prefix_block_www")

        # Create dot-prefix block rule
        user.add_rule(profile_id, "block", ".facebook.com")

        # facebook.com itself should be blocked
        resp_root = await user.wait_for(profile_id, "facebook.com", A, is_blocked)
        assert_blocked(resp_root, "facebook.com")

        # www.facebook.com should also be blocked
        resp_www = await user.resolve(profile_id, "www.facebook.com", A)
        assert_blocked(resp_www, "www.facebook.com")

    @pytest.mark.asyncio
    @pytest.mark.parametrize(
        "pattern,subdomain,expect_blocked",
        [
            ("facebook.com", "www.facebook.com", False),
            ("facebook.com", "ads.facebook.com", False),
            ("*.facebook.com", "www.facebook.com", True),
            ("*.facebook.com", "ads.facebook.com", True),
            (".facebook.com", "www.facebook.com", True),
            (".facebook.com", "m.facebook.com", True),
        ],
        ids=[
            "exact-no-www",
            "exact-no-ads",
            "wildcard-www",
            "wildcard-ads",
            "dot-www",
            "dot-mobile",
        ],
    )
    async def test_custom_block_subdomain_matching_matrix(
        self, user, pattern, subdomain, expect_blocked
    ):
        """Parametrized matrix: which custom rule patterns block which subdomains.

        Uses "exact" mode so that plain domains are stored as-is without
        auto-prepend.  This tests the proxy's pattern matching semantics:
        exact rules ("facebook.com") only block the exact domain, while
        wildcard ("*.facebook.com") and dot-prefix (".facebook.com") block
        the root domain and all subdomains.
        """
        profile_id = user.new_profile(f"test_matrix_{pattern}_{subdomain}")

        # Use "exact" mode so pattern matching is tested without auto-prepend
        user.patch_setting(
            profile_id, "/settings/privacy/custom_rules_subdomains_rule", "exact"
        )

        user.add_rule(profile_id, "block", pattern)

        if expect_blocked:
            resp = await user.wait_for(profile_id, subdomain, A, is_blocked)
        else:
            # NOTE: negative assertion — cannot poll; may read pre-mutation state (see DNSLib.wait_until docstring)
            resp = await user.resolve(profile_id, subdomain, A)

        if expect_blocked:
            assert_blocked(resp, subdomain)
        else:
            assert_not_blocked(resp, subdomain)

    # ------------------------------------------------------------------
    # custom_rules_subdomains_rule setting tests
    # ------------------------------------------------------------------

    @pytest.mark.asyncio
    async def test_include_mode_auto_prepends_wildcard(self, user):
        """Verify that "include" mode (default) auto-expands plain domains to block subdomains.

        When custom_rules_subdomains_rule is "include", adding "facebook.com" should
        store "*.facebook.com" and therefore block www.facebook.com.
        """
        profile_id = user.new_profile("test_include_mode_auto_prepend")

        # Default is "include" -- no need to explicitly set it
        user.add_rule(profile_id, "block", "facebook.com")

        # facebook.com itself should be blocked
        resp_root = await user.wait_for(profile_id, "facebook.com", A, is_blocked)
        assert_blocked(resp_root, "facebook.com")

        # www.facebook.com should also be blocked (auto-prepend made it *.facebook.com)
        resp_www = await user.resolve(profile_id, "www.facebook.com", A)
        assert_blocked(resp_www, "www.facebook.com")

    @pytest.mark.asyncio
    async def test_exact_mode_does_not_block_subdomain(self, user):
        """Verify that "exact" mode stores plain domains as-is without wildcard expansion.

        When custom_rules_subdomains_rule is "exact", adding "facebook.com" should
        only block the exact domain, not www.facebook.com.
        """
        profile_id = user.new_profile("test_exact_mode_no_subdomain")

        user.patch_setting(
            profile_id, "/settings/privacy/custom_rules_subdomains_rule", "exact"
        )

        user.add_rule(profile_id, "block", "facebook.com")

        # facebook.com itself should be blocked
        resp_root = await user.wait_for(profile_id, "facebook.com", A, is_blocked)
        assert_blocked(resp_root, "facebook.com")

        # www.facebook.com should NOT be blocked (exact match only)
        resp_www = await user.resolve(profile_id, "www.facebook.com", A)
        assert_not_blocked(resp_www, "www.facebook.com")

    @pytest.mark.asyncio
    async def test_custom_rules_subdomains_rule_setting_patch(self, user):
        """Verify that the custom_rules_subdomains_rule setting can be toggled via PATCH API.

        Steps:
          1. Create a profile (default "include")
          2. Verify the setting is "include" via GET
          3. PATCH to "exact"
          4. Verify the setting is "exact" via GET
          5. PATCH back to "include"
          6. Verify the setting is "include" via GET
        """
        profile_id = user.new_profile("test_setting_patch")

        # Step 1: Verify default is "include"
        profile = user.get_profile(profile_id)
        assert (
            profile.settings.privacy.custom_rules_subdomains_rule == "include"
        ), "Default custom_rules_subdomains_rule should be 'include'"

        # Step 2: PATCH to "exact"
        user.patch_setting(
            profile_id, "/settings/privacy/custom_rules_subdomains_rule", "exact"
        )
        profile = user.get_profile(profile_id)
        assert (
            profile.settings.privacy.custom_rules_subdomains_rule == "exact"
        ), "custom_rules_subdomains_rule should be 'exact' after PATCH"

        # Step 3: PATCH back to "include"
        user.patch_setting(
            profile_id, "/settings/privacy/custom_rules_subdomains_rule", "include"
        )
        profile = user.get_profile(profile_id)
        assert (
            profile.settings.privacy.custom_rules_subdomains_rule == "include"
        ), "custom_rules_subdomains_rule should be 'include' after toggling back"
