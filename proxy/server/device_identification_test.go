package server

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"testing"

	"github.com/AdguardTeam/dnsproxy/proxy"
)

func TestDeviceIdentification(t *testing.T) {
	fmt.Println("Testing Device Identification...")

	// Ensure tests reflect updated max length (36)

	// Test DoH device identification
	t.Run("DoH Device Identification", func(t *testing.T) {
		testDoHDeviceIdentification(t)
	})

	// Test DoT/DoQ device identification
	t.Run("DoT/DoQ Device Identification", func(t *testing.T) {
		testDoTDeviceIdentification(t)
	})
}

func TestProfileIDMinLengthConfigurable(t *testing.T) {
	original := profileIDMinLength
	defer func() { profileIDMinLength = original }()
	os.Setenv("PROFILE_ID_MIN_LENGTH", "12")
	defer os.Unsetenv("PROFILE_ID_MIN_LENGTH")

	// Simulate server init override
	if v := os.Getenv("PROFILE_ID_MIN_LENGTH"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n < 64 {
			profileIDMinLength = n
		}
	}

	shortID := "abcdefghijk"  // 11 chars
	validID := "abcdefghijkl" // 12 chars
	if isValidProfileID(shortID) {
		t.Fatalf("expected shortID (%d chars) to be invalid when min len 12", len(shortID))
	}
	if !isValidProfileID(validID) {
		t.Fatalf("expected validID (%d chars) to be valid when min len 12", len(validID))
	}
}

func testDoHDeviceIdentification(t *testing.T) {
	testCases := []struct {
		url            string
		expectedDevice string
		expectedClient string
	}{
		{"/dns-query/abc123", "", "abc123"},
		{"/dns-query/abc123/my-laptop", "my-laptop", "abc123"},
		// Apostrophe removed by normalization
		{"/dns-query/abc123/John%27s%20iPhone", "Johns iPhone", "abc123"},
		{"/dns-query/abc123/Home%20Router", "Home Router", "abc123"},
		// Previously truncated at 16; now length < 36 so remains whole
		{"/dns-query/abc123/ThisDeviceNameIsWayTooLong", "ThisDeviceNameIsWayTooLong", "abc123"},
		// Script tag & angle brackets removed. No truncation now (length 19 < 36)
		{"/dns-query/abc123/%3Cscript%3Ealert(1)%3Cscript%3E", "scriptalert1script", "abc123"},
		// CRLF removed; length (18) < 36 so no truncation
		{"/dns-query/abc123/DeviceName%0d%0aAnother", "DeviceNameAnother", "abc123"},
		// ANSI escape sequences stripped (ESC = %1b)
		{"/dns-query/abc123/Name%1b[31mRED%1b[0m", "Name31mRED0m", "abc123"},
		// Mixed special chars
		{"/dns-query/abc123/@@@My__Device!!!", "MyDevice", "abc123"},
		// New truncation test >36 chars
		{"/dns-query/abc123/ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789EXTRA", "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789", "abc123"},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("URL_%s", tc.url), func(t *testing.T) {
			// Create a mock DNS context for DoH
			req, _ := http.NewRequest("POST", tc.url, nil)
			dctx := &proxy.DNSContext{
				Proto:       proxy.ProtoHTTPS,
				HTTPRequest: req,
			}

			clientID, deviceId, err := clientIDFromDNSContextHTTPS(dctx)
			if err != nil {
				t.Errorf("Error: %v", err)
				return
			}

			if clientID != tc.expectedClient {
				t.Errorf("Expected client ID: %s, got: %s", tc.expectedClient, clientID)
			}

			if deviceId != tc.expectedDevice {
				t.Errorf("Expected device ID: %s, got: %s", tc.expectedDevice, deviceId)
			}
		})
	}
}

func testDoTDeviceIdentification(t *testing.T) {
	testCases := []struct {
		serverName     string
		expectedDevice string
		expectedClient string
	}{
		{"3mdq3851b9.example.com", "", "3mdq3851b9"},
		{"test-3mdq3851b9.example.com", "test", "3mdq3851b9"},
		{"my-laptop-3mdq3851b9.example.com", "my-laptop", "3mdq3851b9"},
		{"home--router-3mdq3851b9.example.com", "home router", "3mdq3851b9"},
		{"johns--iphone-3mdq3851b9.example.com", "johns iphone", "3mdq3851b9"},
		// Previously truncated at 16; now length < 36 so remains whole
		{"thisisaveryverylongname-3mdq3851b9.example.com", "thisisaveryverylongname", "3mdq3851b9"},
		// Invalid chars removed by sanitation
		{"my*lap^top-3mdq3851b9.example.com", "mylaptop", "3mdq3851b9"},
		// Percent-encoded pattern; we only keep allowed chars
		{"script%3Calert%3E-3mdq3851b9.example.com", "script3Calert3E", "3mdq3851b9"},
		// New truncation test (>36 chars) -> truncated to first 36 chars
		{"abcdefghijklmnopqrstuvwxyz0123456789extra-3mdq3851b9.example.com", "abcdefghijklmnopqrstuvwxyz0123456789", "3mdq3851b9"},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("ServerName_%s", tc.serverName), func(t *testing.T) {
			clientID, deviceId, err := clientIDFromClientServerName("example.com", tc.serverName, false, proxy.ProtoTLS)
			if err != nil {
				t.Errorf("Error: %v", err)
				return
			}

			if clientID != tc.expectedClient {
				t.Errorf("Expected client ID: %s, got: %s", tc.expectedClient, clientID)
			}

			if deviceId != tc.expectedDevice {
				t.Errorf("Expected device ID: %s, got: %s", tc.expectedDevice, deviceId)
			}
		})
	}
}

// TestLocationSubdomainIdentification verifies that profile/device extraction
// works when the client connects via a location-specific subdomain
// (e.g. prof123.ams1.dns.moddns.net) by matching against the location
// server name (ams1.dns.moddns.net) rather than the anycast name.
func TestLocationSubdomainIdentification(t *testing.T) {
	testCases := []struct {
		name           string
		hostSrvName    string
		cliSrvName     string
		expectedClient string
		expectedDevice string
	}{
		{
			name:           "profile only via location subdomain",
			hostSrvName:    "ams1.dns.moddns.net",
			cliSrvName:     "3mdq3851b9.ams1.dns.moddns.net",
			expectedClient: "3mdq3851b9",
			expectedDevice: "",
		},
		{
			name:           "device and profile via location subdomain",
			hostSrvName:    "ams1.dns.moddns.net",
			cliSrvName:     "my-laptop-3mdq3851b9.ams1.dns.moddns.net",
			expectedClient: "3mdq3851b9",
			expectedDevice: "my-laptop",
		},
		{
			name:           "profile only via anycast domain",
			hostSrvName:    "dns.moddns.net",
			cliSrvName:     "3mdq3851b9.dns.moddns.net",
			expectedClient: "3mdq3851b9",
			expectedDevice: "",
		},
		{
			name:           "location SNI does not match anycast host (IsImmediateSubdomain fails)",
			hostSrvName:    "dns.moddns.net",
			cliSrvName:     "3mdq3851b9.ams1.dns.moddns.net",
			expectedClient: "",
			expectedDevice: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			clientID, deviceId, err := clientIDFromClientServerName(tc.hostSrvName, tc.cliSrvName, false, proxy.ProtoTLS)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if clientID != tc.expectedClient {
				t.Errorf("clientID: got %q, want %q", clientID, tc.expectedClient)
			}
			if deviceId != tc.expectedDevice {
				t.Errorf("deviceId: got %q, want %q", deviceId, tc.expectedDevice)
			}
		})
	}
}

// TestMultiServerNameIteration verifies the multi-name iteration logic:
// when a proxy is configured with both an anycast and location server name,
// the correct one is selected based on the client's SNI.
func TestMultiServerNameIteration(t *testing.T) {
	serverNames := []string{"dns.moddns.net", "ams1.dns.moddns.net"}

	testCases := []struct {
		name           string
		cliSrvName     string
		expectedClient string
		expectedDevice string
	}{
		{
			name:           "anycast SNI matches first server name",
			cliSrvName:     "3mdq3851b9.dns.moddns.net",
			expectedClient: "3mdq3851b9",
			expectedDevice: "",
		},
		{
			name:           "location SNI matches second server name",
			cliSrvName:     "3mdq3851b9.ams1.dns.moddns.net",
			expectedClient: "3mdq3851b9",
			expectedDevice: "",
		},
		{
			name:           "device+profile via location",
			cliSrvName:     "my-laptop-3mdq3851b9.ams1.dns.moddns.net",
			expectedClient: "3mdq3851b9",
			expectedDevice: "my-laptop",
		},
		{
			name:           "device+profile via anycast",
			cliSrvName:     "my-laptop-3mdq3851b9.dns.moddns.net",
			expectedClient: "3mdq3851b9",
			expectedDevice: "my-laptop",
		},
		{
			name:           "unknown location returns empty",
			cliSrvName:     "3mdq3851b9.fra1.dns.moddns.net",
			expectedClient: "",
			expectedDevice: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var clientID, deviceId string
			for _, hostSrvName := range serverNames {
				cid, did, err := clientIDFromClientServerName(hostSrvName, tc.cliSrvName, false, proxy.ProtoTLS)
				if err == nil && cid != "" {
					clientID = cid
					deviceId = did
					break
				}
			}
			if clientID != tc.expectedClient {
				t.Errorf("clientID: got %q, want %q", clientID, tc.expectedClient)
			}
			if deviceId != tc.expectedDevice {
				t.Errorf("deviceId: got %q, want %q", deviceId, tc.expectedDevice)
			}
		})
	}
}

// specRef: #Q11 — profile IDs are strictly alphanumeric with a minimum
// length, checked per rune. This predicate is the selection gate for
// untrusted SNI input, so both accept and reject sides are pinned here.
func TestIsValidProfileIDCharacterClasses(t *testing.T) {
	// Default minimum length (10) applies; every reject case below that is
	// long enough fails on characters, not length.
	tests := []struct {
		name string
		id   string
		want bool
	}{
		{"lowercase accepted", "abcdefghij", true},
		{"uppercase accepted", "ABCDEFGHIJ", true},
		{"digits accepted", "0123456789", true},
		{"mixed accepted", "3mdq3851b9", true},
		{"below min length rejected", "abcdefghi", false},
		{"empty rejected", "", false},
		{"underscore rejected", "abcdefghi_", false},
		{"dot rejected", "abcdefghi.", false},
		{"colon rejected", "abcdefghi:", false},
		{"space rejected", "abcdefghi ", false},
		{"hyphen rejected", "abcde-fghi", false},
		{"control char rejected", "abcdefghi\x00", false},
		{"multi-byte rune rejected", "abcdefghiä", false},
		{"emoji rejected", "abcdefghi🌐", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidProfileID(tt.id); got != tt.want {
				t.Errorf("isValidProfileID(%q) = %v, want %v", tt.id, got, tt.want)
			}
		})
	}
}

// specRef: #Q11 — an invalid character in an SNI part must change profile-ID
// selection: the failing part is never chosen, an earlier valid part wins,
// and a subdomain with no valid part is an error. IsImmediateSubdomain does
// not DNS-validate label content, so these bytes genuinely reach the gate.
func TestClientIDFromClientServerNameProfileIDSelection(t *testing.T) {
	const host = "example.com"

	tests := []struct {
		name         string
		cliSrvName   string
		wantClientID string
		wantDeviceID string
		wantErr      bool
	}{
		{
			name:         "valid last part selected",
			cliSrvName:   "mydevice-3mdq3851b9.example.com",
			wantClientID: "3mdq3851b9",
			wantDeviceID: "mydevice",
		},
		{
			name:         "invalid char in last part falls back to earlier valid part",
			cliSrvName:   "3mdq3851b9xy-bad_part.example.com",
			wantClientID: "3mdq3851b9xy",
			wantDeviceID: "",
		},
		{
			name:       "invalid char in only long-enough part is an error",
			cliSrvName: "mydevice-3mdq3851b_9x.example.com",
			wantErr:    true,
		},
		{
			name:       "colon in single part is an error",
			cliSrvName: "3mdq3851:b9x.example.com",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientID, deviceId, err := clientIDFromClientServerName(host, tt.cliSrvName, false, proxy.ProtoTLS)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got clientID=%q deviceId=%q", clientID, deviceId)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if clientID != tt.wantClientID {
				t.Errorf("clientID = %q, want %q", clientID, tt.wantClientID)
			}
			if deviceId != tt.wantDeviceID {
				t.Errorf("deviceId = %q, want %q", deviceId, tt.wantDeviceID)
			}
		})
	}
}
