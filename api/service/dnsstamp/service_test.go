package dnsstamp

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ivpn/dns/api/api/requests"
	"github.com/ivpn/dns/api/config"
	"github.com/ivpn/dns/libs/dnsstamps"
	"github.com/ivpn/dns/libs/dohpath"
)

const (
	testDomain   = "dns.moddns.net"
	testIPv4     = "198.51.100.10"
	testDoTPort  = 853
	testDoQPort  = 853
	testProfile  = "abc123def4"
	testDevice   = "Living Room"
	testDeviceUR = "Living%20Room"
	testDeviceLB = "Living--Room"
)

func newTestService(t *testing.T) DNSStampService {
	t.Helper()
	cfg := &config.Config{
		Server: &config.ServerConfig{
			DnsDomain:       testDomain,
			ServerAddresses: []string{testIPv4},
			DoTPort:         testDoTPort,
			DoQPort:         testDoQPort,
		},
	}
	return NewDNSStampService(cfg)
}

// specRef: M1, M4
func TestGenerateStamps_DoH_DecodesCorrectly(t *testing.T) {
	s := newTestService(t)
	resp, err := s.GenerateStamps(context.Background(), requests.DNSStampReq{ProfileId: testProfile})
	require.NoError(t, err)

	st, err := dnsstamps.NewServerStampFromString(resp.DoH)
	require.NoError(t, err)

	assert.Equal(t, dnsstamps.StampProtoTypeDoH, st.Proto)
	assert.Equal(t, testDomain, st.ProviderName)
	assert.Equal(t, dohpath.For(testProfile, ""), st.Path)
	// dnsstamps re-adds :443 to bare IPs for DoH; we accept either form to be
	// resilient to library version changes.
	assert.True(t,
		st.ServerAddrStr == testIPv4 || st.ServerAddrStr == testIPv4+":443",
		"DoH ServerAddrStr = %q, want %q or %q", st.ServerAddrStr, testIPv4, testIPv4+":443",
	)
}

// specRef: M1 — defensive against dnsstamps library default DoT port (843) vs
// production (853). If the library default ever changes to 853, this test
// still passes — what we care about is that the wire encoding carries 853.
func TestGenerateStamps_DoT_PortExplicit(t *testing.T) {
	s := newTestService(t)
	resp, err := s.GenerateStamps(context.Background(), requests.DNSStampReq{ProfileId: testProfile})
	require.NoError(t, err)

	st, err := dnsstamps.NewServerStampFromString(resp.DoT)
	require.NoError(t, err)
	assert.Equal(t, dnsstamps.StampProtoTypeTLS, st.Proto)
	assert.Equal(t, testIPv4+":853", st.ServerAddrStr, "DoT must carry :853 explicitly")
	assert.Equal(t, testProfile+"."+testDomain, st.ProviderName)
}

// specRef: M1 — same port-mismatch defence for DoQ (library default 784, prod 853).
func TestGenerateStamps_DoQ_PortExplicit(t *testing.T) {
	s := newTestService(t)
	resp, err := s.GenerateStamps(context.Background(), requests.DNSStampReq{ProfileId: testProfile})
	require.NoError(t, err)

	st, err := dnsstamps.NewServerStampFromString(resp.DoQ)
	require.NoError(t, err)
	assert.Equal(t, dnsstamps.StampProtoTypeDoQ, st.Proto)
	assert.Equal(t, testIPv4+":853", st.ServerAddrStr, "DoQ must carry :853 explicitly")
	assert.Equal(t, testProfile+"."+testDomain, st.ProviderName)
}

// specRef: M5 — device id propagated into DoH path (URL-encoded) and DoT/DoQ SNI
// (label-encoded with -- for spaces).
func TestGenerateStamps_WithDeviceID(t *testing.T) {
	s := newTestService(t)
	resp, err := s.GenerateStamps(context.Background(), requests.DNSStampReq{
		ProfileId: testProfile,
		DeviceId:  testDevice,
	})
	require.NoError(t, err)

	doh, err := dnsstamps.NewServerStampFromString(resp.DoH)
	require.NoError(t, err)
	assert.Contains(t, doh.Path, testDeviceUR, "DoH path must URL-encode device id")
	assert.Equal(t, dohpath.For(testProfile, testDevice), doh.Path)

	dot, err := dnsstamps.NewServerStampFromString(resp.DoT)
	require.NoError(t, err)
	assert.Equal(t, testDeviceLB+"-"+testProfile+"."+testDomain, dot.ProviderName,
		"DoT SNI must use <encoded-device>-<profile>.<domain> per clientid.go contract")

	doq, err := dnsstamps.NewServerStampFromString(resp.DoQ)
	require.NoError(t, err)
	assert.Equal(t, testDeviceLB+"-"+testProfile+"."+testDomain, doq.ProviderName)
}

// specRef: M1 — props bitmap matches modDNS reality: DNSSEC=yes, NoLog=yes, NoFilter=no.
func TestGenerateStamps_PropsBitmap(t *testing.T) {
	s := newTestService(t)
	resp, err := s.GenerateStamps(context.Background(), requests.DNSStampReq{ProfileId: testProfile})
	require.NoError(t, err)

	for proto, str := range map[string]string{"doh": resp.DoH, "dot": resp.DoT, "doq": resp.DoQ} {
		st, err := dnsstamps.NewServerStampFromString(str)
		require.NoError(t, err, proto)
		assert.NotZero(t, st.Props&dnsstamps.ServerInformalPropertyDNSSEC, "%s: DNSSEC must be set", proto)
		assert.NotZero(t, st.Props&dnsstamps.ServerInformalPropertyNoLog, "%s: NoLog must be set", proto)
		assert.Zero(t, st.Props&dnsstamps.ServerInformalPropertyNoFilter, "%s: NoFilter must NOT be set (modDNS filters)", proto)
	}
}

// specRef: M4
// The drift-proof trap test. If the proxy ever changes its DoH path scheme,
// this test fails — and the proxy's own router test must fail too because
// both consume libs/dohpath.Prefix. Drift is impossible without both moving.
func TestGenerateStamps_DoHPathMatchesProxyContract(t *testing.T) {
	s := newTestService(t)

	cases := []struct {
		profile, device string
	}{
		{testProfile, ""},
		{testProfile, testDevice},
		{"abc123", "Home Router"}, // same shape as proxy/server/device_identification_test.go fixtures
	}
	for _, c := range cases {
		resp, err := s.GenerateStamps(context.Background(), requests.DNSStampReq{
			ProfileId: c.profile,
			DeviceId:  c.device,
		})
		require.NoError(t, err)

		st, err := dnsstamps.NewServerStampFromString(resp.DoH)
		require.NoError(t, err)
		require.Equal(t, dohpath.For(c.profile, c.device), st.Path,
			"DoH path drifted from libs/dohpath contract — proxy router will reject these stamps")
	}
}

// Sad path: missing anycast IP.
func TestGenerateStamps_NoServerAddress(t *testing.T) {
	cfg := &config.Config{
		Server: &config.ServerConfig{
			DnsDomain:       testDomain,
			ServerAddresses: nil,
			DoTPort:         testDoTPort,
			DoQPort:         testDoQPort,
		},
	}
	s := NewDNSStampService(cfg)
	_, err := s.GenerateStamps(context.Background(), requests.DNSStampReq{ProfileId: testProfile})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNoServerAddress))
}

// Sad path: missing domain.
func TestGenerateStamps_NoDomain(t *testing.T) {
	cfg := &config.Config{
		Server: &config.ServerConfig{
			DnsDomain:       "",
			ServerAddresses: []string{testIPv4},
			DoTPort:         testDoTPort,
			DoQPort:         testDoQPort,
		},
	}
	s := NewDNSStampService(cfg)
	_, err := s.GenerateStamps(context.Background(), requests.DNSStampReq{ProfileId: testProfile})
	require.Error(t, err)
}

// Surface check: all three stamps share the same sdns:// prefix and decode cleanly.
func TestGenerateStamps_AllProtosAreSdnsPrefixed(t *testing.T) {
	s := newTestService(t)
	resp, err := s.GenerateStamps(context.Background(), requests.DNSStampReq{ProfileId: testProfile})
	require.NoError(t, err)

	for proto, str := range map[string]string{"doh": resp.DoH, "dot": resp.DoT, "doq": resp.DoQ} {
		assert.True(t, strings.HasPrefix(str, "sdns://"), "%s missing sdns:// prefix: %q", proto, str)
	}
}
