"""Backend E2E tests for the signup-reset (account retirement) flow.

specRef: docs/specs/signup-reset-behaviour.md (RT3, RT5-RT8, R-E9, and the
no-false-positive invariant)

The ZLA token_hash is stable per IVPN customer, so a second signup carrying the
same token (a reset re-signup) must retire the previous account: it drops to
pending_delete (token_hash unset, deletion_scheduled_at set) and can no longer
be resynced. Two signups with *different* tokens are unrelated and must both
stay active.

Retirement runs in a best-effort background goroutine after the signup HTTP
response, so the tests poll the previous account's status until it flips.
"""

import base64
import hashlib
import os as _os
import random
import string
import time
import uuid
from datetime import datetime, timedelta, timezone

import pytest
import requests as http_requests

import moddns.api as api
import moddns.api_client as client
import moddns.configuration as api_config
from moddns import RequestsLoginBody
from moddns.api.pa_session_api import PASessionApi
from moddns.exceptions import ApiException
from moddns.models.requests_pa_session_req import RequestsPASessionReq
from moddns.models.requests_rotate_pa_session_req import RequestsRotatePASessionReq

from helpers import generate_complex_password
from libs.settings import get_settings

RETIREMENT_TIMEOUT_S = 20


def _api_conf():
    return api_config.Configuration(host=get_settings().DNS_API_ADDR)


def _random_email() -> str:
    return f"reset{''.join(random.choice(string.digits) for _ in range(8))}@ivpn.net"


def _provision_pa_session(token: str, validity_days: int = 30, tier: str = "Tier 2"):
    """Provision a PASession for a SPECIFIC ZLA token.

    Unlike conftest.create_temp_subscription (which randomises the token), this
    lets two signups share the same token — and therefore the same token_hash,
    the signal modDNS uses to detect a reset re-signup.
    """
    subscription_id = str(uuid.uuid4())
    session_id = str(uuid.uuid4())
    preauth_id = str(uuid.uuid4())
    active_until = (
        datetime.utcnow().replace(tzinfo=timezone.utc) + timedelta(days=validity_days)
    ).isoformat().replace("+00:00", "Z")
    token_hash = base64.b64encode(hashlib.sha256(token.encode()).digest()).decode()

    mock_preauth_url = _os.getenv("MOCK_PREAUTH_URL", "http://localhost:8080")
    http_requests.post(
        f"{mock_preauth_url}/entry",
        json={
            "id": preauth_id,
            "token_hash": token_hash,
            "is_active": True,
            "active_until": active_until,
            "tier": tier,
        },
    ).raise_for_status()

    api_conf = _api_conf()
    with client.ApiClient(api_conf) as api_client:
        pa_api = PASessionApi(api_client)
        pa_api.api_client.default_headers["Authorization"] = "Bearer "
        pa_api.api_v1_pasession_add_post(
            body=RequestsPASessionReq(id=session_id, preauth_id=preauth_id, token=token)
        )
    with client.ApiClient(api_conf) as api_client:
        pa_api = PASessionApi(api_client)
        rotate = pa_api.api_v1_pasession_rotate_put_with_http_info(
            body=RequestsRotatePASessionReq(sessionid=session_id)
        )
        assert rotate.status_code == 200, f"PASession rotate failed: {rotate.status_code}"
        pa_cookie = rotate.headers.get("Set-Cookie", "")
        assert "pa_session=" in pa_cookie, f"no pa_session cookie: {pa_cookie}"
    return subscription_id, pa_cookie


def _signup_and_login(token: str) -> str:
    """Register a new account whose ZLA token is `token`, then log in.

    Returns the session cookie.
    """
    email = _random_email()
    password = generate_complex_password()
    subscription_id, pa_cookie = _provision_pa_session(token)

    api_conf = _api_conf()
    with client.ApiClient(api_conf) as api_client:
        account_api = api.AccountApi(api_client)
        account_api.api_client.default_headers["Cookie"] = pa_cookie
        reg = account_api.api_v1_accounts_post_with_http_info(
            body={"email": email, "password": password, "subid": subscription_id}
        )
        assert reg.status_code == 201, f"registration failed: {reg.status_code}"

        auth_api = api.AuthenticationApi(api_client)
        login = auth_api.api_v1_login_post_with_http_info(
            body=RequestsLoginBody(email=email, password=password)
        )
        assert login.status_code == 200, f"login failed: {login.status_code}"
        cookie = login.headers.get("Set-Cookie")
        assert cookie, "no session cookie after login"
    return cookie


def _get_status(cookie: str) -> str:
    api_conf = _api_conf()
    with client.ApiClient(api_conf) as api_client:
        sub_api = api.SubscriptionApi(api_client)
        sub_api.api_client.default_headers["Cookie"] = cookie
        return sub_api.api_v1_sub_get().status


def _wait_for_status(cookie: str, want: str, timeout_s: int = RETIREMENT_TIMEOUT_S) -> str:
    deadline = time.time() + timeout_s
    status = None
    while time.time() < deadline:
        status = _get_status(cookie)
        if status == want:
            return status
        time.sleep(1)
    return status


def test_signup_reset_retires_previous_account(start_compose):
    """A second signup sharing the token_hash retires the first account."""
    token = str(uuid.uuid4())  # shared => same token_hash for both signups

    # First signup (account A) — the account that will be reset.
    cookie_a = _signup_and_login(token)
    assert _get_status(cookie_a) == "active", "account A should start active"

    # Second signup (account B) — the post-reset re-signup, same customer/token.
    cookie_b = _signup_and_login(token)

    # Retirement is backgrounded: poll A until it flips to pending_delete.
    status_a = _wait_for_status(cookie_a, "pending_delete")
    assert status_a == "pending_delete", (
        f"account A should be retired (pending_delete) after the second signup, got {status_a}"
    )

    # Account B is the live account.
    assert _get_status(cookie_b) == "active", "account B (new signup) should be active"


def test_retired_account_cannot_resync(start_compose):
    """A retired account must not be resurrected via resync (HTTP 409)."""
    token = str(uuid.uuid4())
    cookie_a = _signup_and_login(token)
    cookie_b = _signup_and_login(token)  # retires A

    assert _wait_for_status(cookie_a, "pending_delete") == "pending_delete", (
        "precondition: account A must be retired before testing resync"
    )

    # Resync on the retired account is refused with 409 — the guard fires before
    # any preauth/write, so no pa_session cookie is needed to observe it.
    api_conf = _api_conf()
    with client.ApiClient(api_conf) as api_client:
        sub_api = api.SubscriptionApi(api_client)
        sub_api.api_client.default_headers["Cookie"] = cookie_a
        with pytest.raises(ApiException) as exc:
            sub_api.api_v1_sub_update_put(body=None)
        assert exc.value.status == 409, f"expected 409 for retired resync, got {exc.value.status}"

    # The new account B is unaffected.
    assert _get_status(cookie_b) == "active"


def test_distinct_tokens_do_not_retire(start_compose):
    """Two signups with DIFFERENT tokens are unrelated — neither is retired.

    Guards against false-positive retirement (the detection must be scoped to a
    shared token_hash, not fire on every signup).
    """
    cookie_x = _signup_and_login(str(uuid.uuid4()))
    cookie_y = _signup_and_login(str(uuid.uuid4()))

    # Give any (erroneous) background retirement a chance to run before asserting.
    time.sleep(3)

    assert _get_status(cookie_x) == "active", "first account must stay active (distinct token)"
    assert _get_status(cookie_y) == "active", "second account must stay active (distinct token)"
