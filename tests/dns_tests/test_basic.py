from ipaddress import ip_address

import pytest
from libs.dns_lib import DNSLib
from libs.settings import get_settings
from dns.message import ShortHeader
from dns.rdataclass import IN
from dns.rdatatype import A
import random
import string

from helpers import generate_complex_password
import moddns.api_client as client
import moddns.api as api
import moddns.configuration as api_config
from moddns import RequestsLoginBody
from conftest import create_temp_subscription


class TestBasic:
    def setup_class(self):
        """Setup the test class."""
        self.config = get_settings()
        self.api_config = api_config.Configuration(host=self.config.DNS_API_ADDR)
        self.dns_lib = DNSLib(self.config.DOH_ENDPOINT)

    @pytest.mark.asyncio
    @pytest.mark.parametrize("profile_id", ["", "123"])
    async def test_profile_id_not_provided_or_non_existing(self, profile_id: str):
        """
        Verify that missing profile_id in the DNS DoH request raises an
        exception (connection is dropped, user does not get any response).
        """
        with pytest.raises(ShortHeader):
            await self.dns_lib.send_doh_request(profile_id, "example.com", "A")

    @pytest.mark.asyncio
    async def test_regular_account(self):
        """
        Create account and use its profile_id to resolve some DNS request.
        """
        with client.ApiClient(self.api_config) as api_client:
            api_instance = api.AccountApi(api_client)

            password = generate_complex_password()
            subscription_id, pa_cookie = create_temp_subscription()
            email = f"test{''.join(random.choice(string.digits) for i in range(5))}@ivpn.net"

            api_instance.api_client.default_headers["Cookie"] = pa_cookie
            reg_resp = api_instance.api_v1_accounts_post(
                body={"email": email, "password": password, "subid": subscription_id}
            )
            # Login to obtain cookie
            auth_api = api.AuthenticationApi(api_client)
            login_resp = auth_api.api_v1_login_post_with_http_info(
                body=RequestsLoginBody(email=email, password=password)
            )
            assert login_resp.status_code == 200
            cookie = login_resp.headers.get("Set-Cookie")
            assert cookie
            api_instance.api_client.default_headers["Cookie"] = cookie
            account = api_instance.api_v1_accounts_current_get()
            assert len(account.profiles) == 1
            profile_id = account.profiles[0]
            resp = await self.dns_lib.send_doh_request(profile_id, "facebook.com", "A")
            assert (
                len(resp.answer) == 1
            )  # 1 answer since DNSSEC is not configured on facebook.com
            assert resp.answer[0].rdtype == A
            assert resp.answer[0].rdclass == IN
            ipv4_addr = resp.answer[0].to_text().split(" ")[-1]
            assert ip_address(ipv4_addr) != ip_address("0.0.0.0")
