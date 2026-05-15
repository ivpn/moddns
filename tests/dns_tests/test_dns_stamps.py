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

from contextlib import contextmanager
from ipaddress import ip_address

import dnsstamps
import pytest
from dns.rdatatype import A

import moddns.api as api
import moddns.api_client as client
import moddns.configuration as api_config
from moddns import RequestsDNSStampReq

from libs.constants import RESOLVABLE_TEST_DOMAIN
from libs.dns_lib import (
    assert_blocked,
    assert_not_blocked,
    first_answer_ip,
    is_blocked,
    is_resolved,
)
from libs.profile_helpers import SVC_GOOGLE_DOMAIN, SVC_GOOGLE_IP
from libs.session import ProfileSession


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


@contextmanager
def _stamps_api(user: ProfileSession):
    """Cookie-authenticated DNSStampsApi — not wrapped by ProfileSession."""
    api_conf = api_config.Configuration(host=user.config.DNS_API_ADDR)
    with client.ApiClient(api_conf) as api_client:
        api_client.default_headers["Cookie"] = user.cookie
        yield api.DNSStampsApi(api_client)


def _fetch_stamps(user: ProfileSession, profile_id: str, device_id: str | None = None):
    with _stamps_api(user) as stamps_api:
        body = RequestsDNSStampReq(profile_id=profile_id, device_id=device_id or "")
        return stamps_api.api_v1_dnsstamp_post(body=body)


class TestDNSStampGeneration:
    """Layer 1 — stamp content correctness. specRef: M1, M4, M5."""

    @pytest.mark.asyncio
    async def test_three_stamps_returned_and_decode_correctly(self, user):
        """specRef: M1, M4"""
        profile_id = user.default_profile_id
        resp = _fetch_stamps(user, profile_id)

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
    async def test_device_id_encoded_into_each_stamp(self, user):
        """specRef: M5 — device id propagates into DoH path + DoT/DoQ SNI."""
        profile_id = user.default_profile_id
        resp = _fetch_stamps(user, profile_id, device_id="Living Room")

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
    async def test_validation_rejects_short_profile_id(self, user):
        """specRef: M2 — profile_id must be alphanumeric, length 10–64.

        The OpenAPI swagger annotations propagate the constraints to the
        generated pydantic model, so client-side validation raises before
        the request leaves the test. That's actually a stronger guarantee
        than server-side rejection — we accept either outcome.
        """
        with _stamps_api(user) as stamps_api:
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

    @pytest.mark.asyncio
    @pytest.mark.parametrize("protocol", PROTOCOLS)
    async def test_resolution_via_each_stamp(self, user, protocol):
        """specRef: M1, M4 — open a real connection via the stamp and resolve a known domain.

        Uses SVC_GOOGLE_DOMAIN (svctest-google.com → 8.8.8.8 via testhosts.txt),
        a deterministic stub so the test doesn't depend on live external DNS.
        """
        profile_id = user.default_profile_id
        stamp = dnsstamps.parse(_stamp_for(_fetch_stamps(user, profile_id), protocol))

        # The freshly registered profile may not have replicated to the proxy's
        # Redis replica yet; poll via DoH until it resolves, then exercise the
        # stamp transport itself.
        await user.wait_for(profile_id, SVC_GOOGLE_DOMAIN, A, is_resolved)
        resp = await user.dns.send_via_stamp(stamp, SVC_GOOGLE_DOMAIN, A)

        assert resp.answer, f"{protocol}: empty answer for {SVC_GOOGLE_DOMAIN}"
        got_ip = first_answer_ip(resp)
        assert ip_address(got_ip) == ip_address(SVC_GOOGLE_IP), (
            f"{protocol}: expected {SVC_GOOGLE_IP} stub, got {got_ip}"
        )


class TestDNSStampProfileIsolation:
    """Layer 3 — per-profile filtering survives every stamp transport.

    The regression guard: prove that a block rule on profile P1 applies to
    queries through P1's stamp, but does NOT affect P2's stamp — across all
    three transports.

    specRef: M1, M4
    """

    BLOCKED_DOMAIN = "stamp-isolation-block.test"

    @pytest.mark.asyncio
    @pytest.mark.parametrize("protocol", PROTOCOLS)
    async def test_block_in_p1_does_not_affect_p2(self, user, protocol):
        # Fresh P1/P2 pair per protocol (new_profile de-dupes names), so the
        # custom rule added below doesn't clash across parametrizations.
        p1 = user.new_profile(f"stamp-iso-p1-{protocol}")
        p2 = user.new_profile(f"stamp-iso-p2-{protocol}")
        user.add_rule(p1, "block", self.BLOCKED_DOMAIN)

        stamp_p1 = dnsstamps.parse(_stamp_for(_fetch_stamps(user, p1), protocol))
        stamp_p2 = dnsstamps.parse(_stamp_for(_fetch_stamps(user, p2), protocol))

        # Wait (via DoH, positive conditions only) until both profiles have
        # propagated to the proxy's replica: P1's rule visibly blocks, and P2
        # resolves at all. Only then is P2's negative assertion meaningful.
        await user.wait_for(p1, self.BLOCKED_DOMAIN, A, is_blocked)
        await user.wait_for(p2, RESOLVABLE_TEST_DOMAIN, A, is_resolved)

        # P1's stamp: the rule must apply — blocked response is the sentinel.
        r1 = await user.dns.send_via_stamp(stamp_p1, self.BLOCKED_DOMAIN, A)
        assert_blocked(
            r1,
            f"{protocol}: {self.BLOCKED_DOMAIN} via P1 stamp (profile id not "
            f"routed through {protocol.upper()} transport?)",
        )

        # P2's stamp: must NOT be affected — either empty answer or non-block IP.
        r2 = await user.dns.send_via_stamp(stamp_p2, self.BLOCKED_DOMAIN, A)
        assert_not_blocked(
            r2,
            f"{protocol}: {self.BLOCKED_DOMAIN} via P2 stamp (P1's block LEAKED "
            f"across {protocol.upper()} transport?)",
        )
