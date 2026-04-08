import asyncio
from ipaddress import ip_address
from collections import namedtuple
import random
import string
import pytest
from dns.rdataclass import IN
from dns.rdatatype import A

from libs.dns_lib import DNSLib
from libs.settings import get_settings
from helpers import generate_complex_password
from moddns import RequestsLoginBody
import moddns.api_client as client
import moddns.api as api
import moddns.configuration as api_config
from conftest import create_temp_subscription


DNSRequest = namedtuple("DNSRequest", ["domain", "ipv4_answers"])


class TestMultipleUsers:
    def setup_class(self):
        """Setup the test class."""
        self.config = get_settings()
        self.api_config = api_config.Configuration(host=self.config.DNS_API_ADDR)
        self.dns_lib = DNSLib(self.config.DOH_ENDPOINT)

    @pytest.mark.asyncio
    async def test_multiple_temporary_accounts_sending_doh_requests(self):
        """
        Create 4 temporary accounts to resolve some DNS requests asynchronously (make sure the answers are properly assigned to requests).
        """
        with client.ApiClient(self.api_config) as api_client:
            api_instance = api.AccountApi(api_client)

            # Create multiple accounts with subscription markers
            profiles: list[str] = []
            for idx in range(4):
                subscription_id, pa_cookie = create_temp_subscription()
                email = f"test{''.join(random.choice(string.digits) for i in range(5))}@ivpn.net"
                password = generate_complex_password()

                # Register account (201 expected, no account object returned)
                api_instance.api_client.default_headers["Cookie"] = pa_cookie
                api_instance.api_v1_accounts_post(
                    body={
                        "email": email,
                        "password": password,
                        "subid": subscription_id,
                    }
                )

                # Login to obtain session cookie
                auth_api = api.AuthenticationApi(api_client)
                login_resp = auth_api.api_v1_login_post_with_http_info(
                    body=RequestsLoginBody(email=email, password=password)
                )
                assert login_resp.status_code == 200
                cookie = login_resp.headers.get("Set-Cookie")
                assert cookie
                api_instance.api_client.default_headers["Cookie"] = cookie

                # Fetch current account to obtain profile ID
                account = api_instance.api_v1_accounts_current_get()
                assert len(account.profiles) == 1
                profiles.append(account.profiles[0])

            expected_results = {
                profiles[0]: DNSRequest("news.ycombinator.com", ["209.216.230.207"]),
                profiles[1]: DNSRequest("wp.pl", ["212.77.98.9"]),
                profiles[2]: DNSRequest(
                    "edition.cnn.com",
                    ["151.101.131.5", "151.101.195.5", "151.101.3.5", "151.101.67.5"],
                ),
                profiles[3]: DNSRequest(
                    "linkedin.com", ["13.107.42.14", "150.171.22.12"]
                ),
            }

            results = await asyncio.gather(
                *[
                    self.dns_lib.send_doh_request(profile_id, dns_request.domain, "A")
                    for profile_id, dns_request in expected_results.items()
                ]
            )

            for resp, (profile_id, dns_request) in zip(
                results, expected_results.items()
            ):
                assert len(resp.answer) == 1
                assert resp.answer[0].rdtype == A
                assert resp.answer[0].rdclass == IN
                ipv4_addr = resp.answer[0].to_text().split(" ")[-1]
                assert ip_address(ipv4_addr) != ip_address("0.0.0.0")
                assert ipv4_addr in dns_request.ipv4_answers
