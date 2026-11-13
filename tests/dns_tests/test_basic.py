from ipaddress import ip_address

import pytest
from libs.session import ProfileSession
from dns.rdataclass import IN
from dns.rdatatype import A


class TestBasic:
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
