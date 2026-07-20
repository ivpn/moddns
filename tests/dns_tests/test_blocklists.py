import pytest
from dns.rdatatype import A

from libs.constants import (
    BLOCKLISTED_DOMAIN,
    BLOCKLISTED_SUBDOMAIN,
    RESOLVABLE_TEST_DOMAIN,
    TEST_BLOCKLIST_ID,
)
from libs.dns_lib import assert_blocked, assert_not_blocked, is_blocked, is_resolved


class TestBlocklistFilters:
    """
    Test cases for DNS blocklist functionality.
    """

    def test_threat_intelligence_feeds_blocklist(
        self, user, ensure_test_blocklisted, redis_client
    ):
        """
        Test that the Threat Intelligence Feeds blocklist is enabled by default.
        """
        blocklist_set = f"blocklist:{TEST_BLOCKLIST_ID}"
        assert redis_client.sismember(
            blocklist_set, BLOCKLISTED_DOMAIN
        ), f'"{BLOCKLISTED_DOMAIN}" is not present in Redis set {blocklist_set}'

        profile = user.get_profile(user.default_profile_id)
        assert (
            len(profile.settings.privacy.blocklists) == 1
        ), "Threat Intelligence Feeds blocklist is not enabled for profile"
        assert (
            profile.settings.privacy.blocklists[0] == TEST_BLOCKLIST_ID
        ), "Threat Intelligence Feeds blocklist is not enabled for profile"

    @pytest.mark.asyncio
    @pytest.mark.parametrize(
        "domain,expected_blocked",
        [
            (BLOCKLISTED_DOMAIN, True),
            (RESOLVABLE_TEST_DOMAIN, False),
        ],
    )
    async def test_blocklist_blocking(
        self,
        user,
        domain,
        expected_blocked,
        ensure_test_blocklisted,
    ):
        """Test that domains in the blocklist are blocked and others are not."""
        profile_id = user.default_profile_id
        profile = user.get_profile(profile_id)
        assert (
            len(profile.settings.privacy.blocklists) == 1
        ), "Threat Intelligence Feeds blocklist is not enabled for profile"
        assert (
            profile.settings.privacy.blocklists[0] == TEST_BLOCKLIST_ID
        ), "Threat Intelligence Feeds blocklist is not enabled for profile"

        if expected_blocked:
            resp = await user.wait_for(profile_id, domain, A, is_blocked)
            assert_blocked(resp, domain)
        else:
            # NOTE: negative assertion — cannot poll; may read pre-mutation state (see DNSLib.wait_until docstring)
            resp = await user.resolve(profile_id, domain, A)
            assert_not_blocked(resp, domain)

    @pytest.mark.asyncio
    async def test_blocklist_disable_unblocks_domain(
        self, user, ensure_test_blocklisted
    ):
        """Test that disabling the blocklist unblocks a previously blocked domain."""
        # Fresh profile: this test disables the blocklist and must not
        # mutate the shared class profile other tests assert against.
        profile_id = user.new_profile("bl_disable")

        resp = await user.wait_for(profile_id, BLOCKLISTED_DOMAIN, A, is_blocked)
        assert_blocked(resp, BLOCKLISTED_DOMAIN)

        user.disable_blocklists(profile_id, [TEST_BLOCKLIST_ID])

        profile = user.get_profile(profile_id)
        assert (
            len(profile.settings.privacy.blocklists) == 0
        ), "Blocklist still enabled after disabling"

        resp2 = await user.wait_for(profile_id, BLOCKLISTED_DOMAIN, A, is_resolved)
        assert_not_blocked(resp2, BLOCKLISTED_DOMAIN)

    @pytest.mark.asyncio
    async def test_blocklist_subdomain_behavior(
        self, user, ensure_test_blocklisted
    ):
        """Test blocklist default subdomain blocking behavior."""
        profile_id = user.new_profile("test_profile")

        # Parent domain should be blocked
        resp_parent = await user.wait_for(profile_id, BLOCKLISTED_DOMAIN, A, is_blocked)
        assert_blocked(resp_parent, BLOCKLISTED_DOMAIN)

        # Subdomain should be blocked when subdomain blocking rule is active by default (added explicitly as entry)
        resp_sub = await user.resolve(profile_id, BLOCKLISTED_SUBDOMAIN, A)
        assert_blocked(resp_sub, BLOCKLISTED_SUBDOMAIN)
