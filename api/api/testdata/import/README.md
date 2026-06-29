# Profile-import validation fixtures

Hand-authored `*.moddns.json` files that exercise the profile-import validation
and the corrected, user-facing error messages (QA issue
[#604](https://github.com/ivpn/moddns-shadow/issues/604)).

They serve **two** purposes from this one location:
- **Manual** — drop them into the web app's import dialog to see the messages a
  real user gets.
- **Automated** — `api/api/profile_import_validation_test.go` feeds each one through the real
  import handler and asserts the exact `error` string, so the messages documented
  here can't silently drift from the validator.

Each file is a normal export envelope (`schemaVersion`, `kind`, `exportedAt`,
`profiles[]`) — i.e. exactly what a downloaded export looks like — but with **one**
deliberate problem (except `16`, which has two).

## Manual usage (web app)
1. **Account → Backup & Restore → Import profiles**.
2. Drop or browse to one of the files below.
3. Observe the error. For the `NN-*` files it appears after you reauth and submit
   (the API returns it); for the `client-*` files it appears immediately on file
   selection (the dialog rejects them before any upload).

## Automated usage (Go test)
`api/api/profile_import_validation_test.go` reads this directory via the package-relative path
`testdata/import` and:
- wraps each bare envelope into the request body the frontend sends
  (`{mode, payload: <envelope>, current_password}`), POSTs it through the real
  import handler (`app.Test`), and asserts the response `error` equals the message
  in the table below (`serverFixtureMessages`);
- `TestManualImportFixtures_AllCovered` fails if a server-reachable file has **no**
  documented expected message — so a new fixture can't be added without recording
  what it produces.

Run it:
```bash
cd api && go test ./api/ -run 'TestProfileExportImportSuite/TestManualImportFixtures' -v
```

**Adding a fixture:** drop the `.moddns.json` file here, then add its filename →
expected message to `serverFixtureMessages` in `profile_import_validation_test.go` (a
`client-*` file is browser-validated and is excluded from the test instead).

## Server-validated files (the corrected messages)
These pass the dialog's client-side pre-check (valid JSON, `schemaVersion: 1`,
`kind: "moddns-export"`, ≥1 profile) and reach the API. The API returns each
message below **prefixed with `Validation error: `** (e.g.
`Validation error: profiles must be at most 100`); the table lists the message
body for brevity:

| File | Message shown |
|------|---------------|
| `01-too-many-profiles.moddns.json` | `profiles must be at most 100` |
| `02-too-many-custom-rules.moddns.json` | `customRules must be at most 1000` |
| `03-too-many-blocklists.moddns.json` | `blocklists must be at most 100` |
| `04-invalid-default-rule.moddns.json` | `defaultRule must be one of: block, allow` |
| `05-invalid-blocklists-subdomains-rule.moddns.json` | `blocklistsSubdomainsRule must be one of: block, allow` |
| `06-invalid-custom-rules-subdomains-rule.moddns.json` | `customRulesSubdomainsRule must be one of: include, exact` |
| `07-invalid-custom-rule-action.moddns.json` | `action must be one of: block, allow, comment` |
| `08-custom-rule-value-too-long.moddns.json` | `value must be at most 255` |
| `09-invalid-retention.moddns.json` | `retention must be one of: 1h, 6h, 1d, 1w, 1m` |
| `10-invalid-recursor.moddns.json` | `recursor must be one of: sdns, unbound` |
| `11-profile-name-too-long.moddns.json` | `name must be at most 200` |
| `12-profile-name-invalid-chars.moddns.json` | `name contains invalid characters` |
| `13-missing-exported-at.moddns.json` | `exportedAt is required` |
| `14-unknown-field-id.moddns.json` | `Unknown field '_id' is not allowed.` |
| `15-wrong-type-bool.moddns.json` | `Field 'enabled' has the wrong type (expected true or false).` |
| `16-multiple-errors.moddns.json` | `defaultRule must be one of: block, allow and action must be one of: block, allow, comment` |

None of these contain the old raw `[Field]: Needs to implement 'tag'` or `json: …`
text.

## Client-validated files (rejected in the browser, never reach the API)
The import dialog checks `schemaVersion`, `kind`, non-empty `profiles`, and JSON
parseability up front, so these show a generic dialog message instead of a
server message. They are intentionally **excluded** from the Go test:

| File | Message shown (client-side) |
|------|-----------------------------|
| `client-01-wrong-schema-version.moddns.json` | `Invalid export file. Expected a moddns-export JSON with at least one profile.` |
| `client-02-empty-profiles.moddns.json` | `Invalid export file. Expected a moddns-export JSON with at least one profile.` |
| `client-03-malformed-json.moddns.json` | `Could not parse the file. Make sure it is a valid moddns export JSON.` |
