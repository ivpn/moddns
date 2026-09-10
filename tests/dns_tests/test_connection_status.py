"""Backend E2E tests for the DNS connection-status check (dnscheck).

Flow under test, end to end through public interfaces only:

1. DoH query for ``<label>.<check domain>`` to the proxy, profile ID in the path.
2. The proxy forwards it to dnscheck with the profile ID in EDNS0 option 0xfeed
   (the hostname carries no profile ID).
3. dnscheck classifies the proxy's source address and records
   ``{status, profile_id}`` under the probe label.
4. HTTP GET with the probe hostname in ``Host`` returns that record once.

The proxy resolves the check domain through a network alias on the dnscheck
container (``tests/docker-compose.yml``), and the HTTP side is reached through
the published API port with a ``Host`` header, because the public check zone
resolves to production.

Spec: ``docs/specs/dnscheck-behaviour.md`` — rows are referenced per test.
"""

import asyncio
import random
import string

import pytest
import requests
from dns.rdatatype import A

from libs.settings import get_settings

# Must equal DNS_CHECK_DOMAIN in config/proxy.env and the dnscheck network alias.
CHECK_DOMAIN = "test.moddns.net"
LABEL_ALPHABET = string.ascii_letters + string.digits


def probe_label() -> str:
    """A fresh 12-char probe label, the shape the frontend generates."""
    return "".join(random.choice(LABEL_ALPHABET) for _ in range(12))


def check_http(label: str, origin: str | None = None) -> requests.Response:
    headers = {"Host": f"{label}.{CHECK_DOMAIN}"}
    if origin:
        headers["Origin"] = origin
    return requests.get(f"{get_settings().DNSCHECK_API_ADDR}/", headers=headers, timeout=10)


async def probe(user, profile_id: str, label: str) -> None:
    """Send the probe query through the proxy and assert it was answered."""
    resp = await user.wait_for(
        profile_id, f"{label}.{CHECK_DOMAIN}", A, lambda r: len(r.answer) > 0
    )
    assert len(resp.answer) > 0, f"probe query for {label} was not answered: {resp}"


@pytest.mark.integration
class TestDnsConnectionStatus:

    # specRef: dnscheck-behaviour.md #D5, #D7, #A3
    @pytest.mark.asyncio
    async def test_query_through_proxy_is_reported_as_configured(self, user):
        pid = user.default_profile_id
        label = probe_label()

        await probe(user, pid, label)

        resp = check_http(label, origin="http://localhost:5173")
        assert resp.status_code == 200, resp.text
        body = resp.json()
        assert body == {"status": "ok", "profile_id": pid}, body
        assert "Access-Control-Allow-Origin" in resp.headers

    # The profile comes from the proxy's EDNS0 option, never from the hostname:
    # querying with a different profile changes the answer, the label does not.
    #
    # specRef: dnscheck-behaviour.md #D7
    @pytest.mark.asyncio
    async def test_reports_the_profile_that_actually_queried(self, user):
        other = user.new_profile("connection-check-other")
        label = probe_label()

        await probe(user, other, label)

        body = check_http(label).json()
        assert body["status"] == "ok"
        assert body["profile_id"] == other
        assert body["profile_id"] != user.default_profile_id

    # specRef: dnscheck-behaviour.md #A2
    def test_no_prior_query_is_disconnected(self, user):
        resp = check_http(probe_label())
        assert resp.status_code == 404, resp.text
        assert resp.json() == {"error": "disconnected"}

    # specRef: dnscheck-behaviour.md #A4
    @pytest.mark.asyncio
    async def test_record_is_single_use(self, user):
        label = probe_label()
        await probe(user, user.default_profile_id, label)

        assert check_http(label).status_code == 200
        assert check_http(label).status_code == 404

    # specRef: dnscheck-behaviour.md #A1
    def test_malformed_label_is_rejected(self, user):
        for label in ("short", "abcdefghijklm", "abcdefghijkl-", "abcdefghij_l"):
            resp = check_http(label)
            assert resp.status_code == 400, f"{label}: {resp.status_code} {resp.text}"

    # Clients still on the previous frontend bundle append "-<profile id>".
    #
    # specRef: dnscheck-behaviour.md #D3, #A1
    @pytest.mark.asyncio
    async def test_legacy_suffixed_label_still_works(self, user):
        pid = user.default_profile_id
        label = f"{probe_label()}-{pid}"

        await probe(user, pid, label)

        resp = check_http(label)
        assert resp.status_code == 200, resp.text
        assert resp.json() == {"status": "ok", "profile_id": pid}

    # specRef: dnscheck-behaviour.md #D5
    @pytest.mark.asyncio
    async def test_concurrent_probes_are_kept_apart(self, user):
        pid = user.default_profile_id
        labels = [probe_label() for _ in range(3)]

        await asyncio.gather(*(probe(user, pid, label) for label in labels))

        for label in labels:
            resp = check_http(label)
            assert resp.status_code == 200, f"{label}: {resp.status_code} {resp.text}"
            assert resp.json() == {"status": "ok", "profile_id": pid}
