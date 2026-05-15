package dohpath

import (
	"strings"
	"testing"
)

func TestConstants(t *testing.T) {
	if Segment != "dns-query" {
		t.Errorf("Segment = %q, want %q", Segment, "dns-query")
	}
	if Prefix != "/dns-query/" {
		t.Errorf("Prefix = %q, want %q", Prefix, "/dns-query/")
	}
	if !strings.HasPrefix(Prefix, "/") || !strings.HasSuffix(Prefix, "/") {
		t.Errorf("Prefix %q must begin and end with a slash", Prefix)
	}
}

func TestFor(t *testing.T) {
	tests := []struct {
		name    string
		profile string
		device  string
		want    string
	}{
		{"profile only", "abc123def4", "", "/dns-query/abc123def4"},
		{"profile + simple device", "abc123def4", "laptop", "/dns-query/abc123def4/laptop"},
		{"profile + device with space", "abc123def4", "Living Room", "/dns-query/abc123def4/Living%20Room"},
		{"profile + device with hyphen", "abc123def4", "device-1", "/dns-query/abc123def4/device-1"},
		{"profile + alphanumeric device", "abc123def4", "iPhone12", "/dns-query/abc123def4/iPhone12"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := For(tt.profile, tt.device)
			if got != tt.want {
				t.Errorf("For(%q, %q) = %q, want %q", tt.profile, tt.device, got, tt.want)
			}
		})
	}
}

// TestForMatchesProxyFixtures pins For() against the same shapes the proxy's
// device_identification_test.go fixtures expect. If this test fails together
// with the proxy fixtures, the api and proxy have drifted — both must move
// in lockstep when the path scheme changes.
func TestForMatchesProxyFixtures(t *testing.T) {
	// proxy/server/device_identification_test.go fixtures use these exact paths.
	cases := []struct {
		profile string
		device  string
		path    string
	}{
		{"abc123", "", "/dns-query/abc123"},
		{"abc123", "my-laptop", "/dns-query/abc123/my-laptop"},
		{"abc123", "Home Router", "/dns-query/abc123/Home%20Router"},
	}
	for _, c := range cases {
		if got := For(c.profile, c.device); got != c.path {
			t.Errorf("For(%q, %q) = %q, proxy fixture expects %q", c.profile, c.device, got, c.path)
		}
	}
}
