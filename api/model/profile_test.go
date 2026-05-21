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
// profile-name length. Every place that caps a profile name at 50 must read
// from MaxProfileNameLen; the literal `max=50` in the struct tags is asserted
// here to match the const so CI fails fast if anyone bumps one without the
// other.
//
// Sites guarded:
//   - model.Profile.Name             — storage shape
//   - model.ExportedProfile.Name     — export-envelope shape
//
// The parallel guard for createProfileBody.Name (the request DTO in
// package api) lives in api/api/profiles_max_name_test.go because the type
// is unexported there.
func TestProfileName_MaxLenMatchesCanonicalConst(t *testing.T) {
	cases := []struct {
		label string
		typ   reflect.Type
	}{
		{"model.Profile.Name", reflect.TypeOf(Profile{})},
		{"model.ExportedProfile.Name", reflect.TypeOf(ExportedProfile{})},
	}

	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			f, ok := tc.typ.FieldByName("Name")
			require.True(t, ok, "%s: Name field not found on %s", tc.label, tc.typ)
			got := extractMaxFromValidateTag(t, tc.label, f.Tag)
			assert.Equal(t, MaxProfileNameLen, got,
				"%s validate tag has max=%d but MaxProfileNameLen=%d — update both together",
				tc.label, got, MaxProfileNameLen)
		})
	}
}
