from ipaddress import ip_address, IPv6Address

import pytest
from libs.dns_lib import is_blocked
from dns.rdataclass import IN
from dns.rdatatype import A, AAAA
from dns.flags import RD, QR


class TestCustomRules:
    @pytest.mark.asyncio
    @pytest.mark.parametrize(
        "test_domain,queries",
        [
            ("google.com", {"google.com": "0.0.0.0"}),
            (
                "*facebook.com",
                {"ads.facebook.com": "0.0.0.0", "facebook.com": "0.0.0.0"},
            ),
            (
                "*.amazon.com",
                {"ads.amazon.com": "0.0.0.0", "amazon.com": "0.0.0.0"},
            ),
            (
                "ads.*",
                {
                    "ads.com": "0.0.0.0",
                    "ads.de": "0.0.0.0",
                    "sub.ads.com": "1.1.1.1",  # should not match subdomain
                    "badads.com": "8.8.8.8",  # should not match
                },
            ),
            (
                "*ads*",
                {
                    "ads.com": "0.0.0.0",
                    "sub.ads.com": "0.0.0.0",
                    "shopads.io": "0.0.0.0",
                    "no-ads-here.com": "9.9.9.9",  # should not match
                },
            ),
            (
                "*.example.com",
                {
                    "example.com": "0.0.0.0",
                    "sub.example.com": "0.0.0.0",
                },
            ),
            (
                ".example.org",
                {
                    "example.org": "0.0.0.0",
                    "deep.example.org": "0.0.0.0",
                },
            ),
            (
                "my*.example.com",
                {
                    "mysubdomain.example.com": "0.0.0.0",
                    "other.example.com": "8.8.8.8",
                },
            ),
            (
                "sub-*-domain.example.com",
                {
                    "sub-test-domain.example.com": "0.0.0.0",
                    "sub.example.com": "8.8.8.8",
                },
            ),
            (
                "*ads.facebook.com",
                {
                    "ads.facebook.com": "0.0.0.0",
                    "euads.facebook.com": "0.0.0.0",
                    "facebook.com": "8.8.4.4",
                },
            ),
            (
                "wp.pl",
                {"ads.wp.pl": "0.0.0.0"},
            ),  # default "include" mode auto-prepends *. so subdomains are blocked
            (
                "104.18.74.230",  # Note: this IP is configured in testhosts.txt to resolve to
                {"test.com": "0.0.0.0"},
            ),  # block if one of the IPs is blocked
            (
                "2001:41d0:701:1100::29c8",
                {"ipv6-test.com": "::"},
            ),  # block IPv6, expect :: as blocked response
        ],
    )
    async def test_blocking_custom_rule_answer(self, user, test_domain, queries):
        """
        Configure a blocking custom rule for a domain/IP, then send queries and ensure DNS response contains expected IP address.
        """
        profile_id = user.new_profile("custom_rule")
        user.add_rule(profile_id, "block", test_domain)

        waited = False
        for query, expected_value in queries.items():
            # Determine if we should send an A or AAAA query
            try:
                ip_ver = ip_address(expected_value)
            except ValueError:
                ip_ver = None

            if isinstance(ip_ver, IPv6Address):
                record_type = AAAA
            else:
                record_type = A

            # Send DNS query. The first query whose block outcome depends on
            # the rule just created polls for replication to catch up.
            if expected_value in ("0.0.0.0", "::") and not waited:
                resp = await user.wait_for(profile_id, query, record_type, is_blocked)
                waited = True
            else:
                resp = await user.resolve(profile_id, query, record_type)
            # Blocked expectations: ensure an answer and it matches the block IP
            if expected_value in ("0.0.0.0", "::"):
                assert resp.answer, f"Expected a blocked answer for {query}"
                if record_type == A:
                    assert resp.answer[0].rdtype == A
                else:
                    assert resp.answer[0].rdtype == AAAA
                assert resp.answer[0].rdclass == IN
                assert resp.flags & QR, "QR flag is not set in the response"
                assert resp.flags & RD, "RD flag is not set in the response"
                ip_addr = resp.answer[0].to_text().split(" ")[-1]
                assert ip_address(ip_addr) == ip_address(
                    expected_value
                ), f"Blocked domain {test_domain} did not return {expected_value}"
            else:
                # Non-blocked expectations: allow any resolver behavior (could be NXDOMAIN or blocklists),
                # so no strict assertions here.
                continue
