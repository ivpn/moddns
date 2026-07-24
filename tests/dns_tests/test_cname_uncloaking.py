import pytest
from dns.rdatatype import A

from libs.dns_lib import assert_blocked, assert_not_blocked, is_blocked, is_resolved

# Fixture names served by the knot recursor (config/knot.config.yaml
# local-data.records — CNAMEs cannot be expressed in the sdns hosts file, so
# this suite relies on the stack default DNS_UPSTREAMS_DEFAULT=knot).
CLOAKED = "cloaked.cnametest.com"
CLOAKED_MULTIHOP = "cloaked-multihop.cnametest.com"
CLOAKED_CLEAN = "cloaked-clean.cnametest.com"
CLOAKED_SUB = "cloaked-sub.cnametest.com"
TRACKER_TARGET = "cnametracker-blocked.com"
INTERMEDIATE_TARGET = "hop.cnamechain.com"
CLEAN_TARGET = "cnameclean.com"
TRACKER_PARENT = "cnamepark.com"


class TestCNAMEUncloaking:
    """End-to-end tests for CNAME uncloaking (spec: proxy-filtering-behaviour
    Section F).

    Sites hide third-party trackers behind first-party subdomains that CNAME
    to the tracker's real domain. The recursor follows the chain internally,
    so the tracker name never appears as a QNAME — the proxy must re-check the
    CNAME targets in the answer against blocklists and custom domain rules.
    """

    @pytest.mark.asyncio
    async def test_cname_target_on_blocklist_blocks_query(
        self, user, ensure_domain_blocklisted
    ):
        """A queried name whose CNAME target is blocklisted must be blocked.

        specRef: proxy-filtering-behaviour F/U2
        """
        ensure_domain_blocklisted(TRACKER_TARGET)
        profile_id = user.new_profile("cname-uncloak")

        resp = await user.wait_for(profile_id, CLOAKED, A, is_blocked)
        assert_blocked(resp, CLOAKED)

    @pytest.mark.asyncio
    async def test_intermediate_chain_name_blocked(
        self, user, ensure_domain_blocklisted
    ):
        """Every name in a multi-hop chain is checked, not only the last one.

        specRef: proxy-filtering-behaviour F/U3
        """
        ensure_domain_blocklisted(INTERMEDIATE_TARGET)
        profile_id = user.new_profile("cname-multihop")

        resp = await user.wait_for(profile_id, CLOAKED_MULTIHOP, A, is_blocked)
        assert_blocked(resp, CLOAKED_MULTIHOP)

    @pytest.mark.asyncio
    async def test_parent_of_target_blocked_via_subdomains_rule(
        self, user, ensure_domain_blocklisted
    ):
        """A blocklisted parent of the CNAME target blocks the chain when the
        profile's subdomains rule is on (default "block").

        specRef: proxy-filtering-behaviour F/U4
        """
        ensure_domain_blocklisted(TRACKER_PARENT)
        profile_id = user.new_profile("cname-subdomain")

        resp = await user.wait_for(profile_id, CLOAKED_SUB, A, is_blocked)
        assert_blocked(resp, CLOAKED_SUB)

    @pytest.mark.asyncio
    async def test_clean_cname_chain_resolves(self, user):
        """A CNAME chain whose targets match nothing must resolve normally.

        specRef: proxy-filtering-behaviour F/U1 F/U6
        """
        profile_id = user.new_profile("cname-clean")

        resp = await user.wait_for(profile_id, CLOAKED_CLEAN, A, is_resolved)
        assert_not_blocked(resp, CLOAKED_CLEAN)

    @pytest.mark.asyncio
    async def test_custom_block_rule_matches_cname_target(self, user):
        """A custom Block rule (wildcard) on the CNAME target blocks the chain.

        specRef: proxy-filtering-behaviour F/U7
        """
        profile_id = user.new_profile("cname-custom-block")
        user.add_rule(profile_id, "block", f"*.{CLEAN_TARGET}")

        resp = await user.wait_for(profile_id, CLOAKED_CLEAN, A, is_blocked)
        assert_blocked(resp, CLOAKED_CLEAN)

    @pytest.mark.asyncio
    async def test_custom_allow_on_qname_overrides_cname_block(
        self, user, ensure_domain_blocklisted
    ):
        """A custom Allow on the queried name wins over a blocklisted target
        (global rule: any Allow beats any Block).

        specRef: proxy-filtering-behaviour F/U9
        """
        ensure_domain_blocklisted(TRACKER_TARGET)
        profile_id = user.new_profile("cname-qname-allow")
        user.add_rule(profile_id, "allow", CLOAKED)

        resp = await user.wait_for(profile_id, CLOAKED, A, is_resolved)
        assert_not_blocked(resp, CLOAKED)

    @pytest.mark.asyncio
    async def test_custom_allow_on_target_overrides_blocklist(
        self, user, ensure_domain_blocklisted
    ):
        """A custom Allow on the CNAME target itself also wins over a
        blocklist hit on that target.

        specRef: proxy-filtering-behaviour F/U8
        """
        ensure_domain_blocklisted(TRACKER_TARGET)
        profile_id = user.new_profile("cname-target-allow")
        user.add_rule(profile_id, "allow", TRACKER_TARGET)

        resp = await user.wait_for(profile_id, CLOAKED, A, is_resolved)
        assert_not_blocked(resp, CLOAKED)
