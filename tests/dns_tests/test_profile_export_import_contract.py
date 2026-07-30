"""HTTP-contract backend E2E tests for profile export/import endpoints.

Covers Sections E, I, V, M, S of docs/specs/account-export-import-behaviour.md.
These tests assert only HTTP-level behaviour (status codes, headers, response
envelope shape, validation rejections, reauth and CSRF guards). End-to-end
filtering and DNS resolution are not exercised here.
"""

import re
from datetime import datetime

import moddns.api as api
import moddns.api_client as client
import moddns.configuration as api_config
from moddns import RequestsLoginBody, ApiCreateProfileBody

from libs.settings import get_settings
from libs.export_import_helpers import (
    build_envelope,
    build_profile,
    create_account_with_password,
    do_export,
    do_import,
    export_request_body,
    import_request_body,
    raw_export,
    raw_import,
)


class TestExportEndpoint:
    def setup_class(self):
        self.config = get_settings()
        self.api_config = api_config.Configuration(host=self.config.DNS_API_ADDR)

    def test_export_all_returns_envelope_with_defaults(self):
        """Export with scope=all returns envelope + correct response headers. specRef: E1, E5, E12-E15, V1-V4."""
        _, cookie, password, _ = create_account_with_password()
        resp = do_export(cookie, password, scope="all")
        assert resp.status_code == 200, resp.text

        ctype = resp.headers.get("Content-Type", "")
        assert ctype.startswith("application/vnd.moddns.export+json"), ctype
        cdisp = resp.headers.get("Content-Disposition", "")
        assert cdisp.startswith('attachment; filename="moddns-export-'), cdisp
        assert cdisp.endswith('.moddns.json"'), cdisp
        assert resp.headers.get("Cache-Control") == "no-store"
        assert resp.headers.get("Pragma") == "no-cache"

        body = resp.json()
        assert body["schemaVersion"] == 1
        assert body["kind"] == "moddns-export"
        exported_at = body["exportedAt"]
        truncated = re.sub(r"(\.\d{6})\d+", r"\1", exported_at).replace("Z", "+00:00")
        datetime.fromisoformat(truncated)
        assert body["exportedFrom"]["service"] == "modDNS"
        assert len(body["profiles"]) == 1

    def test_export_selected_owned_profiles(self):
        """scope=selected with owned profile_id returns just that profile. specRef: E7."""
        _, cookie, password, _ = create_account_with_password()
        new_name = "extra_export_profile"
        with client.ApiClient(self.api_config) as api_client:
            profiles_api = api.ProfileApi(api_client)
            profiles_api.api_client.default_headers["Cookie"] = cookie
            create_resp = profiles_api.api_v1_profiles_post_with_http_info(
                body=ApiCreateProfileBody(name=new_name)
            )
            assert create_resp.status_code == 201, create_resp
            new_profile_id = create_resp.data.profile_id

        resp = do_export(cookie, password, scope="selected", profile_ids=[new_profile_id])
        assert resp.status_code == 200, resp.text
        body = resp.json()
        assert len(body["profiles"]) == 1
        assert body["profiles"][0]["name"] == new_name

    def test_export_scope_all_with_profile_ids_rejected(self):
        """scope=all with non-empty profile_ids is rejected. specRef: E6."""
        _, cookie, password, _ = create_account_with_password()
        resp = raw_export(
            cookie,
            export_request_body(scope="all", profile_ids=["x"], current_password=password),
        )
        assert resp.status_code == 400, resp.text

    def test_export_scope_selected_without_ids_rejected(self):
        """scope=selected without profile_ids is rejected. specRef: E8."""
        _, cookie, password, _ = create_account_with_password()
        resp = raw_export(
            cookie,
            export_request_body(scope="selected", current_password=password),
        )
        assert resp.status_code == 400, resp.text

    def test_export_selected_foreign_profile_id_returns_404(self):
        """Exporting another user's profile id returns 404. specRef: E9, S3."""
        _, cookie_a, password_a, _ = create_account_with_password()
        account_b, _, _, _ = create_account_with_password()
        foreign_profile_id = account_b.profiles[0]
        resp = raw_export(
            cookie_a,
            export_request_body(
                scope="selected",
                profile_ids=[foreign_profile_id],
                current_password=password_a,
            ),
        )
        assert resp.status_code == 404, resp.text

    def test_export_unknown_scope_rejected(self):
        """Unknown scope value is rejected. specRef: E11."""
        _, cookie, password, _ = create_account_with_password()
        resp = raw_export(cookie, {"scope": "weird", "current_password": password})
        assert resp.status_code == 400, resp.text

    def test_export_without_reauth_returns_400(self):
        """Missing reauth credentials are rejected with 400 (not 401, to avoid client logout flow). specRef: E3, M5."""
        _, cookie, _, _ = create_account_with_password()
        resp = raw_export(cookie, {"scope": "all"})
        assert resp.status_code == 400, resp.text

    def test_export_with_wrong_password_returns_400(self):
        """Wrong current_password is rejected with 400 (not 401, to avoid client logout flow). specRef: M6."""
        _, cookie, _, _ = create_account_with_password()
        resp = raw_export(
            cookie,
            export_request_body(scope="all", current_password="WrongPassw0rd!"),
        )
        assert resp.status_code == 400, resp.text


class TestImportEndpoint:
    def setup_class(self):
        self.config = get_settings()
        self.api_config = api_config.Configuration(host=self.config.DNS_API_ADDR)

    def test_import_create_new_minimal_envelope(self):
        """Minimal envelope with one profile succeeds and returns ids+names. specRef: I1, I8, I19, I19b, I20."""
        _, cookie, password, _ = create_account_with_password()
        env = build_envelope(profiles=[build_profile(name="Imported")])
        resp = do_import(cookie, password, env)
        assert resp.status_code == 200, resp.text
        body = resp.json()
        assert isinstance(body["createdProfileIds"], list)
        assert len(body["createdProfileIds"]) == 1
        assert all(isinstance(i, str) for i in body["createdProfileIds"])
        assert body["createdProfileNames"] == ["Imported"]
        assert body["warnings"] == []

    def test_import_missing_csrf_header_returns_400(self):
        """Missing X-modDNS-Import header is rejected. specRef: I4, S7."""
        _, cookie, password, _ = create_account_with_password()
        env = build_envelope()
        resp = do_import(cookie, password, env, include_csrf=False)
        assert resp.status_code == 400, resp.text

    def test_import_wrong_csrf_header_value_returns_400(self):
        """Wrong X-modDNS-Import header value is rejected. specRef: I4."""
        _, cookie, password, _ = create_account_with_password()
        env = build_envelope()
        resp = do_import(
            cookie, password, env, include_csrf=False, headers={"X-modDNS-Import": "yes"}
        )
        assert resp.status_code == 400, resp.text

    def test_import_gzip_content_encoding_returns_415(self):
        """Content-Encoding: gzip is rejected with 415. specRef: I6, S8."""
        _, cookie, password, _ = create_account_with_password()
        env = build_envelope()
        resp = do_import(cookie, password, env, headers={"Content-Encoding": "gzip"})
        assert resp.status_code == 415, resp.text
        assert "gzip" in resp.text.lower()

    def test_import_form_content_type_returns_415(self):
        """application/x-www-form-urlencoded Content-Type is rejected with 415. specRef: I7."""
        _, cookie, password, _ = create_account_with_password()
        body = import_request_body(
            mode="create_new", payload=build_envelope(), current_password=password
        )
        resp = raw_import(
            cookie,
            body,
            headers={"Content-Type": "application/x-www-form-urlencoded"},
        )
        assert resp.status_code == 415, resp.text

    def test_import_unsupported_mode_replace_rejected(self):
        """mode=replace is rejected. specRef: I9."""
        _, cookie, password, _ = create_account_with_password()
        body = {
            "mode": "replace",
            "payload": build_envelope(),
            "current_password": password,
        }
        resp = raw_import(cookie, body)
        assert resp.status_code == 400, resp.text

    def test_import_unknown_mode_rejected(self):
        """Unknown mode value is rejected. specRef: I10."""
        _, cookie, password, _ = create_account_with_password()
        body = {
            "mode": "merge",
            "payload": build_envelope(),
            "current_password": password,
        }
        resp = raw_import(cookie, body)
        assert resp.status_code == 400, resp.text

    def test_import_mode_absent_rejected(self):
        """Missing mode field is rejected. specRef: I11."""
        _, cookie, password, _ = create_account_with_password()
        body = {"payload": build_envelope(), "current_password": password}
        resp = raw_import(cookie, body)
        assert resp.status_code == 400, resp.text

    def test_import_without_reauth_returns_400(self):
        """Missing reauth credentials are rejected with 400 (not 401, to avoid client logout flow). specRef: I2, M5."""
        _, cookie, _, _ = create_account_with_password()
        body = {"mode": "create_new", "payload": build_envelope()}
        resp = raw_import(cookie, body)
        assert resp.status_code == 400, resp.text

    def test_import_wrong_password_returns_400(self):
        """Wrong current_password is rejected with 400 (not 401, to avoid client logout flow). specRef: M6."""
        _, cookie, _, _ = create_account_with_password()
        body = import_request_body(
            mode="create_new",
            payload=build_envelope(),
            current_password="WrongPassw0rd!",
        )
        resp = raw_import(cookie, body)
        assert resp.status_code == 400, resp.text

    def test_import_unknown_root_field_rejected(self):
        """Unknown root field in envelope is rejected. specRef: V6, S2."""
        _, cookie, password, _ = create_account_with_password()
        env = build_envelope(extra={"email": "attacker@example.com"})
        resp = do_import(cookie, password, env)
        assert resp.status_code == 400, resp.text

    def test_import_unknown_per_profile_field_rejected(self):
        """Unknown per-profile field is rejected. specRef: V15, S2."""
        _, cookie, password, _ = create_account_with_password()
        env = build_envelope(profiles=[build_profile(extra={"accountId": "x"})])
        resp = do_import(cookie, password, env)
        assert resp.status_code == 400, resp.text

    def test_import_account_level_fields_rejected_account_unchanged(self):
        """Account-level fields are rejected and the account remains usable with original password. specRef: S2."""
        _, cookie, password, email = create_account_with_password()
        env = build_envelope(
            extra={"password": "newpw", "mfa": {}, "tokens": []}
        )
        resp = do_import(cookie, password, env)
        assert resp.status_code == 400, resp.text

        with client.ApiClient(self.api_config) as api_client:
            auth_api = api.AuthenticationApi(api_client)
            login_resp = auth_api.api_v1_login_post_with_http_info(
                body=RequestsLoginBody(email=email, password=password)
            )
            assert login_resp.status_code == 200

    def test_import_wrong_schema_version_rejected(self):
        """schemaVersion != 1 is rejected. specRef: V1."""
        _, cookie, password, _ = create_account_with_password()
        env = build_envelope(schema_version=2)
        resp = do_import(cookie, password, env)
        assert resp.status_code == 400, resp.text

    def test_import_wrong_kind_rejected(self):
        """kind != moddns-export is rejected. specRef: V2."""
        _, cookie, password, _ = create_account_with_password()
        env = build_envelope(kind="other")
        resp = do_import(cookie, password, env)
        assert resp.status_code == 400, resp.text

    def test_import_empty_profiles_array_rejected(self):
        """Empty profiles array is rejected. specRef: V5."""
        _, cookie, password, _ = create_account_with_password()
        env = build_envelope(profiles=[])
        resp = do_import(cookie, password, env)
        assert resp.status_code == 400, resp.text
