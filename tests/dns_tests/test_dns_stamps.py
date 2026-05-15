"""Integration tests for the DNS Stamp generator and end-to-end stamp-based resolution.

Three layers:

1. **Generation correctness** — call POST /api/v1/dnsstamp, decode each returned
   sdns:// with the Python dnsstamps library, assert encoded fields match what
   the proxy actually accepts. Covers spec rows M1, M4, M5.

2. **Resolution end-to-end** — actually connect via DoH/DoT/DoQ on the host/port
   the stamp encodes (substituting 127.0.0.1 for the public anycast IP but
   preserving SNI), send a DNS query, get a real answer.

3. **Per-profile filtering across all three transports** — block a domain for
   profile P1, generate stamps for both P1 and P2, verify P1's stamp returns
   the blocked response while P2's stamp doesn't. Proves profile identity
   travels correctly through DoH path, DoT SNI, and DoQ SNI.

Spec: docs/specs/api-endpoint-behaviour.md §M.
"""
from __future__ import annotations

from ipaddress import ip_address

import dnsstamps
import pytest
from dns.rdatatype import A

import moddns.api as api
import moddns.api_client as client
import moddns.configuration as api_config
from moddns import RequestsDNSStampReq

from libs.dns_lib import DNSLib
from libs.profile_helpers import (
    ProfileHelpers,
    SVC_GOOGLE_DOMAIN,
    SVC_GOOGLE_IP,
    extract_ip,
)
from libs.settings import get_settings


# Test domain set up via tests/config/api.env:
#   SERVER_DNS_DOMAIN=ivpndns.com
#   SERVER_DNS_SERVER_ADDRESSES=127.0.0.1
EXPECTED_DOMAIN = "ivpndns.com"
EXPECTED_IP = "127.0.0.1"
EXPECTED_DOT_PORT = 853
EXPECTED_DOQ_PORT = 853

PROTO_DOH = "doh"
PROTO_DOT = "dot"
PROTO_DOQ = "doq"
PROTOCOLS = [PROTO_DOH, PROTO_DOT, PROTO_DOQ]


def _stamp_for(resp, protocol: str) -> str:
    return {PROTO_DOH: resp.doh, PROTO_DOT: resp.dot, PROTO_DOQ: resp.doq}[protocol]


def _fetch_stamps(api_client, profile_id, device_id: str | None = None):
    """Wrapper that swallows OpenAPI client model boilerplate."""
    stamps_api = api.DNSStampsApi(api_client)
    body = RequestsDNSStampReq(profile_id=profile_id, device_id=device_id or "")
    return stamps_api.api_v1_dnsstamp_post(body=body)


class TestDNSStampGeneration:
    """Layer 1 — stamp content correctness. specRef: M1, M4, M5."""

    def setup_class(self):
        self.config = get_settings()
        self.api_config = api_config.Configuration(host=self.config.DNS_API_ADDR)

    @pytest.mark.asyncio
    async def test_three_stamps_returned_and_decode_correctly(self, create_account_and_login):
        """specRef: M1, M4"""
        account, cookie = create_account_and_login
        profile_id = account.profiles[0]

        with client.ApiClient(self.api_config) as api_client:
            api_client.default_headers["Cookie"] = cookie
            resp = _fetch_stamps(api_client, profile_id)

        # Each protocol field is a non-empty sdns:// string.
        assert resp.doh.startswith("sdns://"), f"DoH missing prefix: {resp.doh!r}"
        assert resp.dot.startswith("sdns://"), f"DoT missing prefix: {resp.dot!r}"
        assert resp.doq.startswith("sdns://"), f"DoQ missing prefix: {resp.doq!r}"

        doh = dnsstamps.parse(resp.doh)
        assert doh.protocol == dnsstamps.Protocol.DOH
        assert doh.hostname == EXPECTED_DOMAIN, f"DoH hostname={doh.hostname!r}"
        assert doh.path == f"/dns-query/{profile_id}", f"DoH path={doh.path!r}"
        # DoH address has port stripped (443 is library default); just the IP remains.
        assert doh.address == EXPECTED_IP, f"DoH address={doh.address!r}"

        dot = dnsstamps.parse(resp.dot)
        assert dot.protocol == dnsstamps.Protocol.DOT
        assert dot.hostname == f"{profile_id}.{EXPECTED_DOMAIN}", f"DoT hostname={dot.hostname!r}"
        assert dot.address == f"{EXPECTED_IP}:{EXPECTED_DOT_PORT}", (
            f"DoT must carry :{EXPECTED_DOT_PORT} explicitly, got {dot.address!r}"
        )

        doq = dnsstamps.parse(resp.doq)
        assert doq.protocol == dnsstamps.Protocol.DOQ
        assert doq.hostname == f"{profile_id}.{EXPECTED_DOMAIN}", f"DoQ hostname={doq.hostname!r}"
        assert doq.address == f"{EXPECTED_IP}:{EXPECTED_DOQ_PORT}", (
            f"DoQ must carry :{EXPECTED_DOQ_PORT} explicitly, got {doq.address!r}"
        )

        # Props bitmap — DNSSEC + NoLog set, NoFilter intentionally not set
        # (modDNS filters; advertising NoFilter would be misleading).
        for name, stamp in (("doh", doh), ("dot", dot), ("doq", doq)):
            assert dnsstamps.Option.DNSSEC in stamp.options, f"{name}: DNSSEC must be set"
            assert dnsstamps.Option.NO_LOGS in stamp.options, f"{name}: NO_LOGS must be set"
            assert dnsstamps.Option.NO_FILTERS not in stamp.options, (
                f"{name}: NO_FILTERS must NOT be set (modDNS filters)"
            )

    @pytest.mark.asyncio
    async def test_device_id_encoded_into_each_stamp(self, create_account_and_login):
        """specRef: M5 — device id propagates into DoH path + DoT/DoQ SNI."""
        account, cookie = create_account_and_login
        profile_id = account.profiles[0]

        with client.ApiClient(self.api_config) as api_client:
            api_client.default_headers["Cookie"] = cookie
            resp = _fetch_stamps(api_client, profile_id, device_id="Living Room")

        doh = dnsstamps.parse(resp.doh)
        assert doh.path == f"/dns-query/{profile_id}/Living%20Room", (
            f"DoH path must URL-encode device id, got {doh.path!r}"
        )

        dot = dnsstamps.parse(resp.dot)
        assert dot.hostname == f"Living--Room-{profile_id}.{EXPECTED_DOMAIN}", (
            f"DoT SNI must use <encoded-device>-<profile>.<domain>, got {dot.hostname!r}"
        )

        doq = dnsstamps.parse(resp.doq)
        assert doq.hostname == f"Living--Room-{profile_id}.{EXPECTED_DOMAIN}", (
            f"DoQ SNI must use <encoded-device>-<profile>.<domain>, got {doq.hostname!r}"
        )

    @pytest.mark.asyncio
    async def test_validation_rejects_short_profile_id(self, create_account_and_login):
        """specRef: M2 — profile_id must be alphanumeric, length 10–64.

        The OpenAPI swagger annotations propagate the constraints to the
        generated pydantic model, so client-side validation raises before
        the request leaves the test. That's actually a stronger guarantee
        than server-side rejection — we accept either outcome.
        """
        _, cookie = create_account_and_login

        with client.ApiClient(self.api_config) as api_client:
            api_client.default_headers["Cookie"] = cookie
            stamps_api = api.DNSStampsApi(api_client)
            with pytest.raises(Exception) as exc_info:
                stamps_api.api_v1_dnsstamp_post(
                    body=RequestsDNSStampReq(profile_id="abc")
                )
            # Acceptable outcomes:
            #   - pydantic ValidationError on the client (model has min_length=10)
            #   - BadRequestException / ApiException with 400 from the server
            err_name = exc_info.type.__name__
            assert err_name in {"ValidationError", "BadRequestException", "ApiException"} or \
                   "400" in str(exc_info.value), (
                f"Expected client validation or server 400, got {err_name}: {exc_info.value!r}"
            )


class TestDNSStampResolution:
    """Layer 2 — every stamp actually resolves end-to-end."""

    def setup_class(self):
        self.config = get_settings()
        self.api_config = api_config.Configuration(host=self.config.DNS_API_ADDR)
        self.dns_lib = DNSLib(self.config.DOH_ENDPOINT)

    @pytest.mark.asyncio
    @pytest.mark.parametrize("protocol", PROTOCOLS)
    async def test_resolution_via_each_stamp(
        self, create_account_and_login, protocol
    ):
        """specRef: M1, M4 — open a real connection via the stamp and resolve a known domain.

        Uses SVC_GOOGLE_DOMAIN (svctest-google.com → 8.8.8.8 via testhosts.txt),
        a deterministic stub so the test doesn't depend on live external DNS.
        """
        account, cookie = create_account_and_login
        profile_id = account.profiles[0]

        with client.ApiClient(self.api_config) as api_client:
            api_client.default_headers["Cookie"] = cookie
            stamp_str = _stamp_for(_fetch_stamps(api_client, profile_id), protocol)

        stamp = dnsstamps.parse(stamp_str)
        resp = await self.dns_lib.send_via_stamp(stamp, SVC_GOOGLE_DOMAIN, A)

        assert resp.answer, f"{protocol}: empty answer for {SVC_GOOGLE_DOMAIN}"
        got_ip = extract_ip(resp)
        assert ip_address(got_ip) == ip_address(SVC_GOOGLE_IP), (
            f"{protocol}: expected {SVC_GOOGLE_IP} stub, got {got_ip}"
        )


class TestDNSStampProfileIsolation(ProfileHelpers):
    """Layer 3 — per-profile filtering survives every stamp transport.

    The regression guard: prove that a block rule on profile P1 applies to
    queries through P1's stamp, but does NOT affect P2's stamp — across all
    three transports.

    specRef: M1, M4
    """

    BLOCKED_DOMAIN = "stamp-isolation-block.test"

    def setup_class(self):
        self.config = get_settings()
        self.api_config = api_config.Configuration(host=self.config.DNS_API_ADDR)
        self.dns_lib = DNSLib(self.config.DOH_ENDPOINT)

    @pytest.mark.asyncio
    @pytest.mark.parametrize("protocol", PROTOCOLS)
    async def test_block_in_p1_does_not_affect_p2(
        self, create_account_and_login, protocol
    ):
        # create_account_and_login is class-scoped, so all three parametrizations
        # share one account. Spin up a fresh P1/P2 pair per protocol so the custom
        # rule, added below, doesn't clash with the previous parametrization's
        # rule on the same profile.
        _, cookie = create_account_and_login

        with client.ApiClient(self.api_config) as api_client:
            api_client.default_headers["Cookie"] = cookie
            profiles_api = api.ProfileApi(api_client)

            p1 = self._create_profile(profiles_api, f"stamp-iso-p1-{protocol}")
            p2 = self._create_profile(profiles_api, f"stamp-iso-p2-{protocol}")
            self._create_custom_rule(profiles_api, p1, "block", self.BLOCKED_DOMAIN)

            s1 = _stamp_for(_fetch_stamps(api_client, p1), protocol)
            s2 = _stamp_for(_fetch_stamps(api_client, p2), protocol)

        stamp_p1 = dnsstamps.parse(s1)
        stamp_p2 = dnsstamps.parse(s2)

        # P1's stamp: the rule must apply — blocked response is 0.0.0.0.
        r1 = await self.dns_lib.send_via_stamp(stamp_p1, self.BLOCKED_DOMAIN, A)
        assert r1.answer, f"{protocol}: P1 stamp returned no answer"
        assert extract_ip(r1) == "0.0.0.0", (
            f"{protocol}: P1 stamp failed to apply block — profile id not "
            f"routed through {protocol.upper()} transport"
        )

        # P2's stamp: must NOT be affected — either empty answer or non-block IP.
        r2 = await self.dns_lib.send_via_stamp(stamp_p2, self.BLOCKED_DOMAIN, A)
        leaked = bool(r2.answer) and extract_ip(r2) == "0.0.0.0"
        assert not leaked, (
            f"{protocol}: P2 stamp received P1's block — profile id LEAKED "
            f"across {protocol.upper()} transport"
        )
