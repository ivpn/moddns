"""ProfileSession — facade bundling a logged-in account, its cookie-authenticated
API access, and DNS resolution.

Replaces the per-test ``ApiClient``/``ProfileApi``/``default_headers["Cookie"]``
boilerplate. Typical use via the class-scoped ``user`` fixture from conftest:

    async def test_block(self, user):
        pid = user.new_profile("my_case")
        user.add_rule(pid, "block", "ads.example")
        resp = await user.wait_for(pid, "ads.example", A, is_blocked)
        assert_blocked(resp, "ads.example")

For API calls the facade doesn't wrap, drop down to the raw client:

    with user.profiles_api() as p:
        p.api_v1_profiles_id_logs_get_with_http_info(...)
"""

import uuid
from contextlib import contextmanager
from dataclasses import dataclass
from typing import Any, Callable, Iterator, Optional

import moddns.api as api
import moddns.api_client as client
import moddns.configuration as api_config
from moddns import (
    ApiBlocklistsUpdates,
    ApiCreateProfileBody,
    ApiServicesUpdates,
    ModelProfileUpdate,
    RequestsCreateProfileCustomRuleBody,
    RequestsProfileUpdates,
)
from dns.message import Message

from libs.accounts import create_account, delete_account
from libs.dns_lib import DNSLib
from libs.settings import Settings, get_settings


@dataclass
class ProfileSession:
    """A logged-in test user: account, session cookie, API and DNS access."""

    account: Any
    cookie: str
    password: str
    email: str
    config: Settings
    dns: DNSLib

    @classmethod
    def create(cls, **create_account_kwargs) -> "ProfileSession":
        account, cookie, password, email = create_account(**create_account_kwargs)
        config = get_settings()
        return cls(
            account=account,
            cookie=cookie,
            password=password,
            email=email,
            config=config,
            dns=DNSLib(config.DOH_ENDPOINT),
        )

    # ------------------------------------------------------------------
    # API access
    # ------------------------------------------------------------------
    @property
    def default_profile_id(self) -> str:
        """The profile created automatically at registration."""
        return self.account.profiles[0]

    @contextmanager
    def profiles_api(self) -> Iterator[Any]:
        """Cookie-authenticated ProfileApi for calls the facade doesn't wrap."""
        api_conf = api_config.Configuration(host=self.config.DNS_API_ADDR)
        with client.ApiClient(api_conf) as api_client:
            p = api.ProfileApi(api_client)
            p.api_client.default_headers["Cookie"] = self.cookie
            yield p

    # ------------------------------------------------------------------
    # Profile management
    # ------------------------------------------------------------------
    def new_profile(self, name: Optional[str] = None) -> str:
        """Create a fresh profile and return its id.

        A unique suffix is always appended — the API rejects duplicate profile
        names per account, and parametrized tests re-enter with the same name.
        The base is truncated so the result fits the API's 50-char name limit.
        """
        suffix = f"-{uuid.uuid4().hex[:8]}"
        unique_name = f"{(name or 'p')[: 50 - len(suffix)]}{suffix}"
        with self.profiles_api() as p:
            resp = p.api_v1_profiles_post_with_http_info(
                body=ApiCreateProfileBody(name=unique_name)
            )
            assert resp.status_code == 201, (
                f"Profile creation failed: {resp.status_code}"
            )
            return resp.data.profile_id

    def get_profile(self, profile_id: str) -> Any:
        with self.profiles_api() as p:
            resp = p.api_v1_profiles_id_get_with_http_info(id=profile_id)
            assert resp.status_code == 200, (
                f"Failed to get profile {profile_id}: {resp.status_code}"
            )
            return resp.data

    def add_rule(self, profile_id: str, action: str, value: str) -> None:
        with self.profiles_api() as p:
            resp = p.api_v1_profiles_id_custom_rules_post_with_http_info(
                id=profile_id,
                body=RequestsCreateProfileCustomRuleBody(action=action, value=value),
            )
            assert resp.status_code == 201, (
                f"Custom rule creation failed for {value}: {resp.status_code}"
            )

    def block_services(self, profile_id: str, service_ids: list) -> None:
        with self.profiles_api() as p:
            resp = p.api_v1_profiles_id_services_post_with_http_info(
                id=profile_id, service_ids=ApiServicesUpdates(service_ids=service_ids)
            )
            assert resp.status_code == 200, (
                f"Service block failed for {service_ids}: {resp.status_code}"
            )

    def unblock_services(self, profile_id: str, service_ids: list) -> None:
        with self.profiles_api() as p:
            resp = p.api_v1_profiles_id_services_delete_with_http_info(
                id=profile_id, service_ids=ApiServicesUpdates(service_ids=service_ids)
            )
            assert resp.status_code == 200, (
                f"Service unblock failed for {service_ids}: {resp.status_code}"
            )

    def enable_blocklists(self, profile_id: str, blocklist_ids: list) -> None:
        with self.profiles_api() as p:
            resp = p.api_v1_profiles_id_blocklists_post_with_http_info(
                id=profile_id,
                blocklist_ids=ApiBlocklistsUpdates(blocklist_ids=blocklist_ids),
            )
            assert resp.status_code == 200, (
                f"Blocklist enable failed: {resp.status_code}"
            )

    def disable_blocklists(self, profile_id: str, blocklist_ids: list) -> None:
        with self.profiles_api() as p:
            resp = p.api_v1_profiles_id_blocklists_delete_with_http_info(
                id=profile_id,
                blocklist_ids=ApiBlocklistsUpdates(blocklist_ids=blocklist_ids),
            )
            assert resp.status_code == 200, (
                f"Blocklist disable failed: {resp.status_code}"
            )

    def patch_setting(self, profile_id: str, path: str, value: Any) -> None:
        """PATCH a single profile setting, e.g.
        ``patch_setting(pid, "/settings/privacy/blocklists_subdomains_rule", "allow")``.
        """
        with self.profiles_api() as p:
            body = RequestsProfileUpdates(
                updates=[
                    ModelProfileUpdate(
                        operation="replace", path=path, value={"value": value}
                    )
                ]
            )
            resp = p.api_v1_profiles_id_patch_with_http_info(profile_id, body=body)
            assert resp.status_code == 200, (
                f"PATCH {path} failed: {resp.status_code}"
            )

    # ------------------------------------------------------------------
    # DNS
    # ------------------------------------------------------------------
    async def resolve(self, profile_id: str, domain: str, record_type) -> Message:
        return await self.dns.send_doh_request(profile_id, domain, record_type)

    async def wait_for(
        self,
        profile_id: str,
        domain: str,
        record_type,
        predicate: Callable[[Message], bool],
        **kwargs,
    ) -> Message:
        """``DNSLib.wait_until`` shorthand — see its docstring for when (not)
        to poll."""
        return await self.dns.wait_until(
            profile_id, domain, record_type, predicate, **kwargs
        )

    # ------------------------------------------------------------------
    # Lifecycle
    # ------------------------------------------------------------------
    def cleanup(self) -> None:
        delete_account(
            self.cookie, self.password, account_id=getattr(self.account, "id", "?")
        )
