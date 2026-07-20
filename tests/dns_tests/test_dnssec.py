from ipaddress import ip_address

import pytest
from libs.dns_lib import DNSLib, is_resolved
from libs.settings import get_settings
from dns.rdataclass import IN
from dns.rdatatype import A, RRSIG
from dns.flags import AD, CD, DO
from dns.rcode import NOERROR, SERVFAIL

from conftest import create_acc_and_login_func
import moddns.api_client as client
import moddns.api as api
import moddns.configuration as api_config
from moddns import RequestsProfileUpdates, ModelProfileUpdate


class TestDNSSEC:
    def setup_class(self):
        """Setup the test class."""
        self.config = get_settings()
        self.api_config = api_config.Configuration(host=self.config.DNS_API_ADDR)
        self.dns_lib = DNSLib(self.config.DOH_ENDPOINT)

    @pytest.mark.asyncio
    async def test_valid_dnssec_answer(self, create_account_and_login):
        """
        Create account, then:
        1. Send query to properly DNSSEC-configured domain and make sure the DNS response does not contain DNSSEC validation results (DO bit is not send, therefore end device won't get RRSIG query entries).
        2. Enable DO bit sending, then send query to properly DNSSEC-configured domain and make sure the DNS response does contain DNSSEC validation results (DO bit is sent, therefore end device will get RRSIG query entries).
        """
        account, cookie = create_account_and_login
        profile_id = account.profiles[0]

        with client.ApiClient(self.api_config) as api_client:
            profiles_instance = api.ProfileApi(api_client)
            profiles_instance.api_client.default_headers["Cookie"] = cookie
            profile = profiles_instance.api_v1_profiles_id_get(profile_id)
            assert (
                profile.settings.security.dnssec.enabled
            ), "DNSSEC validation should be enabled by default for new profiles"
            # Make sure DO bit is disabled by default for new profiles
            assert (
                not profile.settings.security.dnssec.send_do_bit
            ), "DO bit is enabled by default for new profiles but should be disabled"

        resp = await self.dns_lib.wait_until(profile_id, "example.com", "A", is_resolved)
        assert (
            len(resp.answer) == 1
        )  # 1 answers since DNSSEC is configured on example.com
        assert resp.rcode() == NOERROR
        assert resp.answer[0].rdtype == A
        assert resp.answer[0].rdclass == IN
        ipv4_addr = resp.answer[0].to_text().split(" ")[-1]
        assert ip_address(ipv4_addr) != ip_address("0.0.0.0")

        with client.ApiClient(self.api_config) as api_client:
            profiles_instance = api.ProfileApi(api_client)

            # Create request body to disable DNSSEC
            update_request = RequestsProfileUpdates(
                updates=[
                    ModelProfileUpdate(
                        operation="replace",
                        path="/settings/security/dnssec/send_do_bit",
                        value={
                            "value": True
                        },  # Dict[string, Any] is a openapi-cli-gen limitation - 'interface{}' Go type is transformed to Dict[string, Any] in the generated code
                    )
                ]
            )
            profiles_instance.api_client.default_headers["Cookie"] = cookie
            resp = profiles_instance.api_v1_profiles_id_patch_with_http_info(
                account.profiles[0], body=update_request
            )
            assert (
                resp.status_code == 200
            ), f"Profile DNSSEC settings update failed with status code: {resp.status_code} and payload {resp.data}"

        resp = await self.dns_lib.wait_until(
            profile_id, "example.com", "A", lambda r: len(r.answer) == 2
        )
        assert (
            len(resp.answer) == 2
        )  # 2 answers since DNSSEC is configured on example.com
        assert resp.rcode() == NOERROR
        assert resp.answer[0].rdtype == A
        assert resp.answer[0].rdclass == IN
        assert resp.answer[1].rdtype == RRSIG
        assert resp.answer[1].rdclass == IN
        assert resp.flags & AD, "AD flag is not set in the response"
        assert not (
            resp.flags & CD
        ), "CD (Checking Disabled) flag is set in the response but should not be"
        ipv4_addr = resp.answer[0].to_text().split(" ")[-1]
        assert ip_address(ipv4_addr) != ip_address("0.0.0.0")

    @pytest.mark.asyncio
    async def test_invalid_dnssec_answer(self, create_account_and_login):
        """
        Create account, send query to improperly DNSSEC-configured domain and make sure the DNS response contains DNSSEC validation results.
        """
        account, _ = create_account_and_login
        assert len(account.profiles) == 1

        profile_id = account.profiles[0]
        resp = await self.dns_lib.wait_until(
            profile_id, "dnssec-failed.org", "A", lambda r: r.rcode() == SERVFAIL
        )
        assert (
            len(resp.answer) == 0
        )  # No answers since DNSSEC check failed on dnssec-failed.org
        assert resp.rcode() == SERVFAIL
        assert (
            resp.flags & DO
        ), "DO flag is not set in repsonse flags"  # DO flag is set in the response

    @pytest.mark.asyncio
    @pytest.mark.parametrize(
        "test_domain,expected_results",
        [
            (
                "example.com",
                {"rdtype": A, "rdclass": IN, "rcode": NOERROR, "resp_length": 1},
            ),
            (
                "dnssec-failed.org",
                {"rdtype": A, "rdclass": IN, "rcode": NOERROR, "resp_length": 1},
            ),
        ],
    )
    async def test_answer_no_dnssec(self, test_domain, expected_results):
        """
        Create account, disable DNSSEC validation, send query to DNSSEC-configured domain and make sure the DNS response does not contain DNSSEC validation results (DO bit is not sent).
        """
        account, cookie, _ = create_acc_and_login_func()
        profile_id = account.profiles[0]

        with client.ApiClient(self.api_config) as api_client:
            profiles_instance = api.ProfileApi(api_client)
            profiles_instance.api_client.default_headers["Cookie"] = cookie
            profile = profiles_instance.api_v1_profiles_id_get(profile_id)
            assert (
                profile.settings.security.dnssec.enabled
            ), "DNSSEC validation should be enabled by default for new profiles"
            # Make sure DO bit is disabled by default for new profiles
            assert (
                not profile.settings.security.dnssec.send_do_bit
            ), "DO bit is enabled by default for new profiles but should be disabled"

        with client.ApiClient(self.api_config) as api_client:
            profiles_instance = api.ProfileApi(api_client)

            # Create request body to disable DNSSEC
            update_request = RequestsProfileUpdates(
                updates=[
                    ModelProfileUpdate(
                        operation="replace",
                        path="/settings/security/dnssec/enabled",
                        value={
                            "value": False
                        },  # Dict[string, Any] is a openapi-cli-gen limitation - 'interface{}' Go type is transformed to Dict[string, Any] in the generated code
                    )
                ]
            )
            profiles_instance.api_client.default_headers["Cookie"] = cookie
            resp = profiles_instance.api_v1_profiles_id_patch_with_http_info(
                profile_id, body=update_request
            )
            assert (
                resp.status_code == 200
            ), f"Profile DNSSEC settings update failed with status code: {resp.status_code} and payload {resp.data}"
            resp = await self.dns_lib.wait_until(
                profile_id, test_domain, "A", lambda r: r.flags & CD
            )
            assert len(resp.answer) == expected_results["resp_length"]
            assert resp.rcode() == expected_results["rcode"]
            assert resp.answer[0].rdtype == expected_results["rdtype"]
            assert resp.answer[0].rdclass == expected_results["rdclass"]

            assert (
                resp.flags & CD
            ), "CD (Checking Disabled) flag is not set in the response"
            assert not (
                resp.flags & AD
            ), "AD (Authenticated Data) flag is set in the response but should not be"
            ipv4_addr = resp.answer[0].to_text().split(" ")[-1]
            assert ip_address(ipv4_addr) != ip_address("0.0.0.0")
