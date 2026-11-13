"""Wire-level request-admission behaviour (proxy-request-admission-behaviour.md).

These tests pin the client-visible admission contract across vendor (dnsproxy)
upgrades. The code answering these queries has already moved between our
handler and the vendor once (dnsproxy v0.83 took over the FORMERR answer our
handler produced in v0.78), so unit tests against our handler alone cannot
guard the end-to-end response.
"""

import httpx
import pytest
from dns import message
from dns.rcode import FORMERR, NOTIMP
from libs.dns_lib import DNSLib
from libs.settings import get_settings

# Admission responses are produced before profile handling, so a bogus,
# unregistered profile ID is sufficient — and proves the ordering.
_PROFILE_ID = "123"

_dns = DNSLib(get_settings().DOH_ENDPOINT)


class TestRequestAdmission:
    @pytest.mark.asyncio
    async def test_any_query_answers_notimp(self):
        """Qtype ANY is answered with NOTIMP on the DoH path.

        Behaviour table #Q2 (proxy-request-admission-behaviour.md).
        """
        resp = await _dns.send_doh_request(_PROFILE_ID, "example.com", "ANY")
        assert resp.rcode() == NOTIMP, f"expected NOTIMP, got {resp.rcode()!r}"

    def test_empty_question_answers_formerr(self):
        """A message whose header declares QDCOUNT=1 but carries no question
        section is answered with FORMERR.

        QDCOUNT (RFC 1035 §4.1.1) is not validated against the body, so this
        12-byte header is a well-formed DNS message with a nil question list —
        dnspython cannot build it, hence the raw POST.

        Behaviour table #Q1 (proxy-request-admission-behaviour.md).
        """
        wire = bytes(
            [
                0x00, 0x2A,  # ID
                0x00, 0x00,  # flags: QR=0, opcode QUERY
                0x00, 0x01,  # QDCOUNT = 1 (lies — there is no question)
                0x00, 0x00,  # ANCOUNT
                0x00, 0x00,  # NSCOUNT
                0x00, 0x00,  # ARCOUNT
            ]
        )

        r = httpx.post(
            f"{get_settings().DOH_ENDPOINT}{_PROFILE_ID}",
            content=wire,
            headers={"Content-Type": "application/dns-message"},
            timeout=10,
        )
        assert r.status_code == 200, f"expected DNS answer, got HTTP {r.status_code}"

        resp = message.from_wire(r.content)
        assert resp.rcode() == FORMERR, f"expected FORMERR, got {resp.rcode()!r}"
        assert resp.id == 0x002A, "response ID must match the request"

    @pytest.mark.parametrize(
        "profile_path",
        ["", "abc", "abcde-fghijk", "zzzzzzzzzzzz"],
        ids=["no-profile", "too-short", "non-alphanumeric", "unknown-profile"],
    )
    def test_doh_without_usable_profile_answers_http_400(self, profile_path):
        """A DoH request whose path carries no usable or no existing profile ID
        gets HTTP 400 with no DNS message, not an empty 200: a 2xx is reserved
        for a valid DNS response (RFC 8484 §4.2.1). Missing, malformed and
        unknown IDs get the same status.

        Behaviour table #Q5, #Q6, #Q11 (proxy-request-admission-behaviour.md).
        """
        base = get_settings().DOH_ENDPOINT.rstrip("/")
        url = f"{base}/{profile_path}" if profile_path else base
        wire = message.make_query("example.com", "A").to_wire()

        r = httpx.post(
            url,
            content=wire,
            headers={"Content-Type": "application/dns-message"},
            timeout=10,
        )
        assert r.status_code == 400, f"expected HTTP 400, got HTTP {r.status_code}"
        assert r.headers.get("content-type", "") != "application/dns-message"
