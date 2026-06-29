// Package version exposes the build-stamped application version.
//
// The Version variable is set at build time via linker flags:
//
//	go build -ldflags "-X github.com/ivpn/dns/api/internal/version.Version=<value>"
//
// In production the value flows from the deploy pipeline:
//
//	ansible image_tag -> Docker ARG APP_VERSION -> -ldflags -X -> Version
//
// Unstamped builds (go test, local go run, plain `go build` without the
// linker flag) keep the default "dev" value, which surfaces in places like
// the profile-export `appVersion` field.
package version

// Version is the application version string. Do not mutate at runtime —
// any change must come from the linker flag at build time.
var Version = "dev"
