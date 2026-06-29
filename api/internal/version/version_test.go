package version

import "testing"

// TestVersion_DefaultIsNonEmpty guards against accidental regression to an
// empty Version string. The package default ("dev") is what unstamped builds
// emit and what `go test` sees here; production builds replace it via
// `-ldflags -X`. Either way the value must never be empty — the export
// envelope's `appVersion` field reads directly from it.
func TestVersion_DefaultIsNonEmpty(t *testing.T) {
	if Version == "" {
		t.Fatal("Version must never be the empty string; expected at least the \"dev\" fallback")
	}
}
