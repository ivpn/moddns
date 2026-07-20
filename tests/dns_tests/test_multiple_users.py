import asyncio
from ipaddress import ip_address
from collections import namedtuple

import pytest
from dns.rdataclass import IN
from dns.rdatatype import A

from libs.session import ProfileSession


DNSRequest = namedtuple("DNSRequest", ["domain", "ipv4_answers"])


class TestMultipleUsers:
    @pytest.mark.asyncio
    @pytest.mark.xfail(
        strict=False,
        reason="asserts live external DNS A records (linkedin.com etc.) which rotate; "
        "per tests/CLAUDE.md live-DNS tests warn rather than hard-fail on upstream changes",
    )
    async def test_multiple_temporary_accounts_sending_doh_requests(self):
        """
        Create 4 temporary accounts to resolve some DNS requests asynchronously (make sure the answers are properly assigned to requests).
        """
        sessions = [ProfileSession.create() for _ in range(4)]
        try:
            requests = [
                (sessions[0], DNSRequest("news.ycombinator.com", ["209.216.230.207"])),
                (sessions[1], DNSRequest("wp.pl", ["212.77.98.9"])),
                (
                    sessions[2],
                    DNSRequest(
                        "edition.cnn.com",
                        [
                            "151.101.131.5",
                            "151.101.195.5",
                            "151.101.3.5",
                            "151.101.67.5",
                        ],
                    ),
                ),
                (
                    sessions[3],
                    DNSRequest(
                        "linkedin.com",
                        ["13.107.42.14", "150.171.22.12", "130.211.32.14"],
                    ),
                ),
            ]

            results = await asyncio.gather(
                *[
                    session.resolve(session.default_profile_id, dns_request.domain, A)
                    for session, dns_request in requests
                ]
            )

            for resp, (session, dns_request) in zip(results, requests):
                assert len(resp.answer) == 1
                assert resp.answer[0].rdtype == A
                assert resp.answer[0].rdclass == IN
                ipv4_addr = resp.answer[0].to_text().split(" ")[-1]
                assert ip_address(ipv4_addr) != ip_address("0.0.0.0")
                assert ipv4_addr in dns_request.ipv4_answers
        finally:
            for session in sessions:
                session.cleanup()
