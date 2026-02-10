from ipaddress import ip_address

import pytest
from libs.dns_lib import DNSLib
from libs.settings import get_settings
from dns.rdatatype import A
import redis

import moddns.api_client as client
import moddns.api as api
import moddns.configuration as api_config
from moddns import (
    RequestsProfileUpdates,
    ModelProfileUpdate,
    RequestsCreateProfileCustomRuleBody,
    ApiCreateProfileBody,
)

from conftest import TEST_BLOCKLIST_ID, TEST_DOMAIN, TEST_SUBDOMAIN


class TestCustomRulesPrecedence:
    """
    End-to-end integration tests verifying that custom rules take precedence
    over blocklist blocking and default_rule settings.

    The DNS proxy evaluates filtering tiers in priority order:
        CustomRules (tier 200) > Blocklists (tier 100) > DefaultRule (tier 0)

    Each test creates an isolated profile to avoid cross-test interference.
    """

    def setup_class(self):
        """Setup the test class."""
        self.config = get_settings()
        self.api_config = api_config.Configuration(host=self.config.DNS_API_ADDR)
        self.dns_lib = DNSLib(self.config.DOH_ENDPOINT)
        self.redis_client = redis.Redis(host="localhost", port=6379, db=0)

    def _create_profile(self, profiles_instance, name):
        """Helper to create a new profile and return its ID."""
        body = ApiCreateProfileBody(name=name)
        resp = profiles_instance.api_v1_profiles_post_with_http_info(body=body)
        assert (
            resp.status_code == 201
        ), f"Failed to create profile with status code: {resp.status_code}"
        return resp.data.profile_id

    def _create_custom_rule(self, profiles_instance, profile_id, action, value):
        """Helper to create a custom rule on a profile."""
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

    def _set_default_rule(self, profiles_instance, profile_id, rule_value):
        """Helper to set the default_rule on a profile via PATCH."""
        update_request = RequestsProfileUpdates(
            updates=[
                ModelProfileUpdate(
                    operation="replace",
                    path="/settings/privacy/default_rule",
                    value={"value": rule_value},
                )
            ]
        )
        resp = profiles_instance.api_v1_profiles_id_patch_with_http_info(
            profile_id, body=update_request
        )
        assert (
            resp.status_code == 200
        ), f"Profile default_rule update failed with status code: {resp.status_code}"
        return resp

    def _set_custom_rules_subdomains_rule(self, profiles_instance, profile_id, value):
        """Helper to set the custom_rules_subdomains_rule setting on a profile via PATCH.

        Args:
            value: "include" (auto-prepend *. to plain FQDNs) or "exact" (store as-is).
        """
        update_request = RequestsProfileUpdates(
            updates=[
                ModelProfileUpdate(
                    operation="replace",
                    path="/settings/privacy/custom_rules_subdomains_rule",
                    value={"value": value},
                )
            ]
        )
        resp = profiles_instance.api_v1_profiles_id_patch_with_http_info(
            profile_id, body=update_request
        )
        assert (
            resp.status_code == 200
        ), f"Profile custom_rules_subdomains_rule update failed with status code: {resp.status_code}"
        return resp

    @pytest.mark.asyncio
    async def test_custom_allow_overrides_blocklist_block(
        self, create_account_and_login, ensure_test_blocklisted
    ):
        """Verify that a custom 'allow' rule overrides a blocklist 'block' for the same domain.

        Setup:
            - example.com is present in the blocklist (via ensure_test_blocklisted fixture).
            - A custom allow rule is created for example.com on a fresh profile.

        Expected:
            - The DNS query for example.com returns a valid IP (not 0.0.0.0)
              because CustomRules tier (200) takes precedence over Blocklists tier (100).
        """
        account, cookie = create_account_and_login
        with client.ApiClient(self.api_config) as api_client:
            profiles_instance = api.ProfileApi(api_client)
            profiles_instance.api_client.default_headers["Cookie"] = cookie

            profile_id = self._create_profile(
                profiles_instance, "test_allow_overrides_blocklist"
            )

            # Confirm the domain is blocked by the blocklist before adding the custom rule
            resp_blocked = await self.dns_lib.send_doh_request(
                profile_id, TEST_DOMAIN, A
            )
            ip_blocked = resp_blocked.answer[0].to_text().split(" ")[-1]
            assert (
                ip_blocked == "0.0.0.0"
            ), f"Expected {TEST_DOMAIN} to be blocked by blocklist, got {ip_blocked}"

            # Create custom allow rule for the blocklisted domain
            self._create_custom_rule(
                profiles_instance, profile_id, "allow", TEST_DOMAIN
            )

            # Query again -- custom allow should override blocklist block
            resp = await self.dns_lib.send_doh_request(profile_id, TEST_DOMAIN, A)
            assert resp.answer, f"Expected an answer for {TEST_DOMAIN}"
            ip_addr = resp.answer[0].to_text().split(" ")[-1]
            assert ip_addr != "0.0.0.0", (
                f"Custom allow rule did not override blocklist block for {TEST_DOMAIN}; "
                f"got {ip_addr}"
            )
            assert ip_address(ip_addr), f"Expected a valid IP, got {ip_addr}"

    @pytest.mark.asyncio
    async def test_custom_allow_overrides_subdomain_blocklist_block(
        self, create_account_and_login, ensure_test_blocklisted
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
        account, cookie = create_account_and_login
        with client.ApiClient(self.api_config) as api_client:
            profiles_instance = api.ProfileApi(api_client)
            profiles_instance.api_client.default_headers["Cookie"] = cookie

            profile_id = self._create_profile(
                profiles_instance, "test_allow_overrides_subdomain_blocklist"
            )

            # Confirm subdomain is blocked by inherited blocklist rule
            resp_blocked = await self.dns_lib.send_doh_request(
                profile_id, TEST_SUBDOMAIN, A
            )
            ip_blocked = resp_blocked.answer[0].to_text().split(" ")[-1]
            assert (
                ip_blocked == "0.0.0.0"
            ), f"Expected {TEST_SUBDOMAIN} to be blocked by blocklist, got {ip_blocked}"

            # Create custom allow rule for the exact subdomain
            self._create_custom_rule(
                profiles_instance, profile_id, "allow", TEST_SUBDOMAIN
            )

            # Query again -- custom allow should override subdomain blocklist match.
            # Note: sub.example.com may not exist in DNS (NXDOMAIN / empty answer),
            # which is fine -- we only verify it's not actively blocked (0.0.0.0).
            resp = await self.dns_lib.send_doh_request(profile_id, TEST_SUBDOMAIN, A)
            if resp.answer:
                ip_addr = resp.answer[0].to_text().split(" ")[-1]
                assert ip_addr != "0.0.0.0", (
                    f"Custom allow rule did not override subdomain blocklist block for "
                    f"{TEST_SUBDOMAIN}; got {ip_addr}"
                )

    @pytest.mark.asyncio
    async def test_custom_wildcard_allow_overrides_blocklist(
        self, create_account_and_login, ensure_test_blocklisted
    ):
        """Verify that a wildcard custom 'allow' rule overrides blocklist blocking for subdomains.

        Setup:
            - example.com is in the blocklist (sub.example.com is blocked by inheritance).
            - A custom allow rule is created for *.example.com (wildcard).

        Expected:
            - The DNS query for sub.example.com returns a valid IP (not 0.0.0.0)
              because the wildcard custom allow rule matches and overrides the blocklist.
        """
        account, cookie = create_account_and_login
        with client.ApiClient(self.api_config) as api_client:
            profiles_instance = api.ProfileApi(api_client)
            profiles_instance.api_client.default_headers["Cookie"] = cookie

            profile_id = self._create_profile(
                profiles_instance, "test_wildcard_allow_overrides_blocklist"
            )

            # Confirm subdomain is blocked before adding wildcard allow
            resp_blocked = await self.dns_lib.send_doh_request(
                profile_id, TEST_SUBDOMAIN, A
            )
            ip_blocked = resp_blocked.answer[0].to_text().split(" ")[-1]
            assert (
                ip_blocked == "0.0.0.0"
            ), f"Expected {TEST_SUBDOMAIN} to be blocked by blocklist, got {ip_blocked}"

            # Create wildcard custom allow rule
            self._create_custom_rule(
                profiles_instance, profile_id, "allow", f"*.{TEST_DOMAIN}"
            )

            # Query subdomain -- wildcard allow should override blocklist.
            # Note: sub.example.com may not exist in DNS (NXDOMAIN / empty answer),
            # which is fine -- we only verify it's not actively blocked (0.0.0.0).
            resp = await self.dns_lib.send_doh_request(profile_id, TEST_SUBDOMAIN, A)
            if resp.answer:
                ip_addr = resp.answer[0].to_text().split(" ")[-1]
                assert ip_addr != "0.0.0.0", (
                    f"Wildcard custom allow rule did not override blocklist block for "
                    f"{TEST_SUBDOMAIN}; got {ip_addr}"
                )

    @pytest.mark.asyncio
    async def test_custom_block_on_non_blocklisted_domain(
        self, create_account_and_login
    ):
        """Verify that a custom 'block' rule blocks a domain that is not in any blocklist.

        Setup:
            - A new profile with default_rule = 'allow' (default behavior).
            - A custom block rule is created for facebook.com.

        Expected:
            - The DNS query for facebook.com returns 0.0.0.0 (blocked by custom rule),
              independent of any blocklist configuration.
        """
        account, cookie = create_account_and_login
        with client.ApiClient(self.api_config) as api_client:
            profiles_instance = api.ProfileApi(api_client)
            profiles_instance.api_client.default_headers["Cookie"] = cookie

            profile_id = self._create_profile(
                profiles_instance, "test_custom_block_non_blocklisted"
            )

            # Create custom block rule for a domain not in any blocklist
            self._create_custom_rule(
                profiles_instance, profile_id, "block", "facebook.com"
            )

            resp = await self.dns_lib.send_doh_request(profile_id, "facebook.com", A)
            assert resp.answer, "Expected a blocked answer for facebook.com"
            ip_addr = resp.answer[0].to_text().split(" ")[-1]
            assert (
                ip_addr == "0.0.0.0"
            ), f"Custom block rule did not block facebook.com; got {ip_addr}"

    @pytest.mark.asyncio
    async def test_default_block_rule_blocks_all(self, create_account_and_login):
        """Verify that setting default_rule to 'block' blocks all domains.

        Setup:
            - A new profile with default_rule set to 'block' via PATCH API.

        Expected:
            - Any DNS query (e.g., google.com) returns 0.0.0.0
              because the default rule blocks everything.
        """
        account, cookie = create_account_and_login
        with client.ApiClient(self.api_config) as api_client:
            profiles_instance = api.ProfileApi(api_client)
            profiles_instance.api_client.default_headers["Cookie"] = cookie

            profile_id = self._create_profile(
                profiles_instance, "test_default_block_all"
            )

            # Set default_rule to block
            self._set_default_rule(profiles_instance, profile_id, "block")

            resp = await self.dns_lib.send_doh_request(profile_id, "google.com", A)
            assert resp.answer, "Expected a blocked answer for google.com"
            ip_addr = resp.answer[0].to_text().split(" ")[-1]
            assert (
                ip_addr == "0.0.0.0"
            ), f"Default block rule did not block google.com; got {ip_addr}"

    @pytest.mark.asyncio
    async def test_custom_allow_overrides_default_block(
        self, create_account_and_login
    ):
        """Verify that a custom 'allow' rule overrides a default_rule of 'block'.

        Setup:
            - A new profile with default_rule set to 'block'.
            - A custom allow rule is created for facebook.com.

        Expected:
            - The DNS query for facebook.com returns a valid IP (not 0.0.0.0)
              because the custom allow rule (tier 200) overrides the default block rule (tier 0).
        """
        account, cookie = create_account_and_login
        with client.ApiClient(self.api_config) as api_client:
            profiles_instance = api.ProfileApi(api_client)
            profiles_instance.api_client.default_headers["Cookie"] = cookie

            profile_id = self._create_profile(
                profiles_instance, "test_allow_overrides_default_block"
            )

            # Set default_rule to block
            self._set_default_rule(profiles_instance, profile_id, "block")

            # Confirm facebook.com is blocked by default rule
            resp_blocked = await self.dns_lib.send_doh_request(
                profile_id, "facebook.com", A
            )
            ip_blocked = resp_blocked.answer[0].to_text().split(" ")[-1]
            assert (
                ip_blocked == "0.0.0.0"
            ), f"Expected facebook.com to be blocked by default rule, got {ip_blocked}"

            # Create custom allow rule for facebook.com
            self._create_custom_rule(
                profiles_instance, profile_id, "allow", "facebook.com"
            )

            # Query again -- custom allow should override default block
            resp = await self.dns_lib.send_doh_request(profile_id, "facebook.com", A)
            assert resp.answer, "Expected an answer for facebook.com"
            ip_addr = resp.answer[0].to_text().split(" ")[-1]
            assert ip_addr != "0.0.0.0", (
                f"Custom allow rule did not override default block for facebook.com; "
                f"got {ip_addr}"
            )
            assert ip_address(ip_addr), f"Expected a valid IP, got {ip_addr}"

    @pytest.mark.asyncio
    async def test_blocklist_block_with_default_block(
        self, create_account_and_login, ensure_test_blocklisted
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
        account, cookie = create_account_and_login
        with client.ApiClient(self.api_config) as api_client:
            profiles_instance = api.ProfileApi(api_client)
            profiles_instance.api_client.default_headers["Cookie"] = cookie

            profile_id = self._create_profile(
                profiles_instance, "test_blocklist_and_default_block"
            )

            # Set default_rule to block
            self._set_default_rule(profiles_instance, profile_id, "block")

            # Blocklisted domain should be blocked (both blocklist and default rule)
            resp_blocklisted = await self.dns_lib.send_doh_request(
                profile_id, TEST_DOMAIN, A
            )
            assert (
                resp_blocklisted.answer
            ), f"Expected a blocked answer for blocklisted {TEST_DOMAIN}"
            ip_blocklisted = resp_blocklisted.answer[0].to_text().split(" ")[-1]
            assert (
                ip_blocklisted == "0.0.0.0"
            ), f"Expected {TEST_DOMAIN} to be blocked, got {ip_blocklisted}"

            # Non-blocklisted domain should also be blocked (by default rule)
            resp_non_blocklisted = await self.dns_lib.send_doh_request(
                profile_id, "google.com", A
            )
            assert (
                resp_non_blocklisted.answer
            ), "Expected a blocked answer for google.com (default block rule)"
            ip_non_blocklisted = (
                resp_non_blocklisted.answer[0].to_text().split(" ")[-1]
            )
            assert (
                ip_non_blocklisted == "0.0.0.0"
            ), f"Expected google.com to be blocked by default rule, got {ip_non_blocklisted}"

    # ------------------------------------------------------------------
    # Custom rule subdomain matching tests
    # ------------------------------------------------------------------

    @pytest.mark.asyncio
    async def test_exact_custom_block_does_not_block_www_subdomain(
        self, create_account_and_login
    ):
        """Verify that an exact custom block rule does NOT block www.<domain>.

        When custom_rules_subdomains_rule is set to "exact", a rule for
        "facebook.com" (no wildcard) must only block the exact domain,
        not www.facebook.com.

        Wildcards (*.facebook.com or .facebook.com) are required to also
        cover subdomains when using exact mode.
        """
        account, cookie = create_account_and_login
        with client.ApiClient(self.api_config) as api_client:
            profiles_instance = api.ProfileApi(api_client)
            profiles_instance.api_client.default_headers["Cookie"] = cookie

            profile_id = self._create_profile(
                profiles_instance, "test_exact_block_no_www"
            )

            # Set custom_rules_subdomains_rule to "exact" so plain domains are not auto-expanded
            self._set_custom_rules_subdomains_rule(profiles_instance, profile_id, "exact")

            # Create exact block rule for facebook.com
            self._create_custom_rule(
                profiles_instance, profile_id, "block", "facebook.com"
            )

            # facebook.com itself should be blocked
            resp_exact = await self.dns_lib.send_doh_request(
                profile_id, "facebook.com", A
            )
            assert resp_exact.answer, "Expected a blocked answer for facebook.com"
            ip_exact = resp_exact.answer[0].to_text().split(" ")[-1]
            assert (
                ip_exact == "0.0.0.0"
            ), f"Exact custom block rule did not block facebook.com; got {ip_exact}"

            # www.facebook.com should NOT be blocked (exact match only)
            resp_www = await self.dns_lib.send_doh_request(
                profile_id, "www.facebook.com", A
            )
            assert resp_www.answer, "Expected an answer for www.facebook.com"
            ip_www = resp_www.answer[0].to_text().split(" ")[-1]
            assert ip_www != "0.0.0.0", (
                f"Exact custom block rule for facebook.com should NOT block "
                f"www.facebook.com; got {ip_www}"
            )

    @pytest.mark.asyncio
    async def test_wildcard_custom_block_blocks_www_subdomain(
        self, create_account_and_login
    ):
        """Verify that a wildcard custom block rule *.facebook.com blocks www.facebook.com.

        Unlike exact rules, the "*.facebook.com" pattern matches the root domain
        AND all subdomains (www.facebook.com, ads.facebook.com, etc.).
        """
        account, cookie = create_account_and_login
        with client.ApiClient(self.api_config) as api_client:
            profiles_instance = api.ProfileApi(api_client)
            profiles_instance.api_client.default_headers["Cookie"] = cookie

            profile_id = self._create_profile(
                profiles_instance, "test_wildcard_block_www"
            )

            # Create wildcard block rule
            self._create_custom_rule(
                profiles_instance, profile_id, "block", "*.facebook.com"
            )

            # facebook.com itself should be blocked
            resp_root = await self.dns_lib.send_doh_request(
                profile_id, "facebook.com", A
            )
            assert resp_root.answer, "Expected a blocked answer for facebook.com"
            ip_root = resp_root.answer[0].to_text().split(" ")[-1]
            assert (
                ip_root == "0.0.0.0"
            ), f"Wildcard block rule did not block facebook.com; got {ip_root}"

            # www.facebook.com should also be blocked
            resp_www = await self.dns_lib.send_doh_request(
                profile_id, "www.facebook.com", A
            )
            assert resp_www.answer, "Expected a blocked answer for www.facebook.com"
            ip_www = resp_www.answer[0].to_text().split(" ")[-1]
            assert (
                ip_www == "0.0.0.0"
            ), f"Wildcard block rule did not block www.facebook.com; got {ip_www}"

    @pytest.mark.asyncio
    async def test_dot_prefix_custom_block_blocks_www_subdomain(
        self, create_account_and_login
    ):
        """Verify that the dot-prefix syntax .facebook.com blocks www.facebook.com.

        The ".facebook.com" syntax is equivalent to "*.facebook.com" -- it blocks
        the root domain and all subdomains.
        """
        account, cookie = create_account_and_login
        with client.ApiClient(self.api_config) as api_client:
            profiles_instance = api.ProfileApi(api_client)
            profiles_instance.api_client.default_headers["Cookie"] = cookie

            profile_id = self._create_profile(
                profiles_instance, "test_dot_prefix_block_www"
            )

            # Create dot-prefix block rule
            self._create_custom_rule(
                profiles_instance, profile_id, "block", ".facebook.com"
            )

            # facebook.com itself should be blocked
            resp_root = await self.dns_lib.send_doh_request(
                profile_id, "facebook.com", A
            )
            assert resp_root.answer, "Expected a blocked answer for facebook.com"
            ip_root = resp_root.answer[0].to_text().split(" ")[-1]
            assert (
                ip_root == "0.0.0.0"
            ), f"Dot-prefix block rule did not block facebook.com; got {ip_root}"

            # www.facebook.com should also be blocked
            resp_www = await self.dns_lib.send_doh_request(
                profile_id, "www.facebook.com", A
            )
            assert resp_www.answer, "Expected a blocked answer for www.facebook.com"
            ip_www = resp_www.answer[0].to_text().split(" ")[-1]
            assert (
                ip_www == "0.0.0.0"
            ), f"Dot-prefix block rule did not block www.facebook.com; got {ip_www}"

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
        self, create_account_and_login, pattern, subdomain, expect_blocked
    ):
        """Parametrized matrix: which custom rule patterns block which subdomains.

        Uses "exact" mode so that plain domains are stored as-is without
        auto-prepend.  This tests the proxy's pattern matching semantics:
        exact rules ("facebook.com") only block the exact domain, while
        wildcard ("*.facebook.com") and dot-prefix (".facebook.com") block
        the root domain and all subdomains.
        """
        account, cookie = create_account_and_login
        with client.ApiClient(self.api_config) as api_client:
            profiles_instance = api.ProfileApi(api_client)
            profiles_instance.api_client.default_headers["Cookie"] = cookie

            profile_id = self._create_profile(
                profiles_instance, f"test_matrix_{pattern}_{subdomain}"
            )

            # Use "exact" mode so pattern matching is tested without auto-prepend
            self._set_custom_rules_subdomains_rule(profiles_instance, profile_id, "exact")

            self._create_custom_rule(
                profiles_instance, profile_id, "block", pattern
            )

            resp = await self.dns_lib.send_doh_request(profile_id, subdomain, A)

            if expect_blocked:
                assert resp.answer, f"Expected a blocked answer for {subdomain}"
                ip_addr = resp.answer[0].to_text().split(" ")[-1]
                assert ip_addr == "0.0.0.0", (
                    f"Pattern '{pattern}' should block {subdomain}; got {ip_addr}"
                )
            else:
                assert resp.answer, f"Expected an answer for {subdomain}"
                ip_addr = resp.answer[0].to_text().split(" ")[-1]
                assert ip_addr != "0.0.0.0", (
                    f"Pattern '{pattern}' should NOT block {subdomain}; got {ip_addr}"
                )

    # ------------------------------------------------------------------
    # custom_rules_subdomains_rule setting tests
    # ------------------------------------------------------------------

    @pytest.mark.asyncio
    async def test_include_mode_auto_prepends_wildcard(
        self, create_account_and_login
    ):
        """Verify that "include" mode (default) auto-expands plain domains to block subdomains.

        When custom_rules_subdomains_rule is "include", adding "facebook.com" should
        store "*.facebook.com" and therefore block www.facebook.com.
        """
        account, cookie = create_account_and_login
        with client.ApiClient(self.api_config) as api_client:
            profiles_instance = api.ProfileApi(api_client)
            profiles_instance.api_client.default_headers["Cookie"] = cookie

            profile_id = self._create_profile(
                profiles_instance, "test_include_mode_auto_prepend"
            )

            # Default is "include" -- no need to explicitly set it
            self._create_custom_rule(
                profiles_instance, profile_id, "block", "facebook.com"
            )

            # facebook.com itself should be blocked
            resp_root = await self.dns_lib.send_doh_request(
                profile_id, "facebook.com", A
            )
            assert resp_root.answer, "Expected a blocked answer for facebook.com"
            ip_root = resp_root.answer[0].to_text().split(" ")[-1]
            assert (
                ip_root == "0.0.0.0"
            ), f"Include mode did not block facebook.com; got {ip_root}"

            # www.facebook.com should also be blocked (auto-prepend made it *.facebook.com)
            resp_www = await self.dns_lib.send_doh_request(
                profile_id, "www.facebook.com", A
            )
            assert resp_www.answer, "Expected a blocked answer for www.facebook.com"
            ip_www = resp_www.answer[0].to_text().split(" ")[-1]
            assert ip_www == "0.0.0.0", (
                f"Include mode should block www.facebook.com via auto-prepended "
                f"wildcard; got {ip_www}"
            )

    @pytest.mark.asyncio
    async def test_exact_mode_does_not_block_subdomain(
        self, create_account_and_login
    ):
        """Verify that "exact" mode stores plain domains as-is without wildcard expansion.

        When custom_rules_subdomains_rule is "exact", adding "facebook.com" should
        only block the exact domain, not www.facebook.com.
        """
        account, cookie = create_account_and_login
        with client.ApiClient(self.api_config) as api_client:
            profiles_instance = api.ProfileApi(api_client)
            profiles_instance.api_client.default_headers["Cookie"] = cookie

            profile_id = self._create_profile(
                profiles_instance, "test_exact_mode_no_subdomain"
            )

            self._set_custom_rules_subdomains_rule(profiles_instance, profile_id, "exact")

            self._create_custom_rule(
                profiles_instance, profile_id, "block", "facebook.com"
            )

            # facebook.com itself should be blocked
            resp_root = await self.dns_lib.send_doh_request(
                profile_id, "facebook.com", A
            )
            assert resp_root.answer, "Expected a blocked answer for facebook.com"
            ip_root = resp_root.answer[0].to_text().split(" ")[-1]
            assert (
                ip_root == "0.0.0.0"
            ), f"Exact mode did not block facebook.com; got {ip_root}"

            # www.facebook.com should NOT be blocked (exact match only)
            resp_www = await self.dns_lib.send_doh_request(
                profile_id, "www.facebook.com", A
            )
            assert resp_www.answer, "Expected an answer for www.facebook.com"
            ip_www = resp_www.answer[0].to_text().split(" ")[-1]
            assert ip_www != "0.0.0.0", (
                f"Exact mode should NOT block www.facebook.com; got {ip_www}"
            )

    @pytest.mark.asyncio
    async def test_custom_rules_subdomains_rule_setting_patch(
        self, create_account_and_login
    ):
        """Verify that the custom_rules_subdomains_rule setting can be toggled via PATCH API.

        Steps:
          1. Create a profile (default "include")
          2. Verify the setting is "include" via GET
          3. PATCH to "exact"
          4. Verify the setting is "exact" via GET
          5. PATCH back to "include"
          6. Verify the setting is "include" via GET
        """
        account, cookie = create_account_and_login
        with client.ApiClient(self.api_config) as api_client:
            profiles_instance = api.ProfileApi(api_client)
            profiles_instance.api_client.default_headers["Cookie"] = cookie

            profile_id = self._create_profile(
                profiles_instance, "test_setting_patch"
            )

            # Step 1: Verify default is "include"
            resp = profiles_instance.api_v1_profiles_id_get_with_http_info(
                id=profile_id
            )
            assert resp.status_code == 200
            assert (
                resp.data.settings.privacy.custom_rules_subdomains_rule == "include"
            ), "Default custom_rules_subdomains_rule should be 'include'"

            # Step 2: PATCH to "exact"
            self._set_custom_rules_subdomains_rule(profiles_instance, profile_id, "exact")
            resp = profiles_instance.api_v1_profiles_id_get_with_http_info(
                id=profile_id
            )
            assert resp.status_code == 200
            assert (
                resp.data.settings.privacy.custom_rules_subdomains_rule == "exact"
            ), "custom_rules_subdomains_rule should be 'exact' after PATCH"

            # Step 3: PATCH back to "include"
            self._set_custom_rules_subdomains_rule(profiles_instance, profile_id, "include")
            resp = profiles_instance.api_v1_profiles_id_get_with_http_info(
                id=profile_id
            )
            assert resp.status_code == 200
            assert (
                resp.data.settings.privacy.custom_rules_subdomains_rule == "include"
            ), "custom_rules_subdomains_rule should be 'include' after toggling back"
