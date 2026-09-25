"""E2E: a real AdGuard Home in front of modDNS over DoH, DoT and DoQ.

AdGuard Home is the resolver most self-hosting modDNS users put in front of the
service. It accepts every upstream form the product hands out — the DoH URL
`https://<domain>/dns-query/<profile>`, `tls://<profile>.<domain>`,
`quic://<profile>.<domain>:853`, and the sdns:// stamps from POST /api/v1/dnsstamp
(Setup → Routers → DNS Stamps) — and it resolves upstream *hostnames* through its
own bootstrap resolvers, never the OS resolver. That bootstrap step is where field
setups break: a firewall or VPN kill-switch that swallows the bootstrap query makes
AdGuard report the upstream as unreachable although the anycast address pings.
Stamps carry the address and need no bootstrap at all.

The container is `adguardhome` in docker-compose.yml, seeded from
config/adguardhome.yaml with knot as bootstrap; knot answers `moddns.dev` and
`<profile>.moddns.dev` with the proxy's compose address (config/knot.config.yaml),
standing in for the public zone. Tests reconfigure the upstreams per profile via
AdGuard's /control API and query it on its published port.
"""
from __future__ import annotations

from urllib.parse import urlparse

import pytest
from dns.rdatatype import A

from libs.adguard_home import AdGuardHome
from libs.constants import RESOLVABLE_TEST_DOMAIN
from libs.dns_lib import assert_blocked, is_blocked, is_resolved
from libs.session import ProfileSession
from libs.settings import get_settings
from libs.stamps import PROTOCOLS, fetch_stamps, with_address

# TEST-NET-1 (RFC 5737): a bootstrap resolver that never answers.
DEAD_BOOTSTRAP = ["192.0.2.1"]
BOGUS_PROFILE = "zzzznobody9"  # well-formed (alnum, len≥10) but nonexistent


def _domain() -> str:
    return urlparse(get_settings().DOH_ENDPOINT).hostname


def url_upstream(profile_id: str, transport: str) -> str:
    """The upstream a user types into AdGuard Home by hand (no stamp)."""
    domain = _domain()
    return {
        "doh": f"https://{domain}/dns-query/{profile_id}",
        "dot": f"tls://{profile_id}.{domain}",
        "quic": f"quic://{profile_id}.{domain}:853",
    }[transport]


def stamp_upstream(user: ProfileSession, profile_id: str, transport: str) -> str:
    """The product's stamp, re-addressed to the proxy's compose address (libs/stamps.py)."""
    stamp = fetch_stamps(user, profile_id)[{"quic": "doq"}.get(transport, transport)]
    return with_address(stamp, get_settings().PROXY_NETWORK_ADDR)


TRANSPORTS = ("doh", "dot", "quic")
FORMS = ("url", "stamp")


def upstream(user: ProfileSession, profile_id: str, transport: str, form: str) -> str:
    if form == "url":
        return url_upstream(profile_id, transport)
    return stamp_upstream(user, profile_id, transport)


@pytest.fixture(scope="module")
def agh() -> AdGuardHome:
    cfg = get_settings()
    client = AdGuardHome(cfg.ADGUARD_HOME_API_ADDR, cfg.ADGUARD_HOME_DNS_ADDR, [cfg.BOOTSTRAP_DNS_ADDR])
    client.wait_ready()
    return client


def _failures(result: dict[str, str]) -> dict[str, str]:
    return {u: v for u, v in result.items() if v != "OK"}


@pytest.mark.integration
class TestAdGuardHome:
    """Real AdGuard Home ↔ modDNS, every transport, both upstream forms."""

    @pytest.mark.asyncio
    async def test_upstream_check_passes_for_every_form(self, user, agh):
        """specRef: M1, M4, Q11 — the *Test upstreams* button accepts all six forms.

        Three URL forms (path / SNI carry the profile) and the three stamps."""
        pid = user.new_profile("agh-check")
        await user.wait_for(pid, RESOLVABLE_TEST_DOMAIN, A, is_resolved)

        upstreams = [upstream(user, pid, t, f) for f in FORMS for t in TRANSPORTS]
        failed = _failures(agh.test_upstreams(upstreams))
        assert not failed, f"AdGuard Home rejected modDNS upstreams: {failed}"

    @pytest.mark.asyncio
    @pytest.mark.parametrize("transport", TRANSPORTS)
    @pytest.mark.parametrize("form", FORMS)
    async def test_resolves(self, user, agh, transport, form):
        """specRef: M1, M4 — a query through AdGuard Home reaches modDNS and resolves."""
        pid = user.new_profile(f"agh-{form}-{transport}")
        await user.wait_for(pid, RESOLVABLE_TEST_DOMAIN, A, is_resolved)

        agh.set_upstreams([upstream(user, pid, transport, form)])
        resp = agh.query(RESOLVABLE_TEST_DOMAIN)
        assert is_resolved(resp), (
            f"AdGuard Home ({form}/{transport}) did not resolve {RESOLVABLE_TEST_DOMAIN} "
            f"via modDNS (rcode={resp.rcode()})"
        )

    @pytest.mark.asyncio
    @pytest.mark.parametrize("transport", TRANSPORTS)
    @pytest.mark.parametrize("form", FORMS)
    async def test_per_profile_block_applies(self, user, agh, transport, form):
        """specRef: M4, M5, Q11 — the profile carried in the path / SNI is the one filtered."""
        pid = user.new_profile(f"agh-block-{form}-{transport}")
        domain = f"agh-{form}-{transport}-block.test"
        user.add_rule(pid, "block", domain)
        # Barrier (positive condition): block visible on the replica first.
        await user.wait_for(pid, domain, A, is_blocked)

        agh.set_upstreams([upstream(user, pid, transport, form)])
        resp = agh.query(domain)
        assert_blocked(resp, f"{domain} via AdGuard Home ({form}/{transport})")

    @pytest.mark.asyncio
    async def test_stamps_need_no_bootstrap_resolver(self, user, agh):
        """specRef: M1 — with a bootstrap resolver that never answers, the stamps still
        pass the upstream check (the address travels inside the stamp) while the URL
        forms cannot even find the proxy. This is the field failure behind
        "AdGuard cannot resolve the DoH/DoQ address although the IP pings"."""
        pid = user.new_profile("agh-bootstrap")
        await user.wait_for(pid, RESOLVABLE_TEST_DOMAIN, A, is_resolved)

        # AdGuard Home keys its verdicts by its own normalised upstream name, so
        # each form is checked in a call of its own and judged by the verdicts alone.
        stamps = agh.test_upstreams([stamp_upstream(user, pid, t) for t in TRANSPORTS], bootstrap=DEAD_BOOTSTRAP)
        assert len(stamps) == len(TRANSPORTS) and not _failures(stamps), (
            f"stamps must not depend on bootstrap DNS: {stamps}"
        )
        urls = agh.test_upstreams([url_upstream(pid, t) for t in TRANSPORTS], bootstrap=DEAD_BOOTSTRAP)
        passed = {u: v for u, v in urls.items() if v == "OK"}
        assert len(urls) == len(TRANSPORTS) and not passed, (
            f"URL upstreams passed with a dead bootstrap, so the bootstrap path is not "
            f"being exercised: {passed}"
        )

    @pytest.mark.asyncio
    async def test_unknown_profile_fails_upstream_check(self, agh):
        """Control for the checks above: an unknown profile in the path / SNI is dropped
        by the proxy, so AdGuard Home must not report the upstream as working."""
        result = agh.test_upstreams([url_upstream(BOGUS_PROFILE, t) for t in TRANSPORTS])
        passed = {u: v for u, v in result.items() if v == "OK"}
        assert len(result) == len(TRANSPORTS) and not passed, (
            f"unknown profile passed the upstream check: {passed}"
        )
