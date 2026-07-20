from ipaddress import ip_address

import pytest
from libs.dns_lib import DNSLib, is_blocked, is_resolved
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

# Import shared test constants & fixture (fixture auto-discovered by pytest, constants used directly)
from conftest import TEST_BLOCKLIST_ID, TEST_DOMAIN, TEST_SUBDOMAIN  # noqa: F401


class TestBlocklistFilters:
    """
    Test cases for DNS blocklist functionality.
    """

    def setup_class(self):
        """Setup the test class."""
        self.config = get_settings()
        self.api_config = api_config.Configuration(host=self.config.DNS_API_ADDR)
        self.dns_lib = DNSLib(self.config.DOH_ENDPOINT)
        self.redis_client = redis.Redis(
            host=self.config.REDIS_HOST, port=self.config.REDIS_PORT, db=0
        )

    def test_threat_intelligence_feeds_blocklist(
        self, create_account_and_login, ensure_test_blocklisted
    ):
        """
        Test that the Threat Intelligence Feeds blocklist is enabled by default.
        """
        blocklist_set = f"blocklist:{TEST_BLOCKLIST_ID}"
        assert self.redis_client.sismember(
            blocklist_set, TEST_DOMAIN
        ), f'"{TEST_DOMAIN}" is not present in Redis set {blocklist_set}'

        account, cookie = create_account_and_login
        with client.ApiClient(self.api_config) as api_client:
            profiles_instance = api.ProfileApi(api_client)
            profile_id = account.profiles[0]

            profiles_instance.api_client.default_headers["Cookie"] = cookie
            resp = profiles_instance.api_v1_profiles_id_get_with_http_info(
                id=profile_id
            )
            assert (
                resp.status_code == 200
            ), f"Failed to get profile ID {profile_id} with status code: {resp.status_code}"
            assert (
                len(resp.data.settings.privacy.blocklists) == 1
            ), "Threat Intelligence Feeds blocklist is not enabled for profile"
            assert (
                resp.data.settings.privacy.blocklists[0] == TEST_BLOCKLIST_ID
            ), "Threat Intelligence Feeds blocklist is not enabled for profile"

    @pytest.mark.asyncio
    @pytest.mark.parametrize(
        "domain,expected_blocked",
        [
            (TEST_DOMAIN, True),
            ("example.com", False),
        ],
    )
    async def test_blocklist_blocking(
        self,
        create_account_and_login,
        domain,
        expected_blocked,
        ensure_test_blocklisted,
    ):
        """Test that domains in the blocklist are blocked and others are not."""
        account, cookie = create_account_and_login
        with client.ApiClient(self.api_config) as api_client:
            profiles_instance = api.ProfileApi(api_client)
            profile_id = account.profiles[0]

            profiles_instance.api_client.default_headers["Cookie"] = cookie
            resp = profiles_instance.api_v1_profiles_id_get_with_http_info(
                id=profile_id
            )
            assert (
                resp.status_code == 200
            ), f"Failed to get profile ID {profile_id} with status code: {resp.status_code}"
            assert (
                len(resp.data.settings.privacy.blocklists) == 1
            ), "Threat Intelligence Feeds blocklist is not enabled for profile"
            assert (
                resp.data.settings.privacy.blocklists[0] == TEST_BLOCKLIST_ID
            ), "Threat Intelligence Feeds blocklist is not enabled for profile"

        if expected_blocked:
            resp = await self.dns_lib.wait_until(profile_id, domain, A, is_blocked)
        else:
            # NOTE: negative assertion — cannot poll; may read pre-mutation state (see DNSLib.wait_until docstring)
            resp = await self.dns_lib.send_doh_request(profile_id, domain, A)
        ip_addr = resp.answer[0].to_text().split(" ")[-1]
        if expected_blocked:
            assert (
                ip_addr == "0.0.0.0"
            ), f"Blocklisted domain {domain} did not return 0.0.0.0"
        else:
            assert ip_address(
                ip_addr
            ), f"Non-blocklisted domain {domain} did not return a valid IP"

    @pytest.mark.asyncio
    async def test_blocklist_disable_unblocks_domain(
        self, create_account_and_login, ensure_test_blocklisted
    ):
        """Test that disabling the blocklist unblocks a previously blocked domain."""
        account, cookie = create_account_and_login
        with client.ApiClient(self.api_config) as api_client:
            profiles_instance = api.ProfileApi(api_client)
            # Fresh profile: this test disables the blocklist and must not
            # mutate the shared class profile other tests assert against.
            profiles_instance.api_client.default_headers["Cookie"] = cookie
            create_resp = profiles_instance.api_v1_profiles_post_with_http_info(
                body=ApiCreateProfileBody(name="bl_disable_test")
            )
            assert (
                create_resp.status_code == 201
            ), f"Failed to create profile with status code: {create_resp.status_code}"
            profile_id = create_resp.data.profile_id

            resp = await self.dns_lib.wait_until(profile_id, TEST_DOMAIN, A, is_blocked)
            ip_addr = resp.answer[0].to_text().split(" ")[-1]
            assert (
                ip_addr == "0.0.0.0"
            ), f"Blocklisted domain {TEST_DOMAIN} did not return 0.0.0.0"

            profiles_instance.api_client.default_headers["Cookie"] = cookie
            disable_body = ApiBlocklistsUpdates(blocklist_ids=[TEST_BLOCKLIST_ID])
            disable_resp = (
                profiles_instance.api_v1_profiles_id_blocklists_delete_with_http_info(
                    id=profile_id, blocklist_ids=disable_body
                )
            )
            assert (
                disable_resp.status_code == 200
            ), f"Failed to disable blocklist with status code: {disable_resp.status_code}"

            get_resp = profiles_instance.api_v1_profiles_id_get_with_http_info(
                id=profile_id
            )
            assert (
                get_resp.status_code == 200
            ), f"Failed to get profile with status code: {get_resp.status_code}"
            assert (
                len(get_resp.data.settings.privacy.blocklists) == 0
            ), "Blocklist still enabled after disabling"

            resp2 = await self.dns_lib.wait_until(profile_id, TEST_DOMAIN, A, is_resolved)
            ip_addr2 = resp2.answer[0].to_text().split(" ")[-1]
            assert (
                ip_address(ip_addr2) and ip_addr2 != "0.0.0.0"
            ), f"Domain {TEST_DOMAIN} still blocked after disabling blocklist"

    @pytest.mark.asyncio
    async def test_blocklist_subdomain_behavior(
        self, create_account_and_login, ensure_test_blocklisted
    ):
        """Test blocklist default subdomain blocking behavior."""
        _, cookie = create_account_and_login
        with client.ApiClient(self.api_config) as api_client:
            profiles_instance = api.ProfileApi(api_client)
            profiles_instance.api_client.default_headers["Cookie"] = cookie
            body = ApiCreateProfileBody(name="test_profile")
            resp = profiles_instance.api_v1_profiles_post_with_http_info(body=body)
            assert (
                resp.status_code == 201
            ), f"Failed to create profile with status code: {resp.status_code}"
            profile_id = resp.data.profile_id

            # Parent domain should be blocked
            resp_parent = await self.dns_lib.wait_until(
                profile_id, TEST_DOMAIN, A, is_blocked
            )
            ip_parent = resp_parent.answer[0].to_text().split(" ")[-1]
            assert (
                ip_parent == "0.0.0.0"
            ), f"Blocklisted parent domain {TEST_DOMAIN} did not return 0.0.0.0"

            # Subdomain should be blocked when subdomain blocking rule is active by default (added explicitly as entry)
            resp_sub = await self.dns_lib.send_doh_request(
                profile_id, TEST_SUBDOMAIN, A
            )
            ip_sub = resp_sub.answer[0].to_text().split(" ")[-1]
            assert (
                ip_sub == "0.0.0.0"
            ), f"Blocklisted subdomain {TEST_SUBDOMAIN} did not return 0.0.0.0"
