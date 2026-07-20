"""End-to-end behaviour tests for profile export/import.

Covers Sections F (file format), I (import endpoint), V (validation), and S
(security mitigations) of docs/specs/account-export-import-behaviour.md.
Exercises round-trip DNS filtering fidelity, name-collision resolution,
catalog degradation, and the profile-count cap.
"""

import pytest
from libs.dns_lib import DNSLib, assert_blocked, is_blocked
from libs.settings import get_settings
from libs.profile_helpers import (
    ProfileHelpers,
    SVC_GOOGLE_DOMAIN,
    SVC_GOOGLE_ID,
    TEST_BLOCKLIST_ID,
)
from libs.export_import_helpers import (
    EXPORTED_CUSTOM_RULES_LIMIT,
    add_custom_rules_batch,
    build_envelope,
    build_profile,
    create_account_with_password,
    do_export,
    do_import,
    export_request_body,
    make_rules,
    raw_export,
)
from libs.constants import BLOCKLISTED_DOMAIN
from dns.rdatatype import A

import moddns.api as api
import moddns.api_client as client
import moddns.configuration as api_config
from moddns import (
    ApiCreateProfileBody,
    RequestsProfileUpdates,
    ModelProfileUpdate,
)


CUSTOM_RULE_DOMAIN = "ads.example.test"
PUNYCODE_RULE = "xn--80ak6aa92e.com"


def _get_profile(api_config_, cookie, profile_id):
    with client.ApiClient(api_config_) as api_client:
        p = api.ProfileApi(api_client)
        p.api_client.default_headers["Cookie"] = cookie
        resp = p.api_v1_profiles_id_get_with_http_info(id=profile_id)
        assert resp.status_code == 200, resp
        return resp.data


def _rename_profile(api_config_, cookie, profile_id, new_name):
    with client.ApiClient(api_config_) as api_client:
        p = api.ProfileApi(api_client)
        p.api_client.default_headers["Cookie"] = cookie
        body = RequestsProfileUpdates(
            updates=[
                ModelProfileUpdate(
                    operation="replace",
                    path="/name",
                    value={"value": new_name},
                )
            ]
        )
        resp = p.api_v1_profiles_id_patch_with_http_info(profile_id, body=body)
        assert resp.status_code == 200, (
            f"Profile rename to '{new_name}' failed: {resp.status_code} {resp.data}"
        )


class TestRoundTrip(ProfileHelpers):
    def setup_class(self):
        self.config = get_settings()
        self.api_config = api_config.Configuration(host=self.config.DNS_API_ADDR)
        self.dns_lib = DNSLib(self.config.DOH_ENDPOINT)

    @pytest.mark.asyncio
    async def test_export_then_import_preserves_dns_filtering(
        self, ensure_test_blocklisted
    ):
        """Round-trip preserves blocklist, service, custom-rule and DNSSEC behaviour. specRef: F1-F6, S3."""
        account_a, cookie_a, password_a, _ = create_account_with_password()
        profile_id_a = account_a.profiles[0]

        with client.ApiClient(self.api_config) as api_client:
            p = api.ProfileApi(api_client)
            p.api_client.default_headers["Cookie"] = cookie_a
            profile_a = p.api_v1_profiles_id_get(id=profile_id_a)
            assert TEST_BLOCKLIST_ID in (profile_a.settings.privacy.blocklists or []), (
                f"Default profile must have {TEST_BLOCKLIST_ID} enabled for round-trip; "
                f"got {profile_a.settings.privacy.blocklists}"
            )
            self._block_service(p, profile_id_a, [SVC_GOOGLE_ID])
            self._create_custom_rule(p, profile_id_a, "block", CUSTOM_RULE_DOMAIN)

        export_resp = do_export(cookie_a, password_a, scope="all")
        assert export_resp.status_code == 200, export_resp.text
        envelope = export_resp.json()
        assert len(envelope["profiles"]) == 1

        envelope["profiles"][0]["name"] = "RestoredProfile"

        _, cookie_b, password_b, _ = create_account_with_password()
        import_resp = do_import(cookie_b, password_b, envelope)
        assert import_resp.status_code == 200, import_resp.text
        body = import_resp.json()
        new_profile_id = body["createdProfileIds"][0]
        assert isinstance(new_profile_id, str) and new_profile_id

        resp = await self.dns_lib.wait_until(
            new_profile_id, BLOCKLISTED_DOMAIN, A, is_blocked
        )
        assert_blocked(resp, f"{BLOCKLISTED_DOMAIN} (imported blocklist)")

        resp = await self.dns_lib.wait_until(
            new_profile_id, SVC_GOOGLE_DOMAIN, A, is_blocked
        )
        assert_blocked(resp, f"{SVC_GOOGLE_DOMAIN} (imported service block)")

        resp = await self.dns_lib.wait_until(
            new_profile_id, CUSTOM_RULE_DOMAIN, A, is_blocked
        )
        assert_blocked(resp, f"{CUSTOM_RULE_DOMAIN} (imported custom rule)")

        imported = _get_profile(self.api_config, cookie_b, new_profile_id)
        assert imported.settings.security.dnssec.enabled is True, (
            "DNSSEC enabled flag did not round-trip through import"
        )

    def test_round_trip_regenerates_internal_ids(self):
        """Imported profile gets a fresh server-generated id; source profile is untouched. specRef: F9, S3."""
        account_a, cookie_a, password_a, _ = create_account_with_password()
        source_profile_id = account_a.profiles[0]

        export_resp = do_export(cookie_a, password_a, scope="all")
        assert export_resp.status_code == 200, export_resp.text
        envelope = export_resp.json()
        envelope["profiles"][0]["name"] = "RegenIdTarget"

        _, cookie_b, password_b, _ = create_account_with_password()
        import_resp = do_import(cookie_b, password_b, envelope)
        assert import_resp.status_code == 200, import_resp.text
        new_profile_id = import_resp.json()["createdProfileIds"][0]

        assert str(new_profile_id) != str(source_profile_id), (
            f"Internal profile id was not regenerated; "
            f"source={source_profile_id} imported={new_profile_id}"
        )

        with client.ApiClient(self.api_config) as api_client:
            p = api.ProfileApi(api_client)
            p.api_client.default_headers["Cookie"] = cookie_a
            resp = p.api_v1_profiles_id_get_with_http_info(id=source_profile_id)
            assert resp.status_code == 200, (
                f"Source profile {source_profile_id} disappeared after import; "
                f"status={resp.status_code}"
            )

    def test_advanced_recursor_not_exported(self):
        """Export envelope must not carry the staging-only advanced.recursor value. specRef: F7."""
        _, cookie, password, _ = create_account_with_password()
        export_resp = do_export(cookie, password, scope="all")
        assert export_resp.status_code == 200, export_resp.text
        envelope = export_resp.json()
        assert envelope["profiles"], "Exported envelope has no profiles"

        for prof in envelope["profiles"]:
            settings = prof.get("settings") or {}
            advanced = settings.get("advanced")
            if advanced is not None:
                assert "recursor" not in advanced, (
                    f"Export leaked advanced.recursor: {advanced}"
                )

    def test_import_silently_ignores_advanced_recursor(self):
        """Hand-edited advanced.recursor in the payload is discarded; imported profile gets the default. specRef: F7."""
        _, cookie, password, _ = create_account_with_password()
        env = build_envelope(
            profiles=[
                build_profile(
                    name="RecursorIgnored",
                    advanced={"recursor": "unbound"},
                )
            ]
        )
        resp = do_import(cookie, password, env)
        assert resp.status_code == 200, resp.text
        new_profile_id = resp.json()["createdProfileIds"][0]

        imported = _get_profile(self.api_config, cookie, new_profile_id)
        recursor = imported.settings.advanced.recursor
        assert recursor == "knot", (
            f"advanced.recursor was carried through import; expected 'knot' got {recursor!r}"
        )


class TestNameCollision:
    def setup_class(self):
        self.config = get_settings()
        self.api_config = api_config.Configuration(host=self.config.DNS_API_ADDR)

    def test_import_collides_with_existing_name_appends_suffix(self):
        """Imported profile name matching an existing one is renamed with (imported) suffix. specRef: I24."""
        account, cookie, password, _ = create_account_with_password()
        existing_profile_id = account.profiles[0]
        _rename_profile(self.api_config, cookie, existing_profile_id, "Home")

        env = build_envelope(profiles=[build_profile(name="Home")])
        resp = do_import(cookie, password, env)
        assert resp.status_code == 200, resp.text
        body = resp.json()

        assert body["createdProfileNames"] == ["Home (imported)"], (
            f"Unexpected resolved names: {body['createdProfileNames']}"
        )
        warnings_blob = " ".join(body["warnings"]).lower()
        assert "home" in warnings_blob and "imported" in warnings_blob, (
            f"Rename warning missing original/resolved names; warnings={body['warnings']}"
        )

    def test_import_two_payload_profiles_same_name_get_unique_names(self):
        """Two payload profiles with the same name collide against each other within the batch. specRef: I24."""
        _, cookie, password, _ = create_account_with_password()

        env = build_envelope(
            profiles=[
                build_profile(name="Work"),
                build_profile(name="Work"),
            ]
        )
        resp = do_import(cookie, password, env)
        assert resp.status_code == 200, resp.text
        body = resp.json()

        assert body["createdProfileNames"] == ["Work", "Work (imported)"], (
            f"Intra-batch collision resolution wrong: {body['createdProfileNames']}"
        )
        warnings_blob = " ".join(body["warnings"]).lower()
        assert "work" in warnings_blob and "imported" in warnings_blob, (
            f"No collision warning emitted; warnings={body['warnings']}"
        )

    def test_import_collision_with_long_name_truncates_to_50_chars(self):
        """Collision with a 50-char name produces a resolved name capped at 50 chars. specRef: I24, V7."""
        account, cookie, password, _ = create_account_with_password()
        existing_profile_id = account.profiles[0]
        long_name = "X" * 50
        _rename_profile(self.api_config, cookie, existing_profile_id, long_name)

        env = build_envelope(profiles=[build_profile(name=long_name)])
        resp = do_import(cookie, password, env)
        assert resp.status_code == 200, resp.text
        body = resp.json()

        resolved = body["createdProfileNames"][0]
        assert len(resolved) <= 50, (
            f"Resolved name exceeds 50 chars: len={len(resolved)} value={resolved!r}"
        )
        assert resolved.endswith("(imported)"), (
            f"Resolved name lost the (imported) suffix: {resolved!r}"
        )


class TestCatalogDegradation:
    def setup_class(self):
        self.config = get_settings()
        self.api_config = api_config.Configuration(host=self.config.DNS_API_ADDR)

    def test_import_unknown_blocklist_warns_and_drops(self):
        """Unknown blocklist IDs are dropped with a warning; import still succeeds. specRef: V8."""
        _, cookie, password, _ = create_account_with_password()
        env = build_envelope(
            profiles=[
                build_profile(
                    name="UnknownBlocklistProfile",
                    blocklists=["nonexistent_blocklist_xyz"],
                )
            ]
        )
        resp = do_import(cookie, password, env)
        assert resp.status_code == 200, resp.text
        body = resp.json()
        assert body["warnings"], "Expected at least one warning for unknown blocklist"
        warnings_blob = " ".join(body["warnings"]).lower()
        assert "blocklist" in warnings_blob, (
            f"No blocklist warning emitted; warnings={body['warnings']}"
        )

        imported = _get_profile(
            self.api_config, cookie, body["createdProfileIds"][0]
        )
        assert "nonexistent_blocklist_xyz" not in (
            imported.settings.privacy.blocklists or []
        ), "Unknown blocklist id was persisted on the imported profile"

    def test_import_unknown_service_warns_and_drops(self):
        """Unknown service IDs are dropped with a warning; import still succeeds. specRef: V9."""
        _, cookie, password, _ = create_account_with_password()
        env = build_envelope(
            profiles=[
                build_profile(
                    name="UnknownServiceProfile",
                    services=["nonexistent_service_xyz"],
                )
            ]
        )
        resp = do_import(cookie, password, env)
        assert resp.status_code == 200, resp.text
        body = resp.json()
        assert body["warnings"], "Expected at least one warning for unknown service"
        warnings_blob = " ".join(body["warnings"]).lower()
        assert "service" in warnings_blob, (
            f"No service warning emitted; warnings={body['warnings']}"
        )

        imported = _get_profile(
            self.api_config, cookie, body["createdProfileIds"][0]
        )
        assert "nonexistent_service_xyz" not in (
            imported.settings.privacy.services or []
        ), "Unknown service id was persisted on the imported profile"

    def test_import_punycode_rule_emits_warning(self):
        """Punycode custom-rule values surface an advisory warning; import succeeds. specRef: S5, V12."""
        _, cookie, password, _ = create_account_with_password()
        env = build_envelope(
            profiles=[
                build_profile(
                    name="PunycodeProfile",
                    custom_rules=[{"action": "block", "value": PUNYCODE_RULE}],
                )
            ]
        )
        resp = do_import(cookie, password, env)
        assert resp.status_code == 200, resp.text
        body = resp.json()
        assert body["warnings"], "Expected at least one warning for punycode rule"
        warnings_blob = " ".join(body["warnings"])
        assert PUNYCODE_RULE in warnings_blob or "internationalized" in warnings_blob.lower(), (
            f"No punycode warning emitted; warnings={body['warnings']}"
        )


class TestProfileCountCap:
    def setup_class(self):
        self.config = get_settings()
        self.api_config = api_config.Configuration(host=self.config.DNS_API_ADDR)

    @staticmethod
    def _fill_profiles(api_config_, cookie, target_total):
        with client.ApiClient(api_config_) as api_client:
            p = api.ProfileApi(api_client)
            p.api_client.default_headers["Cookie"] = cookie
            account_api = api.AccountApi(api_client)
            account_api.api_client.default_headers["Cookie"] = cookie
            account = account_api.api_v1_accounts_current_get()
            existing = len(account.profiles)
            for i in range(existing, target_total):
                resp = p.api_v1_profiles_post_with_http_info(
                    body=ApiCreateProfileBody(name=f"filler_{i:03d}")
                )
                assert resp.status_code == 201, (
                    f"Filler profile {i} failed: {resp.status_code} {resp.data}"
                )

    def test_import_exceeds_max_profiles_rejected(self):
        """Importing 2 profiles when account holds 99 is rejected; no DB writes occur. specRef: I17, S6."""
        _, cookie, password, _ = create_account_with_password()
        self._fill_profiles(self.api_config, cookie, 99)

        env = build_envelope(
            profiles=[
                build_profile(name="cap_overflow_1"),
                build_profile(name="cap_overflow_2"),
            ]
        )
        resp = do_import(cookie, password, env)
        assert resp.status_code == 400, resp.text

        with client.ApiClient(self.api_config) as api_client:
            account_api = api.AccountApi(api_client)
            account_api.api_client.default_headers["Cookie"] = cookie
            account = account_api.api_v1_accounts_current_get()
            assert len(account.profiles) == 99, (
                f"Profile count changed after rejected import: {len(account.profiles)}"
            )

    def test_import_at_max_profiles_rejected(self):
        """Importing any envelope when account already holds MAX_PROFILES is rejected. specRef: I18."""
        _, cookie, password, _ = create_account_with_password()
        self._fill_profiles(self.api_config, cookie, 100)

        env = build_envelope(profiles=[build_profile(name="cap_at_max")])
        resp = do_import(cookie, password, env)
        assert resp.status_code == 400, resp.text

    def test_export_selected_capped_at_max_profiles(self):
        """scope=selected with > MAX_PROFILES profile_ids is rejected. specRef: E10."""
        _, cookie, password, _ = create_account_with_password()
        resp = raw_export(
            cookie,
            export_request_body(
                scope="selected",
                profile_ids=["x"] * 101,
                current_password=password,
            ),
        )
        assert resp.status_code == 400, resp.text


class TestCustomRulesExportLimits(ProfileHelpers):
    def setup_class(self):
        self.config = get_settings()
        self.api_config = api_config.Configuration(host=self.config.DNS_API_ADDR)

    def test_export_truncates_custom_rules_and_round_trips(self):
        """A profile with > EXPORTED_CUSTOM_RULES_LIMIT custom rules exports only
        the first that-many (oldest-first) and reports the trim via the
        X-modDNS-Export-Truncated header; the truncated export re-imports cleanly.
        specRef: E20, E21, V10.
        """
        _, cookie, password, _ = create_account_with_password()

        # Seed a profile at the cap via import (DTO max=1000), then push it over
        # the cap with one batch add. This avoids ~50 rate-limited batch calls
        # that creating 1000+ rules from scratch would need (20/min limit).
        seed = build_envelope(
            profiles=[
                build_profile(
                    name="trunc_src",
                    custom_rules=make_rules(EXPORTED_CUSTOM_RULES_LIMIT),
                )
            ]
        )
        seed_resp = do_import(cookie, password, seed)
        assert seed_resp.status_code == 200, seed_resp.text
        src_id = seed_resp.json()["createdProfileIds"][0]

        # Add a few more so the profile is over the cap even if a seed rule was
        # skipped for any reason.
        over = add_custom_rules_batch(
            cookie,
            src_id,
            [f"over-cap-{i}.example.org" for i in range(5)],
        )
        assert over.status_code == 200, over.text

        # Export just that profile; it must be truncated + flagged.
        export_resp = raw_export(
            cookie,
            export_request_body(
                scope="selected", profile_ids=[src_id], current_password=password
            ),
        )
        assert export_resp.status_code == 200, export_resp.text
        assert export_resp.headers.get("X-modDNS-Export-Truncated") == "1", (
            f"expected truncation header '1', got headers: {dict(export_resp.headers)}"
        )
        envelope = export_resp.json()
        assert len(envelope["profiles"]) == 1
        exported_rules = envelope["profiles"][0]["settings"]["customRules"]
        assert len(exported_rules) == EXPORTED_CUSTOM_RULES_LIMIT, (
            f"export must cap at {EXPORTED_CUSTOM_RULES_LIMIT}, got {len(exported_rules)}"
        )

        # Round-trip: the truncated export is a valid import (<= cap).
        reimport = do_import(cookie, password, envelope)
        assert reimport.status_code == 200, reimport.text
        new_id = reimport.json()["createdProfileIds"][0]
        new_profile = _get_profile(self.api_config, cookie, new_id)
        assert len(new_profile.settings.custom_rules or []) == EXPORTED_CUSTOM_RULES_LIMIT

    def test_import_rejects_over_cap_custom_rules(self):
        """Import payload with > EXPORTED_CUSTOM_RULES_LIMIT rules in a profile is
        rejected by the DTO. specRef: V10."""
        _, cookie, password, _ = create_account_with_password()
        env = build_envelope(
            profiles=[
                build_profile(
                    name="over_cap",
                    custom_rules=make_rules(EXPORTED_CUSTOM_RULES_LIMIT + 1),
                )
            ]
        )
        resp = do_import(cookie, password, env)
        assert resp.status_code == 400, resp.text

    def test_import_body_over_limit_returns_413(self):
        """An import body exceeding the import route's 5 MB limit is rejected with
        413 at the body-read layer, before DTO validation. This is only verifiable
        end-to-end (the Go handler test for it is skipped under app.Test). specRef: I5.
        """
        _, cookie, password, _ = create_account_with_password()
        # ~6 MB of rules. Content validity is irrelevant — 413 fires before the
        # handler/DTO runs, so over-cap / over-length values are fine here.
        big_value = "a" * 250
        big = build_envelope(
            profiles=[
                build_profile(
                    name="too_big",
                    custom_rules=[
                        {"action": "block", "value": big_value} for _ in range(25_000)
                    ],
                )
            ]
        )
        resp = do_import(cookie, password, big)
        assert resp.status_code == 413, (
            f"expected 413 for >5 MB body, got {resp.status_code}"
        )
