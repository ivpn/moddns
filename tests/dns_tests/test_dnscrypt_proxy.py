"""E2E: a real `dnscrypt-proxy` client resolving through modDNS over a DoH stamp.

modDNS DNSCrypt support is currently delivered as per-profile DoH stamps consumed by the
`dnscrypt-proxy` client (docs/features/dnscrypt/). `test_dns_stamps.py` proves
per-profile DoH via dnspython; this module guards the remaining link — that the
actual `dnscrypt-proxy` binary parses a modDNS-emitted DoH stamp and gets correct
per-profile behavior, with profile identity carried in the DoH URL path.

The client runs as a host subprocess (see libs/dnscrypt_proxy.py). It is skipped
when the binary is unavailable (offline, non-linux/x86_64 without an override).
"""
from __future__ import annotations

from contextlib import contextmanager

import dnsstamps
import pytest
from dns import exception as dns_exception
from dns.rdatatype import A
from dnsstamps import Option

import moddns.api as api
import moddns.api_client as client
import moddns.configuration as api_config
from moddns import RequestsDNSStampReq

from libs.constants import RESOLVABLE_TEST_DOMAIN
from libs.dns_lib import _dev_ca_path, assert_blocked, is_blocked, is_resolved
from libs.dnscrypt_proxy import DnscryptProxyClient, resolve_binary
from libs.session import ProfileSession


@contextmanager
def _stamps_api(user: ProfileSession):
    """Cookie-authenticated DNSStampsApi (mirrors test_dns_stamps.py)."""
    api_conf = api_config.Configuration(host=user.config.DNS_API_ADDR)
    with client.ApiClient(api_conf) as api_client:
        api_client.default_headers["Cookie"] = user.cookie
        yield api.DNSStampsApi(api_client)


def _fetch_doh_stamp(user: ProfileSession, profile_id: str, device_id: str = "") -> str:
    """The exact per-profile DoH `sdns://` string the product hands to users."""
    with _stamps_api(user) as stamps_api:
        body = RequestsDNSStampReq(profile_id=profile_id, device_id=device_id)
        return stamps_api.api_v1_dnsstamp_post(body=body).doh


@pytest.fixture(scope="session")
def dnscrypt_bin() -> str:
    """Resolve the dnscrypt-proxy binary once; skips the module if unavailable."""
    return resolve_binary()


@pytest.fixture
def dcp(dnscrypt_bin):
    """Factory: start a dnscrypt-proxy bound to a stamp; all instances torn down."""
    ca = _dev_ca_path()
    clients: list[DnscryptProxyClient] = []

    def _make(stamp: str, expect_ready: bool = True) -> DnscryptProxyClient:
        c = DnscryptProxyClient(stamp, ca_path=ca, binary=dnscrypt_bin)
        c.start(expect_ready=expect_ready)
        clients.append(c)
        return c

    yield _make
    for c in clients:
        c.stop()


@pytest.mark.integration
class TestDnscryptProxyOverDoH:
    """Real dnscrypt-proxy client ↔ modDNS via a per-profile DoH stamp."""

    BOGUS_PROFILE = "zzzznobody9"  # well-formed (alnum, len≥10) but nonexistent

    @pytest.mark.asyncio
    async def test_resolves_via_dnscrypt_proxy(self, user, dcp):
        """The real client parses the API's DoH stamp and resolves per-profile."""
        pid = user.new_profile("dcp-resolve")
        # Barrier: profile live on the proxy's replica before the client queries.
        await user.wait_for(pid, RESOLVABLE_TEST_DOMAIN, A, is_resolved)

        c = dcp(_fetch_doh_stamp(user, pid))
        resp = c.query(RESOLVABLE_TEST_DOMAIN)
        assert is_resolved(resp), (
            f"dnscrypt-proxy did not resolve {RESOLVABLE_TEST_DOMAIN} via the modDNS "
            f"DoH stamp (rcode={resp.rcode()})"
        )

    @pytest.mark.asyncio
    async def test_per_profile_block_applies(self, user, dcp):
        """A block rule on the profile applies to queries via dnscrypt-proxy."""
        pid = user.new_profile("dcp-block")
        domain = "dcp-proxy-block.test"
        user.add_rule(pid, "block", domain)
        # Barrier (positive condition): block visible on the replica first.
        await user.wait_for(pid, domain, A, is_blocked)

        c = dcp(_fetch_doh_stamp(user, pid))
        resp = c.query(domain)
        assert_blocked(resp, f"{domain} via dnscrypt-proxy (per-profile block not applied?)")

    @pytest.mark.asyncio
    async def test_unknown_profile_is_dropped(self, user, dcp):
        """An unknown profile in the DoH path is dropped — proving the path is
        enforced, not ignored. Control for the resolve test above."""
        bogus_stamp = dnsstamps.create_doh(
            "127.0.0.1:443", [], "moddns.dev",
            f"/dns-query/{self.BOGUS_PROFILE}",
            options=[Option.DNSSEC, Option.NO_LOGS],
        )
        c = dcp(bogus_stamp, expect_ready=False)
        try:
            resp = c.query(RESOLVABLE_TEST_DOMAIN, timeout=6)
        except dns_exception.Timeout:
            return  # no response → dropped, as expected
        assert not is_resolved(resp), (
            f"unknown profile unexpectedly resolved (rcode={resp.rcode()}); "
            "modDNS must drop unknown-profile queries"
        )
