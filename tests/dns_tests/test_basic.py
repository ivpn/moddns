from ipaddress import ip_address

import pytest
from libs.dns_lib import DNSLib
from libs.session import ProfileSession
from libs.settings import get_settings
from dns.message import ShortHeader
from dns.rdataclass import IN
from dns.rdatatype import A

# Account-less DoH client for the missing/non-existent profile case, which must
# be exercised without a registered account.
_dns = DNSLib(get_settings().DOH_ENDPOINT)


class TestBasic:
    @pytest.mark.asyncio
    @pytest.mark.parametrize("profile_id", ["", "123"])
    async def test_profile_id_not_provided_or_non_existing(self, profile_id: str):
        """
        Verify that missing profile_id in the DNS DoH request raises an
        exception (connection is dropped, user does not get any response).
        """
        with pytest.raises(ShortHeader):
            await _dns.send_doh_request(profile_id, "example.com", "A")

    @pytest.mark.asyncio
    @pytest.mark.xfail(
        strict=False,
        reason="depends on live external DNS (facebook.com via real recursion)",
    )
    async def test_regular_account(self):
        """
        Create account and use its profile_id to resolve some DNS request.
        """
        session = ProfileSession.create()
        try:
            resp = await session.resolve(session.default_profile_id, "facebook.com", A)
            assert (
                len(resp.answer) == 1
            )  # 1 answer since DNSSEC is not configured on facebook.com
            assert resp.answer[0].rdtype == A
            assert resp.answer[0].rdclass == IN
            ipv4_addr = resp.answer[0].to_text().split(" ")[-1]
            assert ip_address(ipv4_addr) != ip_address("0.0.0.0")
        finally:
            session.cleanup()
