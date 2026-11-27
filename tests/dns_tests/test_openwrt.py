"""E2E: an OpenWrt router set up with the commands from Setup > Routers > OpenWrt.

Each router container (docker-compose.yml: OpenWrt 24.10 with opkg, 25.12 with
apk) runs the command block the app renders for the profile, verbatim, then a
LAN client queries the router's dnsmasq. https-dns-proxy looks up the DoH
hostname through its bootstrap resolvers with plain DNS, which modDNS does not
answer (Q5), so the guide must leave the bootstrap to resolvers that do.
"""
from __future__ import annotations

import pytest
from dns.message import Message
from dns.rcode import NOERROR
from dns.rdatatype import A

from libs.constants import RESOLVABLE_TEST_DOMAIN
from libs.dns_lib import is_blocked, is_resolved
from libs.openwrt import OpenWrtRouter, render_openwrt_commands
from libs.settings import get_settings

_ROUTERS = {
    "openwrt-24": lambda cfg: cfg.OPENWRT_24_DNS_ADDR,
    "openwrt-25": lambda cfg: cfg.OPENWRT_25_DNS_ADDR,
}


@pytest.fixture(scope="module", params=sorted(_ROUTERS))
def router(request) -> OpenWrtRouter:
    cfg = get_settings()
    r = OpenWrtRouter(request.param, _ROUTERS[request.param](cfg))
    r.wait_ready()
    return r


def blocked_behind_router(resp: Message) -> bool:
    """What a LAN client sees when modDNS blocks: dnsmasq's rebind protection
    (on by default) strips the 0.0.0.0 answer, leaving NOERROR with no address.
    Any resolver other than modDNS answers the nonexistent name with NXDOMAIN."""
    if is_blocked(resp):
        return True
    return resp.rcode() == NOERROR and not any(rrset.rdtype == A for rrset in resp.answer)


def apply_guide(router: OpenWrtRouter, pid: str) -> str:
    endpoint = f"{get_settings().DOH_ENDPOINT}{pid}"
    out = router.run_guide(render_openwrt_commands(endpoint))
    # The package-manager branch missing on this release fails by design; the uci part must apply.
    assert router.uci_get("https-dns-proxy.dns.resolver_url") == endpoint, f"guide did not apply\n{out}"
    return out


@pytest.mark.integration
class TestOpenWrtGuide:
    """OpenWrt dnsmasq -> https-dns-proxy (guide config) -> modDNS DoH."""

    @pytest.mark.asyncio
    async def test_router_resolves_via_moddns(self, user, router):
        """specRef: Q5, Q15 - after the guide, LAN queries resolve through the profile."""
        pid = user.new_profile(f"{router.name}-resolve")
        await user.wait_for(pid, RESOLVABLE_TEST_DOMAIN, A, is_resolved)

        out = apply_guide(router, pid)
        resp = router.wait_until(RESOLVABLE_TEST_DOMAIN, is_resolved)
        assert is_resolved(resp), (
            f"{router.name} did not resolve {RESOLVABLE_TEST_DOMAIN} after the guide "
            f"(rcode={resp.rcode()})\n--- guide output ---\n{out}\n--- log ---\n{router.log()}"
        )

    @pytest.mark.asyncio
    async def test_profile_block_applies_on_lan(self, user, router):
        """specRef: Q15 - the profile in the guide's DoH URL is the one filtered."""
        pid = user.new_profile(f"{router.name}-block")
        # Not a special-use name: OpenWrt's dnsmasq answers those itself (RFC 6761 §6).
        domain = f"{router.name}-block-e2e.com"
        user.add_rule(pid, "block", domain)
        # Barrier (positive condition): block visible on the replica first.
        await user.wait_for(pid, domain, A, is_blocked)

        out = apply_guide(router, pid)
        resp = router.wait_until(domain, blocked_behind_router)
        assert blocked_behind_router(resp), (
            f"{domain} was not blocked via {router.name} (rcode={resp.rcode()}, answer={resp.answer})"
            f"\n--- guide output ---\n{out}\n--- log ---\n{router.log()}"
        )
