"""Passkey (WebAuthn) flows for the backend E2E tests.

Every call goes through the generated client (``moddns.api.AuthenticationApi``)
with ``*_without_preload_content`` so tests see the raw status and JSON. The API's
cookies are ``Secure`` and the suite talks plain HTTP, so cookies are carried by
hand, as ``libs/accounts.py`` does. Browser responses come from
``libs/webauthn_authenticator.SoftAuthenticator``.
"""

import json
import re
from contextlib import contextmanager
from dataclasses import dataclass, field
from functools import lru_cache
from typing import Any, Iterator, Optional

import requests

import moddns.api as api
import moddns.api_client as client
import moddns.configuration as api_config
from pymongo import MongoClient

from libs.accounts import create_temp_subscription, random_email
from libs.settings import get_settings
from libs.webauthn_authenticator import SoftAuthenticator

AUTH_COOKIE = "auth"
TEMP_COOKIE = "webauthn_temp"
PA_SESSION_COOKIE = "pa_session"
CEREMONY_KEY_PREFIX = "webauthn_ceremony:"

REAUTH_PURPOSES = (
    "email_change",
    "account_deletion",
    "profile_export",
    "profile_import",
    "password_set",
    "passkey_add",
    "passkey_delete",
)


@dataclass
class Response:
    status: int
    body: Any
    set_cookie: str

    def cookie(self, name: str) -> Optional[str]:
        """The last non-empty value this response sets for cookie ``name``, if any."""
        values = re.findall(rf"(?:^|[,\s]){re.escape(name)}=([^;,\s]*)", self.set_cookie)
        return next((v for v in reversed(values) if v), None)


def cookie_header(**cookies: Optional[str]) -> str:
    return "; ".join(f"{name}={value}" for name, value in cookies.items() if value)


def cookie_value(set_cookie: str, name: str) -> Optional[str]:
    return Response(0, None, set_cookie).cookie(name)


@contextmanager
def auth_api(cookie: str = "") -> Iterator[Any]:
    conf = api_config.Configuration(host=get_settings().DNS_API_ADDR)
    with client.ApiClient(conf) as api_client:
        auth = api.AuthenticationApi(api_client)
        if cookie:
            auth.api_client.default_headers["Cookie"] = cookie
        yield auth


@contextmanager
def account_api(cookie: str) -> Iterator[Any]:
    conf = api_config.Configuration(host=get_settings().DNS_API_ADDR)
    with client.ApiClient(conf) as api_client:
        acc = api.AccountApi(api_client)
        acc.api_client.default_headers["Cookie"] = cookie
        yield acc


def call(method, **kwargs) -> Response:
    """Run a ``*_without_preload_content`` client method and decode its JSON body.

    Some endpoints answer with a plain-text status line (``201 Created``); that body is kept as text.
    """
    raw = method(**kwargs)
    try:
        body = json.loads(raw.data) if raw.data else None
    except ValueError:
        body = raw.data.decode(errors="replace")
    return Response(status=raw.status, body=body, set_cookie=", ".join(raw.headers.getlist("Set-Cookie")))


def raw_request(method: str, path: str, cookie: str, body: dict) -> Response:
    """A request the generated client refuses to send because it validates the body itself."""
    resp = requests.request(method, f"{get_settings().DNS_API_ADDR}{path}", json=body,
                            headers={"Cookie": cookie}, timeout=30)
    try:
        payload = resp.json()
    except ValueError:
        payload = resp.text
    return Response(status=resp.status_code, body=payload, set_cookie=resp.headers.get("Set-Cookie", ""))


@lru_cache()
def mongo():
    cfg = get_settings()
    return MongoClient(cfg.MONGO_URI)[cfg.MONGO_DB]


def stored_credential(credential_id: bytes) -> Optional[dict]:
    """The credentials document of a WebAuthn credential id."""
    return mongo().credentials.find_one({"credential_id": credential_id})


@dataclass
class PasskeyUser:
    """An account signed up with a passkey, and the authenticator that holds it."""

    email: str
    authenticator: SoftAuthenticator
    credential_ids: list[bytes] = field(default_factory=list)
    session: Optional[str] = None
    account_id: Optional[str] = None

    @property
    def credential_id(self) -> bytes:
        return self.credential_ids[0]

    @property
    def session_cookie(self) -> str:
        return cookie_header(**{AUTH_COOKIE: self.session})


def new_authenticator(**flags) -> SoftAuthenticator:
    return SoftAuthenticator(origin=get_settings().API_ALLOW_ORIGIN, **flags)


# ----------------------------------------------------------------------
# Signup (F1, F2)
# ----------------------------------------------------------------------
def register_begin(email: str, sub_id: str, pa_cookie: str) -> Response:
    with auth_api(cookie_header(**{PA_SESSION_COOKIE: pa_cookie})) as a:
        return call(a.api_v1_webauthn_register_begin_post_without_preload_content, body={"email": email, "subid": sub_id})


def register_finish(temp: Optional[str], pa_cookie: str, credential: dict) -> Response:
    with auth_api(cookie_header(**{PA_SESSION_COOKIE: pa_cookie, TEMP_COOKIE: temp})) as a:
        return call(a.api_v1_webauthn_register_finish_post_without_preload_content, body=credential)


def signup(authenticator: Optional[SoftAuthenticator] = None, *, email: Optional[str] = None) -> PasskeyUser:
    """Sign up with a passkey through the ZLA pre-auth flow, then sign in with it."""
    authenticator = authenticator or new_authenticator()
    email = email or random_email("passkey")
    sub_id, pa_set_cookie = create_temp_subscription()
    pa_cookie = cookie_value(pa_set_cookie, PA_SESSION_COOKIE)

    begin = register_begin(email, sub_id, pa_cookie)
    assert begin.status == 200, f"register/begin: {begin.status} {begin.body}"
    credential_id, credential = authenticator.create(begin.body)
    finish = register_finish(begin.cookie(TEMP_COOKIE), pa_cookie, credential)
    assert finish.status == 201, f"register/finish: {finish.status} {finish.body}"

    user = PasskeyUser(email=email, authenticator=authenticator, credential_ids=[credential_id])
    resp = login(user)
    assert resp.status == 201, f"login/finish: {resp.status} {resp.body}"
    user.account_id = str(stored_credential(credential_id)["account_id"])
    return user


# ----------------------------------------------------------------------
# Login (A4, A5)
# ----------------------------------------------------------------------
def login_begin(email: str) -> Response:
    with auth_api() as a:
        return call(a.api_v1_webauthn_login_begin_post_without_preload_content, body={"email": email})


def login_finish(temp: Optional[str], assertion: dict) -> Response:
    with auth_api(cookie_header(**{TEMP_COOKIE: temp})) as a:
        return call(a.api_v1_webauthn_login_finish_post_without_preload_content, body=assertion)


def login(user: PasskeyUser, credential_id: Optional[bytes] = None, *, sign_count: Optional[int] = None) -> Response:
    """Sign in with a passkey; on success the session is kept on ``user``."""
    begin = login_begin(user.email)
    assert begin.status == 200, f"login/begin: {begin.status} {begin.body}"
    assertion = user.authenticator.get(begin.body, credential_id or user.credential_id, sign_count=sign_count)
    finish = login_finish(begin.cookie(TEMP_COOKIE), assertion)
    if finish.status == 201:
        user.session = finish.cookie(AUTH_COOKIE)
    return finish


# ----------------------------------------------------------------------
# Reauth (F5, F6)
# ----------------------------------------------------------------------
def reauth_begin(session_cookie: str, purpose: str) -> Response:
    with auth_api(session_cookie) as a:
        return call(a.api_v1_webauthn_passkey_reauth_begin_post_without_preload_content, body={"purpose": purpose})


def reauth_finish(session_cookie: str, temp: Optional[str], assertion: dict) -> Response:
    cookie = "; ".join(c for c in (session_cookie, cookie_header(**{TEMP_COOKIE: temp})) if c)
    with auth_api(cookie) as a:
        return call(a.api_v1_webauthn_passkey_reauth_finish_post_without_preload_content, body=assertion)


def reauth(user: PasskeyUser, purpose: str, credential_id: Optional[bytes] = None) -> Response:
    begin = reauth_begin(user.session_cookie, purpose)
    assert begin.status == 200, f"reauth/begin: {begin.status} {begin.body}"
    assertion = user.authenticator.get(begin.body, credential_id or user.credential_id)
    return reauth_finish(user.session_cookie, begin.cookie(TEMP_COOKIE), assertion)


def reauth_token(user: PasskeyUser, purpose: str) -> str:
    resp = reauth(user, purpose)
    assert resp.status == 201, f"reauth/finish: {resp.status} {resp.body}"
    return resp.body["reauth_token"]


# ----------------------------------------------------------------------
# Add and delete passkeys (F3, F3b, F4, F8, F8a)
# ----------------------------------------------------------------------
def add_passkey_begin(session_cookie: str, *, reauth_token: Optional[str] = None,
                      current_password: Optional[str] = None) -> Response:
    body = {k: v for k, v in {"reauth_token": reauth_token, "current_password": current_password}.items() if v}
    with auth_api(session_cookie) as a:
        return call(a.api_v1_webauthn_passkey_add_begin_post_without_preload_content, body=body)


def add_passkey_finish(session_cookie: str, temp: Optional[str], credential: dict) -> Response:
    cookie = "; ".join(c for c in (session_cookie, cookie_header(**{TEMP_COOKIE: temp})) if c)
    with auth_api(cookie) as a:
        return call(a.api_v1_webauthn_passkey_add_finish_post_without_preload_content, body=credential)


def add_passkey(user: PasskeyUser, *, reauth_token: Optional[str] = None,
                current_password: Optional[str] = None) -> bytes:
    """Add a passkey on ``user.authenticator``; returns the new credential id."""
    begin = add_passkey_begin(user.session_cookie, reauth_token=reauth_token, current_password=current_password)
    assert begin.status == 200, f"passkey/add/begin: {begin.status} {begin.body}"
    credential_id, credential = user.authenticator.create(begin.body)
    finish = add_passkey_finish(user.session_cookie, begin.cookie(TEMP_COOKIE), credential)
    assert finish.status == 201, f"passkey/add/finish: {finish.status} {finish.body}"
    user.credential_ids.append(credential_id)
    return credential_id


def delete_passkey(session_cookie: str, credential_id: bytes, *, reauth_token: Optional[str] = None,
                   current_password: Optional[str] = None) -> Response:
    """Delete the passkey whose WebAuthn credential id is ``credential_id``."""
    doc = stored_credential(credential_id)
    record_id = str(doc["_id"]) if doc else "000000000000000000000000"
    body = {k: v for k, v in {"reauth_token": reauth_token, "current_password": current_password}.items() if v}
    with auth_api(session_cookie) as a:
        return call(a.api_v1_webauthn_passkey_id_delete_without_preload_content, id=record_id, body=body)


# ----------------------------------------------------------------------
# Account operations a reauth token protects
# ----------------------------------------------------------------------
def set_first_password(session_cookie: str, password: str, *, reauth_token: Optional[str]) -> Response:
    """JSON-patch the password. Sent raw: the generator types ``AccountUpdate.value`` (``{}``,
    any) as a dict, so the generated client refuses the password string."""
    updates = []
    if reauth_token is not None:
        updates.append({"operation": "test", "path": "/password", "value": {"reauth_token": reauth_token}})
    updates.append({"operation": "replace", "path": "/password", "value": password})
    return raw_request("PATCH", "/api/v1/accounts", session_cookie, {"updates": updates})


def export_profiles(session_cookie: str, *, reauth_token: str) -> Response:
    conf = api_config.Configuration(host=get_settings().DNS_API_ADDR)
    with client.ApiClient(conf) as api_client:
        p = api.ProfileApi(api_client)
        p.api_client.default_headers["Cookie"] = session_cookie
        return call(p.api_v1_profiles_export_post_without_preload_content,
                    body={"scope": "all", "reauth_token": reauth_token})


def delete_passkey_account(user: PasskeyUser) -> None:
    """Best-effort deletion of a passkey-only account through a passkey reauth token."""
    try:
        if not user.session:
            login(user)
        token = reauth_token(user, "account_deletion")
        with account_api(user.session_cookie) as acc:
            code = call(acc.api_v1_accounts_current_deletion_code_post_without_preload_content).body["code"]
            resp = call(acc.api_v1_accounts_current_delete_without_preload_content,
                        body={"deletion_code": code, "reauth_token": token})
            assert resp.status in (200, 204), f"account deletion: {resp.status} {resp.body}"
    except Exception as e:  # cleanup must not fail an otherwise green test
        print(f"Warning: failed to delete passkey account {user.email}: {e}")
