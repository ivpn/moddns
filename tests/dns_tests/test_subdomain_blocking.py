from ipaddress import ip_address
import uuid

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
    ApiCreateProfileBody,
    ApiBlocklistsUpdates,
)

from conftest import TEST_BLOCKLIST_ID, TEST_DOMAIN, TEST_SUBDOMAIN  # noqa: F401


def _is_blocked(resp) -> bool:
    """Return True when the DNS response indicates a blocked domain (0.0.0.0)."""
    if not resp.answer:
        return False
    ip_addr = resp.answer[0].to_text().split(" ")[-1]
    return ip_addr == "0.0.0.0"


def _is_not_blocked(resp) -> bool:
    """Return True when the DNS response does NOT indicate blocking.

    A domain is considered not-blocked when:
    - There is no answer section (NXDOMAIN / SERVFAIL), OR
    - The answer IP is anything other than 0.0.0.0
    """
    if not resp.answer:
        return True
    ip_addr = resp.answer[0].to_text().split(" ")[-1]
    return ip_addr != "0.0.0.0"


class TestSubdomainBlocking:
    """End-to-end tests for subdomain blocking behaviour in the DNS proxy.

    When a parent domain (e.g. example.com) is present in a blocklist the
    proxy should, by default, also block all its subdomains (sub.example.com,
    www.example.com, a.b.example.com, etc.).  A per-profile setting called
    ``blocklists_subdomains_rule`` controls this: ``"block"`` (default) means subdomains
    are blocked; ``"allow"`` means only the exact parent domain is blocked.
    """

    def setup_class(self):
        """Setup the test class."""
        self.config = get_settings()
        self.api_config = api_config.Configuration(host=self.config.DNS_API_ADDR)
        self.dns_lib = DNSLib(self.config.DOH_ENDPOINT)
        self.redis_client = redis.Redis(host="localhost", port=6379, db=0)

    # ------------------------------------------------------------------
    # Helpers
    # ------------------------------------------------------------------

    def _create_profile(self, cookie: str) -> str:
        """Create a fresh profile with a unique name and return its profile_id."""
        with client.ApiClient(self.api_config) as api_client:
            profiles_instance = api.ProfileApi(api_client)
            profiles_instance.api_client.default_headers["Cookie"] = cookie
            name = f"test_subdomain_{uuid.uuid4().hex[:8]}"
            body = ApiCreateProfileBody(name=name)
            resp = profiles_instance.api_v1_profiles_post_with_http_info(body=body)
            assert (
                resp.status_code == 201
            ), f"Failed to create profile with status code: {resp.status_code}"
            return resp.data.profile_id

    def _set_blocklists_subdomains_rule(self, cookie: str, profile_id: str, value: str) -> None:
        """PATCH the blocklists_subdomains_rule setting on *profile_id*."""
        with client.ApiClient(self.api_config) as api_client:
            profiles_instance = api.ProfileApi(api_client)
            profiles_instance.api_client.default_headers["Cookie"] = cookie
            update_request = RequestsProfileUpdates(
                updates=[
                    ModelProfileUpdate(
                        operation="replace",
                        path="/settings/privacy/blocklists_subdomains_rule",
                        value={"value": value},
                    )
                ]
            )
            resp = profiles_instance.api_v1_profiles_id_patch_with_http_info(
                profile_id, body=update_request
            )
            assert (
                resp.status_code == 200
            ), f"Failed to update blocklists_subdomains_rule to '{value}' with status code: {resp.status_code}"

    # ------------------------------------------------------------------
    # Tests
    # ------------------------------------------------------------------

    @pytest.mark.asyncio
    async def test_parent_domain_blocked(
        self, create_account_and_login, ensure_test_blocklisted
    ):
        """Verify that a domain explicitly present in the blocklist is blocked.

        This is the baseline: example.com is inserted into the blocklist via
        the ``ensure_test_blocklisted`` fixture and a DNS query for it must
        return 0.0.0.0.
        """
        _, cookie = create_account_and_login
        profile_id = self._create_profile(cookie)

        resp = await self.dns_lib.send_doh_request(profile_id, TEST_DOMAIN, A)
        assert _is_blocked(
            resp
        ), f"Blocklisted parent domain {TEST_DOMAIN} was not blocked (expected 0.0.0.0)"

    @pytest.mark.asyncio
    async def test_subdomain_blocked_by_default(
        self, create_account_and_login, ensure_test_blocklisted
    ):
        """Verify that subdomains are blocked when the parent is in the blocklist.

        This is the core regression test: sub.example.com is NOT inserted into
        the blocklist, yet it must be blocked because example.com is listed and
        the default blocklists_subdomains_rule is "block".
        """
        _, cookie = create_account_and_login
        profile_id = self._create_profile(cookie)

        resp = await self.dns_lib.send_doh_request(profile_id, TEST_SUBDOMAIN, A)
        assert _is_blocked(
            resp
        ), f"Subdomain {TEST_SUBDOMAIN} was not blocked by default (expected 0.0.0.0)"

    @pytest.mark.asyncio
    async def test_www_subdomain_blocked(
        self, create_account_and_login, ensure_test_blocklisted
    ):
        """Verify that www.<parent> is blocked when the parent is in the blocklist.

        Browsers commonly prepend ``www.`` to domains.  The proxy must treat
        www.example.com as a subdomain of the blocklisted example.com.
        """
        _, cookie = create_account_and_login
        profile_id = self._create_profile(cookie)

        domain = f"www.{TEST_DOMAIN}"
        resp = await self.dns_lib.send_doh_request(profile_id, domain, A)
        assert _is_blocked(
            resp
        ), f"www subdomain {domain} was not blocked (expected 0.0.0.0)"

    @pytest.mark.asyncio
    async def test_deep_subdomain_blocked(
        self, create_account_and_login, ensure_test_blocklisted
    ):
        """Verify that deeply-nested subdomains are blocked.

        a.b.example.com should still be blocked when example.com is in the
        blocklist and blocklists_subdomains_rule is "block" (default).
        """
        _, cookie = create_account_and_login
        profile_id = self._create_profile(cookie)

        domain = f"a.b.{TEST_DOMAIN}"
        resp = await self.dns_lib.send_doh_request(profile_id, domain, A)
        assert _is_blocked(
            resp
        ), f"Deep subdomain {domain} was not blocked (expected 0.0.0.0)"

    @pytest.mark.asyncio
    async def test_subdomain_allowed_when_rule_disabled(
        self, create_account_and_login, ensure_test_blocklisted
    ):
        """Verify that subdomains pass through when blocklists_subdomains_rule is "allow".

        When the profile setting is changed to "allow", only the exact parent
        domain (example.com) should be blocked.  sub.example.com must not be
        intercepted by the proxy.
        """
        _, cookie = create_account_and_login
        profile_id = self._create_profile(cookie)

        self._set_blocklists_subdomains_rule(cookie, profile_id, "allow")

        resp = await self.dns_lib.send_doh_request(profile_id, TEST_SUBDOMAIN, A)
        assert _is_not_blocked(
            resp
        ), f"Subdomain {TEST_SUBDOMAIN} was still blocked after setting blocklists_subdomains_rule to 'allow'"

    @pytest.mark.asyncio
    async def test_subdomain_rule_toggle(
        self, create_account_and_login, ensure_test_blocklisted
    ):
        """Verify that toggling blocklists_subdomains_rule takes effect dynamically.

        Steps:
          1. Default (block) -- subdomain query returns 0.0.0.0
          2. Switch to "allow" -- subdomain query is no longer blocked
          3. Switch back to "block" -- subdomain query returns 0.0.0.0 again
        """
        _, cookie = create_account_and_login
        profile_id = self._create_profile(cookie)

        # Step 1: default setting is "block"
        resp1 = await self.dns_lib.send_doh_request(profile_id, TEST_SUBDOMAIN, A)
        assert _is_blocked(
            resp1
        ), f"Step 1 failed: {TEST_SUBDOMAIN} should be blocked with default blocklists_subdomains_rule"

        # Step 2: switch to "allow"
        self._set_blocklists_subdomains_rule(cookie, profile_id, "allow")
        resp2 = await self.dns_lib.send_doh_request(profile_id, TEST_SUBDOMAIN, A)
        assert _is_not_blocked(
            resp2
        ), f"Step 2 failed: {TEST_SUBDOMAIN} should not be blocked after setting blocklists_subdomains_rule to 'allow'"

        # Step 3: switch back to "block"
        self._set_blocklists_subdomains_rule(cookie, profile_id, "block")
        resp3 = await self.dns_lib.send_doh_request(profile_id, TEST_SUBDOMAIN, A)
        assert _is_blocked(
            resp3
        ), f"Step 3 failed: {TEST_SUBDOMAIN} should be blocked again after restoring blocklists_subdomains_rule to 'block'"

    @pytest.mark.asyncio
    async def test_unrelated_domain_not_blocked(
        self, create_account_and_login, ensure_test_blocklisted
    ):
        """Verify that domains NOT in the blocklist are not affected.

        facebook.com is a well-known domain that is not present in the test
        blocklist.  A DNS query for it must return a valid, non-blocked IP.
        """
        _, cookie = create_account_and_login
        profile_id = self._create_profile(cookie)

        resp = await self.dns_lib.send_doh_request(profile_id, "facebook.com", A)
        assert resp.answer, "Expected an answer for unrelated domain facebook.com"
        ip_addr = resp.answer[0].to_text().split(" ")[-1]
        assert ip_address(ip_addr) != ip_address(
            "0.0.0.0"
        ), "Unrelated domain facebook.com should not be blocked"

    @pytest.mark.asyncio
    @pytest.mark.parametrize(
        "subdomain",
        [
            TEST_SUBDOMAIN,
            f"www.{TEST_DOMAIN}",
            f"deep.sub.{TEST_DOMAIN}",
        ],
        ids=["one-level", "www-prefix", "two-levels"],
    )
    async def test_multiple_subdomain_levels_blocked(
        self, create_account_and_login, ensure_test_blocklisted, subdomain
    ):
        """Parametrized: various subdomain depths are all blocked.

        When example.com is in the blocklist and blocklists_subdomains_rule is "block"
        (default), every subdomain regardless of depth must return 0.0.0.0.
        """
        _, cookie = create_account_and_login
        profile_id = self._create_profile(cookie)

        resp = await self.dns_lib.send_doh_request(profile_id, subdomain, A)
        assert _is_blocked(
            resp
        ), f"Subdomain {subdomain} was not blocked (expected 0.0.0.0)"
