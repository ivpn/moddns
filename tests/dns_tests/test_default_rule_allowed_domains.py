import pytest
from dns.rdatatype import A

from libs.constants import (
    RESOLVABLE_TEST_DOMAIN,
    SERVICE_ALLOWED_DOMAIN,
    SERVICE_ALLOWED_IP,
)
from libs.dns_lib import answer_ip_is, assert_blocked, is_blocked


def _service_rule_id(user, profile_id: str) -> str:
    rules = user.get_profile(profile_id).settings.custom_rules or []
    matches = [r.id for r in rules if r.value == SERVICE_ALLOWED_DOMAIN and r.action == "allow"]
    assert len(matches) == 1, f"expected one allow rule for {SERVICE_ALLOWED_DOMAIN}, got {matches}"
    return matches[0]


class TestDefaultRuleAllowedDomains:
    """api-endpoint-behaviour.md G24: with default_rule = block the service's own
    hostnames (SERVER_ALLOWED_DOMAINS) stay resolvable through allow custom rules the API
    writes to Redis, the store the proxy reads custom rules from."""

    @pytest.mark.asyncio
    async def test_default_block_keeps_service_hostnames_resolvable(self, user, redis_client):
        """specRef: api-endpoint-behaviour.md G24"""
        profile_id = user.new_profile("default_block_allowed")
        user.patch_setting(profile_id, "/settings/privacy/default_rule", "block")

        # Written by the PATCH itself, not left to the reconciliation (G25).
        rule_key = f"settings:{profile_id}:custom_rule:{_service_rule_id(user, profile_id)}"
        assert redis_client.sismember(f"settings:{profile_id}:custom_rules", rule_key)
        assert redis_client.hget(rule_key, "value") == SERVICE_ALLOWED_DOMAIN.encode()

        resp = await user.wait_for(profile_id, RESOLVABLE_TEST_DOMAIN, A, is_blocked)
        assert_blocked(resp, RESOLVABLE_TEST_DOMAIN)

        resp = await user.wait_for(profile_id, SERVICE_ALLOWED_DOMAIN, A, answer_ip_is(SERVICE_ALLOWED_IP))
        assert answer_ip_is(SERVICE_ALLOWED_IP)(resp), (
            f"{SERVICE_ALLOWED_DOMAIN} must resolve under default_rule=block; got {resp.answer}"
        )
