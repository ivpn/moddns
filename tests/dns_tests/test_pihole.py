"""E2E: a real Pi-hole in front of modDNS via dnscrypt-proxy and the DoH stamp.

Pi-hole forwards plain DNS only. The documented way to use it with modDNS is a
local dnscrypt-proxy fed the profile's DoH stamp (Setup → Routers → DNS Stamps:
"Pi-hole (via dnscrypt-proxy)"). The `pihole` container in docker-compose.yml
forwards to 127.0.0.1#5353, where libs/pihole.py starts that dnscrypt-proxy with
the stamp under test. Pi-hole's own gravity blocking keeps its defaults; the
domains here are on no list, so every verdict below comes from modDNS.
"""
from __future__ import annotations

import pytest
from dns.rdatatype import A

from dns.message import Message
from dns.rcode import NXDOMAIN

from libs.constants import RESOLVABLE_TEST_DOMAIN
from libs.dns_lib import is_blocked, is_resolved
from libs.dnscrypt_proxy import resolve_binary
from libs.pihole import Pihole
from libs.settings import get_settings
from libs.stamps import fetch_stamps, with_address


@pytest.fixture(scope="module")
def pihole() -> Pihole:
    cfg = get_settings()
    client = Pihole(cfg.PIHOLE_DNS_ADDR, dnscrypt_binary=resolve_binary())
    client.wait_ready()
    return client


def blocked_behind_pihole(resp: Message) -> bool:
    """What a client sees when modDNS blocks: Pi-hole recognises the upstream
    0.0.0.0 answer as an external block ("Blocked (external, IP)" in its query
    log) and hands the client NXDOMAIN instead of passing the sentinel through."""
    return resp.rcode() == NXDOMAIN or is_blocked(resp)


def _doh_stamp(user, profile_id: str) -> str:
    """The product's DoH stamp, re-addressed to the proxy's compose address."""
    return with_address(fetch_stamps(user, profile_id)["doh"], get_settings().PROXY_NETWORK_ADDR)


@pytest.mark.integration
class TestPihole:
    """Real Pi-hole → dnscrypt-proxy (DoH stamp) → modDNS."""

    @pytest.mark.asyncio
    async def test_resolves_via_pihole(self, user, pihole):
        """specRef: M1, M4 — Pi-hole answers from modDNS through the stamp."""
        pid = user.new_profile("pihole-resolve")
        await user.wait_for(pid, RESOLVABLE_TEST_DOMAIN, A, is_resolved)

        pihole.use_stamp(_doh_stamp(user, pid))
        resp = pihole.wait_until(RESOLVABLE_TEST_DOMAIN, is_resolved)
        assert is_resolved(resp), (
            f"Pi-hole did not resolve {RESOLVABLE_TEST_DOMAIN} via modDNS "
            f"(rcode={resp.rcode()})\n{pihole.dnscrypt_log()}"
        )

    @pytest.mark.asyncio
    async def test_per_profile_block_applies(self, user, pihole):
        """specRef: M4 — the profile in the stamp's DoH path is the one filtered."""
        pid = user.new_profile("pihole-block")
        domain = "pihole-block.test"
        user.add_rule(pid, "block", domain)
        # Barrier (positive condition): block visible on the replica first.
        await user.wait_for(pid, domain, A, is_blocked)

        pihole.use_stamp(_doh_stamp(user, pid))
        resp = pihole.wait_until(domain, blocked_behind_pihole)
        assert blocked_behind_pihole(resp), (
            f"{domain} was not blocked via Pi-hole (rcode={resp.rcode()}, answer={resp.answer})"
        )
