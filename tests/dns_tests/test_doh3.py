"""DoH over HTTP/3 (RFC 9114) end to end.

DoH3 uses the same URL as DoH over HTTP/2 (``/dns-query/<profile>[/<device>]``);
only the transport differs. The proxy serves it on UDP at the DoH port when
``DOH_HTTP3_ENABLED`` is set (tests/config/proxy.env).

Spec: docs/specs/proxy-request-admission-behaviour.md Q6, Q15, Q16.
"""
from __future__ import annotations

from urllib.parse import urlparse

import pytest
from dns.message import ShortHeader
from dns.rdatatype import A

from libs.constants import RESOLVABLE_TEST_DOMAIN
from libs.dns_lib import assert_blocked, assert_not_blocked, is_blocked, is_resolved
from libs.settings import get_settings

# Only ever blocked by this test's own custom rule.
BLOCKED_DOMAIN = "doh3-isolation-block.test"
UNKNOWN_PROFILE = "zzzzzzzzz9"
DOH_DOMAIN = urlparse(get_settings().DOH_ENDPOINT).hostname


class TestDoH3:
    @pytest.mark.asyncio
    async def test_resolves_over_http3(self, user):
        """specRef: Q16 - the DoH URL answers over HTTP/3."""
        pid = user.new_profile("doh3-resolve")
        await user.wait_for(pid, RESOLVABLE_TEST_DOMAIN, A, is_resolved)

        resp = await user.dns.send_doh3_request(pid, RESOLVABLE_TEST_DOMAIN, "A")
        assert is_resolved(resp), f"{RESOLVABLE_TEST_DOMAIN} did not resolve over HTTP/3: {resp}"

    @pytest.mark.asyncio
    async def test_device_id_in_path(self, user):
        """specRef: Q15, Q16 - /dns-query/<profile>/<device> works over HTTP/3."""
        pid = user.new_profile("doh3-device")
        await user.wait_for(pid, RESOLVABLE_TEST_DOMAIN, A, is_resolved)

        resp = await user.dns.send_doh3_request(f"{pid}/Laptop", RESOLVABLE_TEST_DOMAIN, "A")
        assert is_resolved(resp)

    @pytest.mark.asyncio
    async def test_block_in_p1_does_not_affect_p2(self, user):
        """specRef: Q16 - per-profile filtering holds over HTTP/3, as over HTTP/2."""
        p1 = user.new_profile("doh3-iso-p1")
        p2 = user.new_profile("doh3-iso-p2")
        user.add_rule(p1, "block", BLOCKED_DOMAIN)

        # Positive conditions only, over HTTP/2: P1's rule blocks and P2 resolves.
        await user.wait_for(p1, BLOCKED_DOMAIN, A, is_blocked)
        await user.wait_for(p2, RESOLVABLE_TEST_DOMAIN, A, is_resolved)

        assert_blocked(await user.dns.send_doh3_request(p1, BLOCKED_DOMAIN, "A"), f"{BLOCKED_DOMAIN} via P1 over HTTP/3")
        assert_not_blocked(await user.dns.send_doh3_request(p2, BLOCKED_DOMAIN, "A"), f"{BLOCKED_DOMAIN} via P2 over HTTP/3")

    @pytest.mark.asyncio
    async def test_unknown_profile_gets_no_answer(self, user):
        """specRef: Q6, Q16 - a profile that does not exist is dropped over HTTP/3 too:
        the response carries no DNS message."""
        with pytest.raises(ShortHeader):
            await user.dns.send_doh3_request(UNKNOWN_PROFILE, RESOLVABLE_TEST_DOMAIN, "A")

    @pytest.mark.asyncio
    async def test_profile_in_server_name_is_ignored(self, user):
        """specRef: Q5, Q15 - over HTTP/3 the client's server name reaches the
        proxy, but DoH takes the profile only from the path."""
        pid = user.new_profile("doh3-sni")
        await user.wait_for(pid, RESOLVABLE_TEST_DOMAIN, A, is_resolved)

        with pytest.raises(ShortHeader):
            await user.dns.send_doh3_request("", RESOLVABLE_TEST_DOMAIN, "A", host=f"{pid}.{DOH_DOMAIN}")
