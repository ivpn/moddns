package api

import (
	"reflect"
	"regexp"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ivpn/dns/api/model"
)

// TestCreateProfileBody_NameMaxLenMatchesCanonicalConst is the drift guard
// for the request-side profile-name length cap. The createProfileBody DTO
// is unexported in package api, so the test lives here (package api, not
// api_test) to access it via reflection. The model-side guard lives in
// api/model/profile_name_max_test.go.
func TestCreateProfileBody_NameMaxLenMatchesCanonicalConst(t *testing.T) {
	re := regexp.MustCompile(`(?:^|,)max=(\d+)(?:,|$)`)

	f, ok := reflect.TypeOf(createProfileBody{}).FieldByName("Name")
	require.True(t, ok, "createProfileBody.Name field not found")

	tag := f.Tag.Get("validate")
	m := re.FindStringSubmatch(tag)
	require.Len(t, m, 2, "createProfileBody.Name validate tag has no max=N segment: %q", tag)

	got, err := strconv.Atoi(m[1])
	require.NoError(t, err)
	assert.Equal(t, model.MaxProfileNameLen, got,
		"createProfileBody.Name validate tag has max=%d but model.MaxProfileNameLen=%d — update both together",
		got, model.MaxProfileNameLen)
}
