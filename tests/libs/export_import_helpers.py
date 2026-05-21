"""Helpers for profile export/import integration tests.

The generated Python API client uses strict pydantic models that reject many
of the invalid inputs we need to test (unknown scope values, schemaVersion=2,
unknown root fields, missing CSRF header, gzip Content-Encoding, …). Negative
tests therefore go through raw HTTP via ``requests`` while positive tests can
go through the generated client.

This module centralises both paths so individual tests stay short.

specRef: covers helper utility for tests against
docs/specs/account-export-import-behaviour.md
"""

from __future__ import annotations

import random
import string
from datetime import datetime, timezone
from typing import Any, Optional

import requests as http_requests

import moddns.api as api
import moddns.api_client as client
import moddns.configuration as api_config
from moddns import RequestsLoginBody
from libs.settings import get_settings


# Special-char set matching the API's `reSpecialChar` regex in
# api/internal/validator/validator.go:23. `helpers.generate_complex_password`
# draws from string.punctuation, which can pick characters outside this set
# (e.g. apostrophe, backslash) and cause flaky registration failures —
# regenerate here with a constrained pool so account creation is deterministic.
_PASSWORD_SPECIALS = "!@#$%^&*(),;.?:{}[]|<>_-"


def _stable_complex_password(length: int = 16) -> str:
    pool = string.ascii_letters + string.digits + _PASSWORD_SPECIALS
    parts = [
        random.choice(string.ascii_uppercase),
        random.choice(string.ascii_lowercase),
        random.choice(string.digits),
        random.choice(_PASSWORD_SPECIALS),
    ]
    parts.extend(random.choice(pool) for _ in range(length - 4))
    random.shuffle(parts)
    return "".join(parts)


# ---------------------------------------------------------------------------
# Account creation that retains the password (needed for reauth)
# ---------------------------------------------------------------------------
def create_account_with_password() -> tuple[Any, str, str, str]:
    """Create a new account and return (account, cookie, password, email).

    Mirrors conftest.create_acc_and_login_func but also surfaces the plaintext
    password and email so tests can perform reauth via the current_password
    path. Each call yields a fresh account so rate-limit / max-profiles tests
    stay isolated.
    """
    from conftest import create_temp_subscription  # local to avoid cycles

    config = get_settings()
    api_conf = api_config.Configuration(host=config.DNS_API_ADDR)
    with client.ApiClient(api_conf) as api_client:
        account_api = api.AccountApi(api_client)
        auth_api = api.AuthenticationApi(api_client)

        email = (
            f"test{''.join(random.choice(string.digits) for _ in range(5))}@ivpn.net"
        )
        password = _stable_complex_password()

        subscription_id, pa_cookie = create_temp_subscription()

        account_api.api_client.default_headers["Cookie"] = pa_cookie
        reg_resp = account_api.api_v1_accounts_post_with_http_info(
            body={"email": email, "password": password, "subid": subscription_id}
        )
        assert reg_resp.status_code == 201, (
            f"Registration failed with status code: {reg_resp.status_code}"
        )

        login_response = auth_api.api_v1_login_post_with_http_info(
            body=RequestsLoginBody(email=email, password=password)
        )
        assert login_response.status_code == 200, (
            f"Login failed with status code: {login_response.status_code}"
        )
        cookie = login_response.headers.get("Set-Cookie")
        assert cookie, "No session cookie returned after login"

        account_api.api_client.default_headers["Cookie"] = cookie
        account = account_api.api_v1_accounts_current_get()
        assert len(account.profiles) == 1
        return account, cookie, password, email


# ---------------------------------------------------------------------------
# Envelope builder
# ---------------------------------------------------------------------------
def build_profile(
    name: str = "TestProfile",
    *,
    blocklists: Optional[list[str]] = None,
    services: Optional[list[str]] = None,
    default_rule: str = "block",
    blocklists_subdomains_rule: str = "block",
    custom_rules_subdomains_rule: str = "include",
    dnssec_enabled: bool = True,
    dnssec_send_do_bit: bool = False,
    custom_rules: Optional[list[dict]] = None,
    logs_enabled: bool = False,
    log_clients_ips: bool = False,
    log_domains: bool = False,
    log_retention: Optional[str] = None,
    statistics_enabled: bool = False,
    comment: Optional[str] = None,
    advanced: Optional[dict] = None,
    extra: Optional[dict] = None,
) -> dict:
    """Build a single ExportedProfile dict in the on-wire camelCase shape."""
    settings: dict[str, Any] = {
        "privacy": {
            "blocklists": list(blocklists) if blocklists else [],
            "services": list(services) if services else [],
            "defaultRule": default_rule,
            "blocklistsSubdomainsRule": blocklists_subdomains_rule,
            "customRulesSubdomainsRule": custom_rules_subdomains_rule,
        },
        "security": {
            "dnssec": {
                "enabled": dnssec_enabled,
                "sendDoBit": dnssec_send_do_bit,
            },
        },
        "customRules": custom_rules or [],
        "logs": {
            "enabled": logs_enabled,
            "logClientsIPs": log_clients_ips,
            "logDomains": log_domains,
        },
        "statistics": {"enabled": statistics_enabled},
    }
    if log_retention is not None:
        settings["logs"]["retention"] = log_retention
    if advanced is not None:
        settings["advanced"] = advanced

    p: dict[str, Any] = {"name": name, "settings": settings}
    if comment is not None:
        p["comment"] = comment
    if extra:
        p.update(extra)
    return p


def build_envelope(
    profiles: Optional[list[dict]] = None,
    *,
    schema_version: int = 1,
    kind: str = "moddns-export",
    exported_at: Optional[str] = None,
    exported_from: Optional[dict] = None,
    extra: Optional[dict] = None,
) -> dict:
    """Build a complete ExportEnvelope dict in the on-wire camelCase shape.

    ``extra`` lets tests inject unknown root fields (V6, S2 negative tests).
    """
    env: dict[str, Any] = {
        "schemaVersion": schema_version,
        "kind": kind,
        "exportedAt": exported_at
        or datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
        "profiles": profiles if profiles is not None else [build_profile()],
    }
    if exported_from is not None:
        env["exportedFrom"] = exported_from
    if extra:
        env.update(extra)
    return env


# ---------------------------------------------------------------------------
# Raw HTTP wrappers (bypass pydantic for negative tests)
# ---------------------------------------------------------------------------
EXPORT_PATH = "/api/v1/profiles/export"
IMPORT_PATH = "/api/v1/profiles/import"


def _api_base() -> str:
    return get_settings().DNS_API_ADDR.rstrip("/")


def raw_export(
    cookie: str,
    body: dict,
    *,
    headers: Optional[dict] = None,
    json_override: Any = None,
) -> http_requests.Response:
    """POST /profiles/export via raw HTTP. Use for negative tests."""
    url = _api_base() + EXPORT_PATH
    hdrs = {"Cookie": cookie, "Content-Type": "application/json"}
    if headers:
        hdrs.update(headers)
    if json_override is not None:
        return http_requests.post(url, headers=hdrs, data=json_override)
    return http_requests.post(url, headers=hdrs, json=body)


def raw_import(
    cookie: str,
    body: dict,
    *,
    headers: Optional[dict] = None,
    include_csrf: bool = True,
    json_override: Any = None,
    data_override: Optional[bytes] = None,
) -> http_requests.Response:
    """POST /profiles/import via raw HTTP. Use for negative tests.

    By default the X-modDNS-Import: confirm CSRF header is included. Set
    ``include_csrf=False`` or pass a different value via ``headers`` to test
    the I4 / S7 rejection path.
    """
    url = _api_base() + IMPORT_PATH
    hdrs = {"Cookie": cookie, "Content-Type": "application/json"}
    if include_csrf:
        hdrs["X-modDNS-Import"] = "confirm"
    if headers:
        hdrs.update(headers)
    if data_override is not None:
        return http_requests.post(url, headers=hdrs, data=data_override)
    if json_override is not None:
        return http_requests.post(url, headers=hdrs, data=json_override)
    return http_requests.post(url, headers=hdrs, json=body)


def export_request_body(
    *,
    scope: str = "all",
    profile_ids: Optional[list[str]] = None,
    current_password: Optional[str] = None,
    reauth_token: Optional[str] = None,
) -> dict:
    """Build the export request body. Snake-case per the request DTO."""
    body: dict[str, Any] = {"scope": scope}
    if profile_ids is not None:
        body["profile_ids"] = profile_ids
    if current_password is not None:
        body["current_password"] = current_password
    if reauth_token is not None:
        body["reauth_token"] = reauth_token
    return body


def import_request_body(
    *,
    mode: Optional[str] = "create_new",
    payload: Optional[dict] = None,
    current_password: Optional[str] = None,
    reauth_token: Optional[str] = None,
    extra: Optional[dict] = None,
) -> dict:
    """Build the import request body. Outer envelope is snake-case; payload
    stays camelCase (matches the on-wire format)."""
    body: dict[str, Any] = {}
    if mode is not None:
        body["mode"] = mode
    if payload is not None:
        body["payload"] = payload
    if current_password is not None:
        body["current_password"] = current_password
    if reauth_token is not None:
        body["reauth_token"] = reauth_token
    if extra:
        body.update(extra)
    return body


def do_export(
    cookie: str, password: str, *, scope: str = "all", profile_ids=None
) -> http_requests.Response:
    """Happy-path export through raw HTTP. Returns the full Response so tests
    can inspect headers (Content-Disposition, Content-Type, Cache-Control)."""
    return raw_export(
        cookie,
        export_request_body(
            scope=scope, profile_ids=profile_ids, current_password=password
        ),
    )


def do_import(
    cookie: str,
    password: str,
    payload: dict,
    *,
    mode: str = "create_new",
    headers: Optional[dict] = None,
    include_csrf: bool = True,
) -> http_requests.Response:
    """Happy-path import through raw HTTP."""
    return raw_import(
        cookie,
        import_request_body(mode=mode, payload=payload, current_password=password),
        headers=headers,
        include_csrf=include_csrf,
    )
