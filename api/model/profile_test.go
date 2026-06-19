package model

import (
	"reflect"
	"regexp"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// validateMaxRegex captures the integer N from a `max=N` segment inside a
// validate struct tag. Anchoring on `,` or string boundary prevents false
// matches on hypothetical future tags like `xmax=...`.
var validateMaxRegex = regexp.MustCompile(`(?:^|,)max=(\d+)(?:,|$)`)

// extractMaxFromValidateTag returns the integer N from a `max=N` segment in
// the field's `validate` struct tag, or fails the test if no such segment
// is present.
func extractMaxFromValidateTag(t *testing.T, label string, tag reflect.StructTag) int {
	t.Helper()
	v := tag.Get("validate")
	m := validateMaxRegex.FindStringSubmatch(v)
	require.Len(t, m, 2, "%s: validate tag has no max=N segment: %q", label, v)
	n, err := strconv.Atoi(m[1])
	require.NoError(t, err, "%s: malformed max=%q", label, m[1])
	return n
}

// TestProfileName_MaxLenMatchesCanonicalConst is the drift guard for the
// profile-name length. The stored Profile.Name must equal MaxProfileNameLen
// (50). ExportedProfile.Name is intentionally higher (200) so the import
// service can accept over-long names and truncate them to MaxProfileNameLen
// rather than rejecting them with a 400 — that higher value is tested in the
// separate TestExportedProfileName_WireMaxIsPermissive test below.
//
// Sites guarded here:
//   - model.Profile.Name             — storage shape (must equal MaxProfileNameLen)
//
// The parallel guard for createProfileBody.Name (the request DTO in
// package api) lives in api/api/profiles_max_name_test.go because the type
// is unexported there.
func TestProfileName_MaxLenMatchesCanonicalConst(t *testing.T) {
	f, ok := reflect.TypeOf(Profile{}).FieldByName("Name")
	require.True(t, ok, "Profile.Name field not found")
	got := extractMaxFromValidateTag(t, "model.Profile.Name", f.Tag)
	assert.Equal(t, MaxProfileNameLen, got,
		"model.Profile.Name validate tag has max=%d but MaxProfileNameLen=%d — update both together",
		got, MaxProfileNameLen)
}

// TestExportedProfileName_WireMaxIsPermissive asserts that the import-wire
// DTO allows names longer than MaxProfileNameLen so that the import service
// can normalise (truncate) them instead of returning a 400. The import
// service must always truncate to MaxProfileNameLen before persisting.
func TestExportedProfileName_WireMaxIsPermissive(t *testing.T) {
	f, ok := reflect.TypeOf(ExportedProfile{}).FieldByName("Name")
	require.True(t, ok, "ExportedProfile.Name field not found")
	got := extractMaxFromValidateTag(t, "model.ExportedProfile.Name", f.Tag)
	assert.Greater(t, got, MaxProfileNameLen,
		"ExportedProfile.Name max=%d must exceed MaxProfileNameLen=%d so the import service can truncate rather than reject",
		got, MaxProfileNameLen)
}

// TestExportedCustomRules_MaxMatchesCanonicalConst is the drift guard for the
// per-profile custom-rules cap: the import-wire DTO tag must equal
// MaxCustomRulesPerProfile — the same limit the create and import services
// enforce — so update both together.
func TestExportedCustomRules_MaxMatchesCanonicalConst(t *testing.T) {
	f, ok := reflect.TypeOf(ExportedSettings{}).FieldByName("CustomRules")
	require.True(t, ok, "ExportedSettings.CustomRules field not found")
	got := extractMaxFromValidateTag(t, "model.ExportedSettings.CustomRules", f.Tag)
	assert.Equal(t, MaxCustomRulesPerProfile, got,
		"ExportedSettings.CustomRules validate tag has max=%d but MaxCustomRulesPerProfile=%d — update both together",
		got, MaxCustomRulesPerProfile)
}
