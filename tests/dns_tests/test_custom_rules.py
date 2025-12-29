from ipaddress import ip_address, IPv6Address

import pytest
from libs.dns_lib import DNSLib
from libs.settings import get_settings
from dns.rdataclass import IN
from dns.rdatatype import A, AAAA
from dns.flags import RD, QR

import moddns.api_client as client
import moddns.api as api
import moddns.configuration as api_config
from moddns import RequestsCreateProfileCustomRuleBody


class TestCustomRules:
    def setup_class(self):
        """Setup the test class."""
        self.config = get_settings()
        self.api_config = api_config.Configuration(host=self.config.DNS_API_ADDR)
        self.dns_lib = DNSLib(self.config.DOH_ENDPOINT)

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
                {"ads.amazon.com": "0.0.0.0", "amazon.com": "54.239.28.8"},
            ),
            (
                "wp.pl",
                {"ads.wp.pl": "212.77.99.7"},
            ),  # do not block subdomains in custom rules by default (wildcard can be used for that case)
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
    async def test_blocking_custom_rule_answer(
        self, create_account_and_login, test_domain, queries
    ):
        """
        Create account, configure blocking custom rule for a domain/IP, then send queries and ensure DNS response contains expected IP address.
        """
        account, cookie = create_account_and_login
        with client.ApiClient(self.api_config) as api_client:
            profiles_instance = api.ProfileApi(api_client)

            profile_id = account.profiles[0]
            custom_rule_body = RequestsCreateProfileCustomRuleBody(
                action="block", value=test_domain
            )
            profiles_instance.api_client.default_headers["Cookie"] = cookie
            ur_resp = (
                profiles_instance.api_v1_profiles_id_custom_rules_post_with_http_info(
                    id=profile_id, body=custom_rule_body
                )
            )
            assert (
                ur_resp.status_code == 201
            ), f"Custom rule creation failed for {test_domain} with status code: {ur_resp.status_code}"

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

                # Send DNS query
                resp = await self.dns_lib.send_doh_request(
                    profile_id, query, record_type
                )

                assert len(resp.answer) == 1, f"Unexpected answer count for {query}"
                if record_type == A:
                    assert resp.answer[0].rdtype == A
                else:
                    assert resp.answer[0].rdtype == AAAA
                assert resp.answer[0].rdclass == IN
                assert resp.flags & QR, "QR flag is not set in the response"
                assert resp.flags & RD, "RD flag is not set in the response"
                ip_addr = resp.answer[0].to_text().split(" ")[-1]
                if expected_value in ("0.0.0.0", "::"):
                    assert ip_address(ip_addr) == ip_address(
                        expected_value
                    ), f"Blocked domain {test_domain} did not return {expected_value}"
                else:
                    # just validate the answer is a proper IP address since we query real DNS servers and IP addresses may change
                    assert ip_address(
                        ip_addr
                    ), f"Blocked domain {test_domain} returned unexpected IP {ip_addr} instead of proper IP address"
