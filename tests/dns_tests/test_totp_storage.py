"""End-to-end checks for how TOTP material is stored and consumed.

Drives the real API (enable, confirm, login, disable) and reads the account
document straight from MongoDB.
specRef: api-endpoint-behaviour #E2 #E4 #E5 #E6 #E7 #E8 #E12.
"""

import base64
import contextlib
import hashlib
import hmac
import struct
import time

import pytest
from bson import ObjectId
from moddns import ApiClient, Configuration
from moddns.api.account_api import AccountApi
from moddns.api.authentication_api import AuthenticationApi
from moddns.api.verification_api import VerificationApi
from moddns.exceptions import ApiException
from moddns.models.requests_confirm_reset_password_body import (
    RequestsConfirmResetPasswordBody,
)
from moddns.models.requests_login_body import RequestsLoginBody
from moddns.models.requests_reset_password_body import RequestsResetPasswordBody
from moddns.models.requests_totp_disable_req import RequestsTotpDisableReq
from moddns.models.requests_totp_req import RequestsTotpReq
from pymongo import MongoClient

from libs.accounts import create_account, delete_account, generate_complex_password
from libs.settings import get_settings

LEGACY_SECRET = "JBSWY3DPEHPK3PXPJBSWY3DPEHPK3PXP"
LEGACY_CODES = ["LegacyCodeAAAAA1", "LegacyCodeBBBBB2"]
CODE_ALPHABET = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"


def totp_code(secret: str, at: float | None = None) -> str:
    """RFC 6238 TOTP (SHA-1, 30 s step, 6 digits), as authenticator apps compute it."""
    key = base64.b32decode(secret + "=" * (-len(secret) % 8))
    counter = int((at if at is not None else time.time()) // 30)
    digest = hmac.new(key, struct.pack(">Q", counter), hashlib.sha1).digest()
    offset = digest[-1] & 0x0F
    value = struct.unpack(">I", digest[offset : offset + 4])[0] & 0x7FFFFFFF
    return f"{value % 1_000_000:06d}"


@pytest.fixture(scope="module")
def mongo_db():
    c = MongoClient(get_settings().MONGO_URI, serverSelectionTimeoutMS=5000)
    yield c[get_settings().MONGO_DB]
    c.close()


@pytest.fixture
def fresh_account():
    account, cookie, password, email = create_account()
    # A test that changes the password or session updates them here for cleanup.
    state = {"secret": None, "cookie": cookie, "password": password}
    yield account, cookie, password, email, state
    # Account deletion needs a second factor while 2FA is on.
    if state["secret"]:
        with _api(state["cookie"]) as api_client, contextlib.suppress(ApiException):
            AccountApi(api_client).api_v1_accounts_mfa_totp_disable_post(
                RequestsTotpDisableReq(otp=totp_code(state["secret"]))
            )
    delete_account(state["cookie"], state["password"], account_id=account.id)


def _api(cookie: str | None = None) -> ApiClient:
    api_client = ApiClient(Configuration(host=get_settings().DNS_API_ADDR))
    if cookie:
        api_client.default_headers["Cookie"] = cookie
    return api_client


def _enable_totp(cookie: str) -> tuple[str, list[str]]:
    with _api(cookie) as api_client:
        acc_api = AccountApi(api_client)
        new = acc_api.api_v1_accounts_mfa_totp_enable_post()
        backup = acc_api.api_v1_accounts_mfa_totp_enable_confirm_post(
            RequestsTotpReq(otp=totp_code(new.secret))
        )
    return new.secret, backup.backup_codes


def _login(email: str, password: str, otp: str) -> int:
    return _login_session(email, password, otp)[0]


def _login_session(email: str, password: str, otp: str) -> tuple[int, str | None]:
    with _api() as api_client:
        try:
            resp = AuthenticationApi(api_client).api_v1_login_post_with_http_info(
                RequestsLoginBody(email=email, password=password),
                x_mfa_code=otp,
                x_mfa_methods=["totp"],
            )
            return resp.status_code, resp.headers.get("Set-Cookie")
        except ApiException as e:
            return e.status, None


def _disable(cookie: str, otp: str) -> int:
    with _api(cookie) as api_client:
        try:
            return AccountApi(api_client).api_v1_accounts_mfa_totp_disable_post_with_http_info(
                RequestsTotpDisableReq(otp=otp)
            ).status_code
        except ApiException as e:
            return e.status


def _reset_token(db, account_id: str, email: str) -> str:
    """Requests a password reset and returns the token the email would carry."""
    # Reset emails go only to verified addresses.
    db.accounts.update_one({"_id": ObjectId(account_id)}, {"$set": {"email_verified": True}})
    with _api() as api_client:
        AccountApi(api_client).api_v1_accounts_reset_password_post(
            RequestsResetPasswordBody(email=email)
        )
    tokens = db.accounts.find_one({"_id": ObjectId(account_id)})["tokens"]
    return next(t["value"] for t in tokens if t["type"] == "password_reset")


def _confirm_reset(token: str, new_password: str, otp: str) -> int:
    with _api() as api_client:
        try:
            return VerificationApi(api_client).api_v1_verify_reset_password_post_with_http_info(
                RequestsConfirmResetPasswordBody(token=token, new_password=new_password),
                x_mfa_code=otp,
                x_mfa_methods=["totp"],
            ).status_code
        except ApiException as e:
            return e.status


def _grouped(code: str) -> str:
    """A backup code as the app displays it: blocks of five joined by hyphens."""
    return "-".join(code[i : i + 5] for i in range(0, len(code), 5))


def _totp_doc(db, account_id: str) -> dict:
    doc = db.accounts.find_one({"_id": ObjectId(account_id)})
    assert doc is not None
    return doc["mfa"]["totp"]


def _strings(value) -> list[str]:
    if isinstance(value, str):
        return [value]
    if isinstance(value, dict):
        return [s for v in value.values() for s in _strings(v)]
    if isinstance(value, list):
        return [s for v in value for s in _strings(v)]
    return []


class TestTotpStorage:
    def test_enable_stores_no_plaintext_secret_or_backup_code(
        self, mongo_db, fresh_account
    ):
        """specRef: E4, E5."""
        account, cookie, _, _, state = fresh_account
        secret, codes = _enable_totp(cookie)
        state["secret"] = secret

        assert len(codes) == 8
        assert all(len(c) == 25 and set(c) <= set(CODE_ALPHABET) for c in codes)
        stored = _strings(_totp_doc(mongo_db, account.id))
        for value in stored:
            assert secret not in value, "TOTP secret stored in plaintext"
            for code in codes:
                assert code not in value, "backup code stored in plaintext"

    def test_enable_accepts_separated_code(self, mongo_db, fresh_account):
        """Set-up takes the code as authenticator apps display it; a wrong code
        does not end the session. specRef: E2."""
        account, cookie, _, _, state = fresh_account
        with _api(cookie) as api_client:
            acc_api = AccountApi(api_client)
            new = acc_api.api_v1_accounts_mfa_totp_enable_post()
            state["secret"] = new.secret
            code = totp_code(new.secret)
            wrong = f"{(int(code) + 1) % 1_000_000:06d}"

            with pytest.raises(ApiException) as refused:
                acc_api.api_v1_accounts_mfa_totp_enable_confirm_post(RequestsTotpReq(otp=wrong))
            assert refused.value.status == 400
            assert _totp_doc(mongo_db, account.id).get("enabled") is not True

            acc_api.api_v1_accounts_mfa_totp_enable_confirm_post(
                RequestsTotpReq(otp=f" {code[:3]} - {code[3:]} ")
            )
        assert _totp_doc(mongo_db, account.id).get("enabled") is True

    def test_totp_and_backup_code_login_after_enable(self, mongo_db, fresh_account):
        """specRef: E4, E6."""
        _, cookie, password, email, state = fresh_account
        secret, codes = _enable_totp(cookie)
        state["secret"] = secret

        assert _login(email, password, totp_code(secret)) == 200
        assert _login(email, password, codes[0]) == 200
        # A backup code is single-use.
        assert _login(email, password, codes[0]) == 401
        # The app shows codes in groups of five; the separators are ignored.
        grouped = _grouped(codes[1])
        assert _login(email, password, grouped) == 200
        assert _login(email, password, codes[1]) == 401
        # Case is not part of the code.
        assert _login(email, password, codes[2].lower()) == 200

    def test_disable_removes_all_totp_material(self, mongo_db, fresh_account):
        """specRef: E7."""
        account, cookie, _, _, state = fresh_account
        secret, _ = _enable_totp(cookie)
        state["secret"] = secret

        with _api(cookie) as api_client:
            AccountApi(api_client).api_v1_accounts_mfa_totp_disable_post(
                RequestsTotpDisableReq(otp=totp_code(secret))
            )

        doc = _totp_doc(mongo_db, account.id)
        assert doc.get("enabled") is False
        assert _strings(doc) == [], f"TOTP material left after disable: {sorted(doc)}"

    def test_disable_with_backup_code(self, mongo_db, fresh_account):
        """A lost authenticator: a backup code turns 2FA off. specRef: E3, E6, E7."""
        account, cookie, password, email, state = fresh_account
        secret, codes = _enable_totp(cookie)
        state["secret"] = secret

        # A used code is refused without ending the session; 2FA stays on.
        assert _login(email, password, codes[0]) == 200
        assert _disable(cookie, codes[0]) == 400
        assert _totp_doc(mongo_db, account.id).get("enabled") is True

        # The grouped form the app shows, typed in lowercase.
        assert _disable(cookie, _grouped(codes[1]).lower()) == 200
        state["secret"] = None
        doc = _totp_doc(mongo_db, account.id)
        assert doc.get("enabled") is False
        assert _strings(doc) == [], f"TOTP material left after disable: {sorted(doc)}"

        # A new device can be enrolled afterwards.
        state["secret"], _ = _enable_totp(cookie)

    def test_password_reset_with_backup_code(self, mongo_db, fresh_account):
        """specRef: C2, E6."""
        account, cookie, _, email, state = fresh_account
        secret, codes = _enable_totp(cookie)
        state["secret"] = secret

        token = _reset_token(mongo_db, account.id, email)
        new_password = generate_complex_password()
        assert _confirm_reset(token, new_password, _grouped(codes[0])) == 204
        state["password"] = new_password

        # The code was used up by the reset.
        assert _login(email, new_password, codes[0]) == 401
        status, new_cookie = _login_session(email, new_password, codes[1])
        assert status == 200 and new_cookie
        state["cookie"] = new_cookie

    def _set_legacy_record(self, db, account_id: str, state: dict) -> None:
        db.accounts.update_one(
            {"_id": ObjectId(account_id)},
            {"$set": {"mfa.totp": {"enabled": True, "secret": LEGACY_SECRET, "backup_codes": list(LEGACY_CODES)}}},
        )
        state["secret"] = LEGACY_SECRET

    def test_password_reset_with_legacy_backup_code(self, mongo_db, fresh_account):
        """Codes issued before encrypted storage match only as typed. specRef: C2, E8."""
        account, _, _, email, state = fresh_account
        self._set_legacy_record(mongo_db, account.id, state)

        token = _reset_token(mongo_db, account.id, email)
        new_password = generate_complex_password()
        assert _confirm_reset(token, new_password, LEGACY_CODES[0].upper()) == 401
        assert _confirm_reset(token, new_password, LEGACY_CODES[0]) == 204
        state["password"] = new_password

        assert _login(email, new_password, LEGACY_CODES[0]) == 401
        status, new_cookie = _login_session(email, new_password, LEGACY_CODES[1])
        assert status == 200 and new_cookie
        state["cookie"] = new_cookie

    def test_disable_with_legacy_backup_code(self, mongo_db, fresh_account):
        """Codes issued before encrypted storage match only as typed. specRef: E3, E7, E8."""
        account, cookie, _, _, state = fresh_account
        self._set_legacy_record(mongo_db, account.id, state)

        assert _disable(cookie, LEGACY_CODES[0].lower()) == 400
        assert _totp_doc(mongo_db, account.id).get("enabled") is True

        assert _disable(cookie, LEGACY_CODES[0]) == 200
        state["secret"] = None
        doc = _totp_doc(mongo_db, account.id)
        assert doc.get("enabled") is False
        assert _strings(doc) == [], f"TOTP material left after disable: {sorted(doc)}"

    # The used list of a plaintext record may be null, empty or missing.
    @pytest.mark.parametrize(
        "used", [None, [], "missing"], ids=["null", "empty", "missing"]
    )
    def test_legacy_record_keeps_working(self, mongo_db, fresh_account, used):
        """Accounts enabled before encrypted storage log in unchanged. specRef: E8."""
        account, _, password, email, state = fresh_account
        legacy = {
            "enabled": True,
            "secret": LEGACY_SECRET,
            "backup_codes": list(LEGACY_CODES),
        }
        if used != "missing":
            legacy["backup_codes_used"] = used
        mongo_db.accounts.update_one(
            {"_id": ObjectId(account.id)}, {"$set": {"mfa.totp": legacy}}
        )
        state["secret"] = LEGACY_SECRET

        assert _login(email, password, totp_code(LEGACY_SECRET)) == 200
        assert _login(email, password, LEGACY_CODES[0]) == 200
        assert _login(email, password, LEGACY_CODES[0]) == 401
        doc = _totp_doc(mongo_db, account.id)
        assert LEGACY_CODES[0] not in doc.get("backup_codes", [])
        # Used codes are not recorded: the old list is left as it was.
        assert doc.get("backup_codes_used", "missing") == used

    def test_failed_attempts_lock_out_valid_codes(self, fresh_account, redis_client):
        """After ID_LIMITER_MAX (default 5) failed attempts every code is refused. specRef: E12."""
        account, cookie, password, email, state = fresh_account
        secret, codes = _enable_totp(cookie)
        state["secret"] = secret

        for _ in range(5):
            assert _login(email, password, "000000") == 401
        assert _login(email, password, totp_code(secret)) == 429
        assert _login(email, password, codes[0]) == 429

        # Lockout ends when the counter expires; clear it so cleanup can log in.
        redis_client.delete(f"totp_fails:{account.id}")
        assert _login(email, password, codes[0]) == 200, "refused code was not consumed"
