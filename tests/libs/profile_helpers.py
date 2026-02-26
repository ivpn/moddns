"""Shared helpers for integration tests that manage profiles, custom rules, services, and blocklists."""

import uuid

from libs.dns_lib import DNSLib
from dns.rdatatype import A

import moddns.api_client as client
import moddns.api as api
import moddns.configuration as api_config
from moddns import (
    RequestsCreateProfileCustomRuleBody,
    ApiCreateProfileBody,
    ApiServicesUpdates,
    ApiBlocklistsUpdates,
)

# ---------------------------------------------------------------------------
# Constants — deterministic via testhosts.txt
# ---------------------------------------------------------------------------
SVC_GOOGLE_DOMAIN = "svctest-google.com"
SVC_GOOGLE_IP = "8.8.8.8"  # AS15169 (Google)
SVC_GOOGLE_ID = "google"

TEST_DOMAIN = "test.com"
TEST_IP = "104.18.74.230"  # AS13335 (Cloudflare, not in catalog)

TEST_BLOCKLIST_ID = "hagezi_threat_intelligence_feeds_full"


# ---------------------------------------------------------------------------
# Helpers mixed into test classes
# ---------------------------------------------------------------------------
class ProfileHelpers:
    """Shared helpers for profile and rule management."""

    def _create_profile(self, p, name):
        body = ApiCreateProfileBody(name=name)
        resp = p.api_v1_profiles_post_with_http_info(body=body)
        assert resp.status_code == 201, (
            f"Profile creation failed: {resp.status_code}"
        )
        return resp.data.profile_id

    def _create_custom_rule(self, p, profile_id, action, value):
        body = RequestsCreateProfileCustomRuleBody(action=action, value=value)
        resp = p.api_v1_profiles_id_custom_rules_post_with_http_info(
            id=profile_id, body=body
        )
        assert resp.status_code == 201, (
            f"Custom rule creation failed for {value}: {resp.status_code}"
        )

    def _block_service(self, p, profile_id, service_ids):
        body = ApiServicesUpdates(service_ids=service_ids)
        resp = p.api_v1_profiles_id_services_post_with_http_info(
            id=profile_id, service_ids=body
        )
        assert resp.status_code == 200, (
            f"Service block failed for {service_ids}: {resp.status_code}"
        )

    def _unblock_service(self, p, profile_id, service_ids):
        body = ApiServicesUpdates(service_ids=service_ids)
        resp = p.api_v1_profiles_id_services_delete_with_http_info(
            id=profile_id, service_ids=body
        )
        assert resp.status_code == 200, (
            f"Service unblock failed for {service_ids}: {resp.status_code}"
        )

    def _enable_blocklist(self, p, profile_id, blocklist_ids):
        body = ApiBlocklistsUpdates(blocklist_ids=blocklist_ids)
        resp = p.api_v1_profiles_id_blocklists_post_with_http_info(
            id=profile_id, blocklist_ids=body
        )
        assert resp.status_code == 200, (
            f"Blocklist enable failed: {resp.status_code}"
        )


def extract_ip(resp):
    """Extract the first IP string from a DNS answer section."""
    assert resp.answer, "Expected a DNS answer section"
    return resp.answer[0].to_text().split(" ")[-1]


_svc_available_cache: dict[str, bool] = {}


async def services_available(dns_lib, profiles_api, cookie):
    """Probe whether services/ASN blocking is operational.

    Creates a throwaway profile, blocks the google service, queries
    svctest-google.com and checks if it gets blocked.  Returns True/False.

    Results are cached per cookie (i.e. per account) so that the probe
    runs at most once per class-scoped fixture.
    """
    if cookie in _svc_available_cache:
        return _svc_available_cache[cookie]

    result = await _services_available_probe(dns_lib, profiles_api)
    _svc_available_cache[cookie] = result
    return result


async def _services_available_probe(dns_lib, profiles_api):
    """Run the actual probe — create profile, block google, check DNS."""
    try:
        suffix = uuid.uuid4().hex[:8]
        body = ApiCreateProfileBody(name=f"svc_probe_{suffix}")
        resp = profiles_api.api_v1_profiles_post_with_http_info(body=body)
        if resp.status_code != 201:
            return False
        probe_id = resp.data.profile_id

        svc_body = ApiServicesUpdates(service_ids=[SVC_GOOGLE_ID])
        profiles_api.api_v1_profiles_id_services_post_with_http_info(
            id=probe_id, service_ids=svc_body
        )

        dns_resp = await dns_lib.send_doh_request(probe_id, SVC_GOOGLE_DOMAIN, A)
        ip_str = extract_ip(dns_resp)
        return ip_str == "0.0.0.0"
    except Exception:
        return False
