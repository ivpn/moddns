package geoip

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	asnFixture  = "../../../libs/geoipdb/testdata/GeoLite2-ASN.mmdb"
	cityFixture = "../../../libs/geoipdb/testdata/GeoLite2-City.mmdb"
)

// tableRef: api-endpoint-behaviour #J22
func TestLookupEnrichesFromBothDatabases(t *testing.T) {
	e := New(Config{ASNFile: asnFixture, CountryFile: cityFixture, ReloadEvery: time.Minute})

	info := e.Lookup("8.8.8.8")
	require.NotNil(t, info.ASN)
	assert.EqualValues(t, 15169, *info.ASN)
	require.NotNil(t, info.ASOrg)
	assert.Equal(t, "GOOGLE", *info.ASOrg)
	require.NotNil(t, info.Country)
	assert.Equal(t, "US", *info.Country)

	// IPv4-mapped IPv6 hits the same entry as the IPv4 form.
	assert.Equal(t, info, e.Lookup("::ffff:8.8.8.8"))
}

// tableRef: api-endpoint-behaviour #J22
func TestLookupUnknownAndUnparsableAreNil(t *testing.T) {
	e := New(Config{ASNFile: asnFixture, CountryFile: cityFixture})
	for _, ip := range []string{"203.0.113.7", "not-an-ip", ""} {
		assert.Equal(t, Info{}, e.Lookup(ip), ip)
	}
}

// tableRef: api-endpoint-behaviour #J22
func TestMissingOrWrongDatabasesNeverFail(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "absent.mmdb")
	cases := map[string]Config{
		"nothing configured":  {},
		"files absent":        {ASNFile: missing, CountryFile: missing},
		"swapped editions":    {ASNFile: cityFixture, CountryFile: asnFixture},
		"only ASN configured": {ASNFile: asnFixture},
	}
	for name, cfg := range cases {
		t.Run(name, func(t *testing.T) {
			e := New(cfg)
			info := e.Lookup("8.8.8.8")
			assert.Nil(t, info.Country)
			if cfg.ASNFile != asnFixture {
				assert.Equal(t, Info{}, info)
			}
		})
	}
}

func TestNilEnricherIsSafe(t *testing.T) {
	var e *Enricher
	assert.Equal(t, Info{}, e.Lookup("8.8.8.8"))
}

// The backend E2E stack mounts these stubs; 8.8.8.8 must enrich to Google / US.
// tableRef: api-endpoint-behaviour #J22
func TestE2EStubDatabases(t *testing.T) {
	e := New(Config{
		ASNFile:     "../../../tests/bootstrap/geolite/GeoLite2-ASN.mmdb",
		CountryFile: "../../../tests/bootstrap/geolite/GeoLite2-Country.mmdb",
	})
	info := e.Lookup("8.8.8.8")
	require.NotNil(t, info.ASN)
	assert.EqualValues(t, 15169, *info.ASN)
	require.NotNil(t, info.Country)
	assert.Equal(t, "US", *info.Country)
}
