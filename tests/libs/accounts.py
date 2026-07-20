"""Account and subscription provisioning shared by fixtures and tests.

Single home for the ZLA signup flow (mock-preauth entry → PASession add/rotate
→ register → login → fetch account) and account deletion. ``conftest``
re-exports the entry points so existing ``from conftest import …`` sites keep
working.
"""

import base64
import hashlib
import random
import string
import uuid
from datetime import datetime, timedelta, timezone
from typing import Any, Optional

import requests as http_requests

import moddns.api as api
import moddns.api_client as client
import moddns.configuration as api_config
from moddns import RequestsLoginBody
from moddns.api.pa_session_api import PASessionApi
from moddns.models.requests_account_deletion_request import (
    RequestsAccountDeletionRequest,
)
from moddns.models.requests_pa_session_req import RequestsPASessionReq
from moddns.models.requests_rotate_pa_session_req import RequestsRotatePASessionReq

from libs.settings import get_settings


def random_email(prefix: str = "test") -> str:
    return f"{prefix}{''.join(random.choice(string.digits) for _ in range(5))}@ivpn.net"


def generate_complex_password(length: int = 16) -> str:
    """Generate a random password with at least one uppercase letter, one
    lowercase letter, one digit and one special character.

    The API accepts any non-alphanumeric character as the special character
    (OWASP guidance), so the full string.punctuation pool is safe.
    """
    password_chars = [
        random.choice(string.ascii_uppercase),
        random.choice(string.ascii_lowercase),
        random.choice(string.digits),
        random.choice(string.punctuation),
    ]
    password_chars.extend(
        random.choice(string.ascii_letters + string.digits + string.punctuation)
        for _ in range(length - 4)
    )
    random.shuffle(password_chars)
    return "".join(password_chars)


def create_temp_subscription(
    validity_days: int = 30,
    *,
    token: Optional[str] = None,
    tier: str = "Tier 2",
) -> tuple[str, str]:
    """Provision a pre-auth session (PASession) for the ZLA signup flow.

    Flow:
      1. Generate a token (random unless ``token`` is given) and its SHA256 hash
      2. Create a preauth entry in the mock preauth service
      3. Call POST /api/v1/pasession/add with PSK to cache the PASession
      4. Call PUT /api/v1/pasession/rotate to get a rotated session cookie
      5. Return (subscription_id, pa_session_cookie)

    Pass ``token`` explicitly to make two signups share the same token_hash —
    the signal modDNS uses to detect a signup-reset re-signup.
    """
    config = get_settings()

    subscription_id = str(uuid.uuid4())
    session_id = str(uuid.uuid4())
    preauth_id = str(uuid.uuid4())
    if token is None:
        token = str(uuid.uuid4())

    active_until_dt = datetime.now(timezone.utc) + timedelta(days=validity_days)
    active_until = active_until_dt.isoformat().replace("+00:00", "Z")

    # Compute token hash (SHA256, base64-encoded) matching what the API validates
    token_hash = base64.b64encode(hashlib.sha256(token.encode()).digest()).decode()

    # 1. Create preauth entry in mock preauth service
    http_requests.post(
        f"{config.MOCK_PREAUTH_URL}/entry",
        json={
            "id": preauth_id,
            "token_hash": token_hash,
            "is_active": True,
            "active_until": active_until,
            "tier": tier,
        },
    ).raise_for_status()

    # 2. Add PASession via API (PSK-protected endpoint)
    api_conf = api_config.Configuration(host=config.DNS_API_ADDR)
    psk = ""  # empty PSK works if no PSK is set in API .env

    with client.ApiClient(api_conf) as api_client:
        pa_api = PASessionApi(api_client)
        pa_api.api_client.default_headers["Authorization"] = f"Bearer {psk}"
        body = RequestsPASessionReq(id=session_id, preauth_id=preauth_id, token=token)
        resp = pa_api.api_v1_pasession_add_post(body=body)
        assert (
            resp.get("message") == "pre-auth session added"
        ), f"Unexpected PASession add response: {resp}"

    # 3. Rotate PASession to get cookie
    with client.ApiClient(api_conf) as api_client:
        pa_api = PASessionApi(api_client)
        rotate_body = RequestsRotatePASessionReq(sessionid=session_id)
        rotate_resp = pa_api.api_v1_pasession_rotate_put_with_http_info(
            body=rotate_body
        )
        assert rotate_resp.status_code == 200, (
            f"PASession rotation failed: {rotate_resp.status_code}"
        )
        pa_cookie = rotate_resp.headers.get("Set-Cookie", "")
        assert "pa_session=" in pa_cookie, (
            f"No pa_session cookie in rotation response: {pa_cookie}"
        )

    return subscription_id, pa_cookie


def create_account(
    *,
    email: Optional[str] = None,
    password: Optional[str] = None,
    token: Optional[str] = None,
    tier: str = "Tier 2",
) -> tuple[Any, str, str, str]:
    """Register a fresh account via the ZLA flow, log in, fetch the account.

    Returns ``(account, cookie, password, email)``. The plaintext password is
    returned so callers can perform reauth flows (e.g. account deletion).
    """
    config = get_settings()
    api_conf = api_config.Configuration(host=config.DNS_API_ADDR)
    email = email or random_email()
    password = password or generate_complex_password()

    subscription_id, pa_cookie = create_temp_subscription(token=token, tier=tier)

    with client.ApiClient(api_conf) as api_client:
        account_api = api.AccountApi(api_client)
        auth_api = api.AuthenticationApi(api_client)

        account_api.api_client.default_headers["Cookie"] = pa_cookie
        reg_resp = account_api.api_v1_accounts_post_with_http_info(
            body={"email": email, "password": password, "subid": subscription_id}
        )
        assert (
            reg_resp.status_code == 201
        ), f"Registration failed with status code: {reg_resp.status_code}"

        login_response = auth_api.api_v1_login_post_with_http_info(
            body=RequestsLoginBody(email=email, password=password)
        )
        assert (
            login_response.status_code == 200
        ), f"Login failed with status code: {login_response.status_code}"
        cookie = login_response.headers.get("Set-Cookie")
        assert cookie, "No session cookie returned after login"

        account_api.api_client.default_headers["Cookie"] = cookie
        account = account_api.api_v1_accounts_current_get()
        assert len(account.profiles) == 1
        return account, cookie, password, email


def delete_account(cookie: str, password: str, *, account_id: str = "?") -> None:
    """Best-effort account deletion via the deletion-code + password-reauth flow.

    Deleting the account removes all its profiles and cached state, so test
    runs don't accumulate data in Mongo/Redis. Failures are logged, not raised —
    cleanup problems must not fail an otherwise green test.
    """
    try:
        config = get_settings()
        api_conf = api_config.Configuration(host=config.DNS_API_ADDR)
        with client.ApiClient(api_conf) as api_client:
            account_api = api.AccountApi(api_client)
            account_api.api_client.default_headers["Cookie"] = cookie
            code_resp = account_api.api_v1_accounts_current_deletion_code_post()
            resp = account_api.api_v1_accounts_current_delete_with_http_info(
                body=RequestsAccountDeletionRequest(
                    deletion_code=code_resp.code, current_password=password
                )
            )
            assert resp.status_code in (200, 204), (
                f"Account deletion failed with status code: {resp.status_code}"
            )
    except Exception as e:
        print(f"Warning: Failed to delete test account {account_id}: {e}")
