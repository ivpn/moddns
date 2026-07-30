from ipaddress import ip_address

import pytest
from libs.dns_lib import is_resolved
from libs.session import ProfileSession
from dns.rdataclass import IN
from dns.rdatatype import A, RRSIG
from dns.flags import AD, CD, DO
from dns.rcode import NOERROR, SERVFAIL


class TestDNSSEC:
    @pytest.mark.asyncio
    async def test_valid_dnssec_answer(self, user):
        """
        Create account, then:
        1. Send query to properly DNSSEC-configured domain and make sure the DNS response does not contain DNSSEC validation results (DO bit is not send, therefore end device won't get RRSIG query entries).
        2. Enable DO bit sending, then send query to properly DNSSEC-configured domain and make sure the DNS response does contain DNSSEC validation results (DO bit is sent, therefore end device will get RRSIG query entries).
        """
        profile_id = user.default_profile_id

        profile = user.get_profile(profile_id)
        assert (
            profile.settings.security.dnssec.enabled
        ), "DNSSEC validation should be enabled by default for new profiles"
        # Make sure DO bit is disabled by default for new profiles
        assert (
            not profile.settings.security.dnssec.send_do_bit
        ), "DO bit is enabled by default for new profiles but should be disabled"

        resp = await user.wait_for(profile_id, "example.com", "A", is_resolved)
        assert (
            len(resp.answer) == 1
        )  # 1 answers since DNSSEC is configured on example.com
        assert resp.rcode() == NOERROR
        assert resp.answer[0].rdtype == A
        assert resp.answer[0].rdclass == IN
        ipv4_addr = resp.answer[0].to_text().split(" ")[-1]
        assert ip_address(ipv4_addr) != ip_address("0.0.0.0")

        user.patch_setting(profile_id, "/settings/security/dnssec/send_do_bit", True)

        resp = await user.wait_for(
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
    async def test_invalid_dnssec_answer(self, user):
        """
        Create account, send query to improperly DNSSEC-configured domain and make sure the DNS response contains DNSSEC validation results.
        """
        assert len(user.account.profiles) == 1

        profile_id = user.default_profile_id
        resp = await user.wait_for(
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
        session = ProfileSession.create()
        try:
            profile_id = session.default_profile_id

            profile = session.get_profile(profile_id)
            assert (
                profile.settings.security.dnssec.enabled
            ), "DNSSEC validation should be enabled by default for new profiles"
            # Make sure DO bit is disabled by default for new profiles
            assert (
                not profile.settings.security.dnssec.send_do_bit
            ), "DO bit is enabled by default for new profiles but should be disabled"

            session.patch_setting(
                profile_id, "/settings/security/dnssec/enabled", False
            )

            resp = await session.wait_for(
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
        finally:
            session.cleanup()
