"""Passkey (WebAuthn) ceremonies against the running API, MongoDB and Redis.

Every browser response is produced by a software authenticator
(``libs/webauthn_authenticator.py``) and verified by the API's go-webauthn relying
party; no credential or token is written to the database. MongoDB and Redis are
only read, to check what the API stored.

tableRef: api-endpoint-behaviour #A4, #A5, #B3a, #F1-#F11
"""

import re
import time
from datetime import datetime, timedelta, timezone

import pytest

from libs.accounts import create_account, create_temp_subscription, delete_account, generate_complex_password, random_email
from libs.passkeys import (
    AUTH_COOKIE,
    CEREMONY_KEY_PREFIX,
    PA_SESSION_COOKIE,
    REAUTH_PURPOSES,
    TEMP_COOKIE,
    PasskeyUser,
    add_passkey,
    add_passkey_begin,
    add_passkey_finish,
    cookie_header,
    cookie_value,
    delete_passkey,
    delete_passkey_account,
    export_profiles,
    login,
    login_begin,
    login_finish,
    mongo,
    new_authenticator,
    raw_request,
    reauth,
    reauth_begin,
    reauth_finish,
    reauth_token,
    register_begin,
    register_finish,
    set_first_password,
    signup,
    stored_credential,
)
from libs.webauthn_authenticator import InvalidStateError
from moddns import RequestsLoginBody
import moddns.api as api
import moddns.api_client as client
import moddns.configuration as api_config
from libs.settings import get_settings


@pytest.fixture
def passkey_user():
    """A passkey-only account, signed in; deleted through a passkey reauth on teardown."""
    user = signup()
    yield user
    delete_passkey_account(user)


@pytest.fixture
def second_passkey_user():
    user = signup()
    yield user
    delete_passkey_account(user)


def parse_rfc3339(value: str) -> datetime:
    """Parse Go's RFC 3339 timestamps, whose nanosecond fractions Python 3.10 rejects."""
    value = re.sub(r"\.(\d+)", lambda m: "." + (m.group(1) + "000000")[:6], value.replace("Z", "+00:00"))
    return datetime.fromisoformat(value)


def ceremony_keys(redis_client) -> set[bytes]:
    return set(redis_client.scan_iter(match=f"{CEREMONY_KEY_PREFIX}*"))


def current_account(session_cookie: str):
    conf = api_config.Configuration(host=get_settings().DNS_API_ADDR)
    with client.ApiClient(conf) as api_client:
        acc = api.AccountApi(api_client)
        acc.api_client.default_headers["Cookie"] = session_cookie
        return acc.api_v1_accounts_current_get_with_http_info()


# ----------------------------------------------------------------------
# Signup and login
# ----------------------------------------------------------------------
def test_passkey_signup_stores_the_credential(passkey_user: PasskeyUser):
    """Signup verifies the attestation and stores one credential for the account.

    tableRef: api-endpoint-behaviour #F1, #F2
    """
    stored = stored_credential(passkey_user.credential_id)
    assert stored is not None
    assert stored["flags"]["user_present"] is True
    assert stored["flags"]["user_verified"] is True
    assert mongo().credentials.count_documents({"account_id": stored["account_id"]}) == 1


def test_passkey_signup_opens_no_session():
    """The signup finish call answers 201 without a login session; sign-in is a separate ceremony.

    tableRef: api-endpoint-behaviour #F2
    """

    authenticator = new_authenticator()
    email = random_email("passkey")
    sub_id, pa_set_cookie = create_temp_subscription()
    pa_cookie = cookie_value(pa_set_cookie, PA_SESSION_COOKIE)
    begin = register_begin(email, sub_id, pa_cookie)
    assert begin.status == 200
    assert begin.cookie(TEMP_COOKIE)
    credential_id, credential = authenticator.create(begin.body)

    finish = register_finish(begin.cookie(TEMP_COOKIE), pa_cookie, credential)
    assert finish.status == 201, finish.body
    assert finish.cookie(AUTH_COOKIE) is None

    user = PasskeyUser(email=email, authenticator=authenticator, credential_ids=[credential_id])
    try:
        account_id = stored_credential(credential_id)["account_id"]
        assert mongo().sessions.count_documents({"account_id": str(account_id)}) == 0
    finally:
        delete_passkey_account(user)


def test_passkey_login_opens_a_session_and_updates_the_counter(passkey_user: PasskeyUser):
    """Login verifies the assertion, issues an auth cookie and stores the new sign count.

    tableRef: api-endpoint-behaviour #A4, #A5
    """
    begin = login_begin(passkey_user.email)
    assert begin.status == 200
    allowed = begin.body["publicKey"]["allowCredentials"]
    assert len(allowed) == 1

    assertion = passkey_user.authenticator.get(begin.body, passkey_user.credential_id, sign_count=41)
    finish = login_finish(begin.cookie(TEMP_COOKIE), assertion)
    assert finish.status == 201, finish.body
    session = finish.cookie(AUTH_COOKIE)
    assert session

    me = current_account(cookie_header(**{AUTH_COOKIE: session}))
    assert me.status_code == 200
    assert me.data.email == passkey_user.email
    assert stored_credential(passkey_user.credential_id)["authenticator"]["sign_count"] == 41


def test_passkey_login_with_a_credential_the_account_does_not_hold(passkey_user: PasskeyUser):
    """tableRef: api-endpoint-behaviour #A5"""
    known = passkey_user.authenticator.credentials[passkey_user.credential_id]
    other = new_authenticator()
    other_cred_id = other.add_credential(known.rp_id, known.user_handle)
    begin = login_begin(passkey_user.email)
    assertion = other.get(begin.body, other_cred_id)

    finish = login_finish(begin.cookie(TEMP_COOKIE), assertion)
    assert finish.status >= 400
    assert finish.cookie(AUTH_COOKIE) is None


def test_passkey_login_begin_for_an_account_without_passkeys():
    """tableRef: api-endpoint-behaviour #A4"""
    account, cookie, password, email = create_account()
    try:
        begin = login_begin(email)
        assert begin.status == 400
        assert begin.cookie(TEMP_COOKIE) is None
    finally:
        delete_account(cookie, password, account_id=account.id)


def test_passkey_login_refuses_a_sign_count_that_does_not_increase(passkey_user: PasskeyUser):
    """A counter at or below the stored one fails sign-in and leaves the stored count as it was.

    W3C WebAuthn L3 §6.1.1. tableRef: api-endpoint-behaviour #A5
    """
    assert login(passkey_user, sign_count=10).status == 201
    resp = login(passkey_user, sign_count=3)
    assert resp.status >= 400
    assert resp.cookie(AUTH_COOKIE) is None
    assert stored_credential(passkey_user.credential_id)["authenticator"]["sign_count"] == 10
    passkey_user.authenticator.credentials[passkey_user.credential_id].sign_count = 10


# ----------------------------------------------------------------------
# Ceremony state
# ----------------------------------------------------------------------
def test_ceremony_state_is_kept_in_redis_only(passkey_user: PasskeyUser, redis_client):
    """tableRef: api-endpoint-behaviour #F9"""
    sessions_before = mongo().sessions.count_documents({})
    begin = login_begin(passkey_user.email)
    key = f"{CEREMONY_KEY_PREFIX}{begin.cookie(TEMP_COOKIE)}"

    assert redis_client.exists(key)
    assert 0 < redis_client.ttl(key) <= 300
    assert mongo().sessions.count_documents({}) == sessions_before


def test_ceremony_state_is_single_use(passkey_user: PasskeyUser, redis_client):
    """tableRef: api-endpoint-behaviour #F9"""
    begin = login_begin(passkey_user.email)
    temp = begin.cookie(TEMP_COOKIE)
    assertion = passkey_user.authenticator.get(begin.body, passkey_user.credential_id)

    assert login_finish(temp, assertion).status == 201
    assert not redis_client.exists(f"{CEREMONY_KEY_PREFIX}{temp}")
    replay = login_finish(temp, assertion)
    assert replay.status >= 400
    assert replay.cookie(AUTH_COOKIE) is None


def test_reauth_state_is_refused_by_login_finish(passkey_user: PasskeyUser, redis_client):
    """State begun by another ceremony type is refused, and consumed.

    tableRef: api-endpoint-behaviour #F10
    """
    begin = reauth_begin(passkey_user.session_cookie, "email_change")
    temp = begin.cookie(TEMP_COOKIE)
    assertion = passkey_user.authenticator.get(begin.body, passkey_user.credential_id)

    finish = login_finish(temp, assertion)
    assert finish.status == 400
    assert finish.cookie(AUTH_COOKIE) is None
    assert not redis_client.exists(f"{CEREMONY_KEY_PREFIX}{temp}")


def test_reauth_state_is_refused_for_another_account(passkey_user: PasskeyUser, second_passkey_user: PasskeyUser):
    """tableRef: api-endpoint-behaviour #F10"""
    begin = reauth_begin(passkey_user.session_cookie, "email_change")
    assertion = passkey_user.authenticator.get(begin.body, passkey_user.credential_id)

    finish = reauth_finish(second_passkey_user.session_cookie, begin.cookie(TEMP_COOKIE), assertion)
    assert finish.status == 400
    assert not (finish.body or {}).get("reauth_token")


@pytest.mark.xfail(
    strict=True,
    reason="An expired ceremony answers 500; spec row F11 states 400.",
)
def test_expired_ceremony_state_is_refused_with_400(passkey_user: PasskeyUser, redis_client):
    """tableRef: api-endpoint-behaviour #F11"""
    begin = login_begin(passkey_user.email)
    temp = begin.cookie(TEMP_COOKIE)
    assertion = passkey_user.authenticator.get(begin.body, passkey_user.credential_id)
    redis_client.pexpire(f"{CEREMONY_KEY_PREFIX}{temp}", 1)
    time.sleep(0.05)

    assert login_finish(temp, assertion).status == 400


# ----------------------------------------------------------------------
# Reauth
# ----------------------------------------------------------------------
@pytest.mark.parametrize("purpose", REAUTH_PURPOSES)
def test_passkey_reauth_issues_a_token_for_each_purpose(passkey_user: PasskeyUser, purpose: str):
    """tableRef: api-endpoint-behaviour #F5, #F6"""
    resp = reauth(passkey_user, purpose)
    assert resp.status == 201, resp.body
    assert resp.body["reauth_token"]
    expires_at = parse_rfc3339(resp.body["expires_at"])
    assert abs(expires_at - (datetime.now(timezone.utc) + timedelta(minutes=5))) < timedelta(seconds=30)


def test_reauth_token_is_accepted_once_by_its_operation(passkey_user: PasskeyUser):
    """tableRef: api-endpoint-behaviour #F6; account-export-import-behaviour #E3, #M4"""
    token = reauth_token(passkey_user, "profile_export")

    assert export_profiles(passkey_user.session_cookie, reauth_token=token).status == 200
    assert export_profiles(passkey_user.session_cookie, reauth_token=token).status >= 400


def test_reauth_token_of_another_purpose_is_refused(passkey_user: PasskeyUser):
    """tableRef: api-endpoint-behaviour #F6; account-export-import-behaviour #M4"""
    token = reauth_token(passkey_user, "email_change")

    assert export_profiles(passkey_user.session_cookie, reauth_token=token).status >= 400


def test_reauth_begin_refuses_an_unknown_purpose(passkey_user: PasskeyUser, redis_client):
    """The server refuses a purpose outside the list (the generated client checks it too, so this is sent raw).

    tableRef: api-endpoint-behaviour #F5
    """
    before = ceremony_keys(redis_client)
    resp = raw_request("POST", "/api/v1/webauthn/passkey/reauth/begin", passkey_user.session_cookie,
                       {"purpose": "unknown_purpose"})

    assert resp.status == 400
    assert ceremony_keys(redis_client) == before


# ----------------------------------------------------------------------
# Adding and deleting passkeys
# ----------------------------------------------------------------------
def test_add_passkey_needs_verification(passkey_user: PasskeyUser, redis_client):
    """tableRef: api-endpoint-behaviour #F3"""
    before = ceremony_keys(redis_client)
    resp = add_passkey_begin(passkey_user.session_cookie)

    assert resp.status == 400
    assert ceremony_keys(redis_client) == before


def test_add_passkey_with_a_reauth_token(passkey_user: PasskeyUser):
    """The options exclude the account's credentials; a new one is added and signs in.

    tableRef: api-endpoint-behaviour #F3, #F3b, #F4
    """
    token = reauth_token(passkey_user, "passkey_add")
    begin = add_passkey_begin(passkey_user.session_cookie, reauth_token=token)
    assert begin.status == 200, begin.body
    assert len(begin.body["publicKey"]["excludeCredentials"]) == 1
    with pytest.raises(InvalidStateError):
        passkey_user.authenticator.create(begin.body)

    second = new_authenticator()
    second_id, credential = second.create(begin.body)
    finish = add_passkey_finish(passkey_user.session_cookie, begin.cookie(TEMP_COOKIE), credential)
    assert finish.status == 201, finish.body

    passkey_user.authenticator.credentials.update(second.credentials)
    passkey_user.credential_ids.append(second_id)
    assert login(passkey_user, second_id).status == 201
    assert add_passkey_begin(passkey_user.session_cookie, reauth_token=token).status == 400, "the token is single-use"


def test_add_passkey_with_the_current_password():
    """tableRef: api-endpoint-behaviour #F3, #F4"""
    account, cookie, password, email = create_account()
    try:
        user = PasskeyUser(email=email, authenticator=new_authenticator(), session=cookie_value(cookie, AUTH_COOKIE))
        credential_id = add_passkey(user, current_password=password)

        assert stored_credential(credential_id)["account_id"] is not None
        assert login(user, credential_id).status == 201
    finally:
        delete_account(cookie, password, account_id=account.id)


def test_delete_passkey_with_a_reauth_token(passkey_user: PasskeyUser):
    """tableRef: api-endpoint-behaviour #F8"""
    first = passkey_user.credential_id
    second = add_passkey_on_new_authenticator(passkey_user)

    token = reauth_token(passkey_user, "passkey_delete")
    resp = delete_passkey(passkey_user.session_cookie, first, reauth_token=token)
    assert resp.status == 204, resp.body
    assert stored_credential(first) is None

    begin = login_begin(passkey_user.email)
    assert len(begin.body["publicKey"]["allowCredentials"]) == 1
    passkey_user.credential_ids.remove(first)
    assert login(passkey_user, second).status == 201


def add_passkey_on_new_authenticator(user: PasskeyUser) -> bytes:
    token = reauth_token(user, "passkey_add")
    begin = add_passkey_begin(user.session_cookie, reauth_token=token)
    other = new_authenticator()
    credential_id, credential = other.create(begin.body)
    assert add_passkey_finish(user.session_cookie, begin.cookie(TEMP_COOKIE), credential).status == 201
    user.authenticator.credentials.update(other.credentials)
    user.credential_ids.append(credential_id)
    return credential_id


def test_the_last_passkey_of_an_account_without_password_cannot_be_deleted(passkey_user: PasskeyUser):
    """tableRef: api-endpoint-behaviour #F8a"""
    token = reauth_token(passkey_user, "passkey_delete")

    resp = delete_passkey(passkey_user.session_cookie, passkey_user.credential_id, reauth_token=token)
    assert resp.status == 400
    assert "only way to sign in" in resp.body["error"]
    assert stored_credential(passkey_user.credential_id) is not None
    second = add_passkey_on_new_authenticator(passkey_user)
    assert delete_passkey(passkey_user.session_cookie, second, reauth_token=token).status == 204, (
        "the refused deletion must not consume the token"
    )


def test_another_accounts_passkey_cannot_be_deleted(passkey_user: PasskeyUser, second_passkey_user: PasskeyUser):
    """Another account's passkey is refused exactly like one that does not exist, and is kept.

    tableRef: api-endpoint-behaviour #F8
    """
    add_passkey_on_new_authenticator(passkey_user)
    add_passkey_on_new_authenticator(second_passkey_user)
    token = reauth_token(second_passkey_user, "passkey_delete")

    foreign = delete_passkey(second_passkey_user.session_cookie, passkey_user.credential_id, reauth_token=token)
    missing = delete_passkey(second_passkey_user.session_cookie, b"no-such-credential", reauth_token=token)

    assert foreign.status >= 400
    assert (foreign.status, foreign.body) == (missing.status, missing.body)
    assert stored_credential(passkey_user.credential_id) is not None


# ----------------------------------------------------------------------
# First password on a passkey-only account
# ----------------------------------------------------------------------
def test_passkey_only_account_sets_a_first_password(passkey_user: PasskeyUser):
    """A session alone cannot set a password; a password_set reauth token can.

    tableRef: api-endpoint-behaviour #B3a
    """
    password = generate_complex_password()
    assert set_first_password(passkey_user.session_cookie, password, reauth_token=None).status >= 400

    token = reauth_token(passkey_user, "password_set")
    resp = set_first_password(passkey_user.session_cookie, password, reauth_token=token)
    assert resp.status in (200, 204), resp.body

    conf = api_config.Configuration(host=get_settings().DNS_API_ADDR)
    with client.ApiClient(conf) as api_client:
        login_resp = api.AuthenticationApi(api_client).api_v1_login_post_with_http_info(
            body=RequestsLoginBody(email=passkey_user.email, password=password)
        )
        assert login_resp.status_code == 200


# ----------------------------------------------------------------------
# User verification (current policy: preferred)
# ----------------------------------------------------------------------
def test_user_verification_is_preferred():
    """Ceremonies use userVerification "preferred" (W3C WebAuthn L3 §5.8.6): a credential
    and an assertion with UV=0 are accepted.

    tableRef: api-endpoint-behaviour #F2, #A5
    """
    user = signup(new_authenticator(user_verified=False))
    try:
        assert stored_credential(user.credential_id)["flags"]["user_verified"] is False
        assert login(user).status == 201
    finally:
        delete_passkey_account(user)
