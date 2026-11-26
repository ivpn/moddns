"""Signing out the other sessions of an account.

specRef: api-endpoint-behaviour #A3 #F5.
"""

import pytest
import requests
from moddns import ApiClient, Configuration
from moddns.api.authentication_api import AuthenticationApi
from moddns.models.requests_login_body import RequestsLoginBody

from libs import passkeys
from libs.accounts import create_account, delete_account
from libs.settings import get_settings


def _delete_other_sessions(cookie: str, body: dict) -> int:
    resp = requests.delete(f"{get_settings().DNS_API_ADDR}/api/v1/sessions", json=body,
                           headers={"Cookie": cookie}, timeout=30)
    return resp.status_code


def _signed_in(cookie: str) -> bool:
    resp = requests.get(f"{get_settings().DNS_API_ADDR}/api/v1/accounts/current",
                        headers={"Cookie": cookie}, timeout=30)
    return resp.status_code == 200


def _second_session(email: str, password: str) -> str:
    with ApiClient(Configuration(host=get_settings().DNS_API_ADDR)) as api_client:
        resp = AuthenticationApi(api_client).api_v1_login_post_with_http_info(
            RequestsLoginBody(email=email, password=password)
        )
        return resp.headers.get("Set-Cookie")


@pytest.fixture
def two_sessions():
    account, cookie, password, email = create_account()
    yield cookie, _second_session(email, password), password
    delete_account(cookie, password, account_id=account.id)


@pytest.mark.parametrize("body", [{}, {"current_password": "Wrong-Passw0rd-123!"}], ids=["none", "wrong-password"])
def test_refused_reauth_signs_nothing_out(two_sessions, body):
    """A session alone cannot sign out the other sessions."""
    current, other, _ = two_sessions
    assert _delete_other_sessions(current, body) == 400
    assert _signed_in(current) and _signed_in(other)


def test_password_signs_out_the_other_sessions(two_sessions):
    current, other, password = two_sessions
    assert _delete_other_sessions(current, {"current_password": password}) == 204
    assert _signed_in(current)
    assert not _signed_in(other)


def test_passkey_reauth_signs_out_the_other_sessions():
    user = passkeys.signup()
    first = user.session
    try:
        current = user.session_cookie
        assert passkeys.login(user).status == 201
        other = user.session_cookie

        wrong = passkeys.reauth_token(user, "passkey_delete")
        assert _delete_other_sessions(current, {"reauth_token": wrong}) == 400
        assert _signed_in(other)

        token = passkeys.reauth_token(user, "sessions_delete")
        assert _delete_other_sessions(current, {"reauth_token": token}) == 204
        assert _signed_in(current)
        assert not _signed_in(other)
        assert _delete_other_sessions(current, {"reauth_token": token}) == 400, "the token is single-use"
    finally:
        user.session = first
        passkeys.delete_passkey_account(user)
