package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// specRef: api-endpoint-behaviour.md J22 — both paths optional, reload defaults to 15m.
func TestGeoIPConfig(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("GEOIP_DB_ASN_FILE", "")
	t.Setenv("GEOIP_DB_COUNTRY_FILE", "")
	t.Setenv("GEOIP_DB_RELOAD", "")
	cfg, err := New()
	require.NoError(t, err)
	require.Empty(t, cfg.Service.GeoIPASNFile)
	require.Empty(t, cfg.Service.GeoIPCountryFile)
	require.Equal(t, 15*time.Minute, cfg.Service.GeoIPReloadEvery)

	t.Setenv("GEOIP_DB_ASN_FILE", " /opt/geo/GeoLite2-ASN.mmdb ")
	t.Setenv("GEOIP_DB_COUNTRY_FILE", "/opt/geo/GeoLite2-City.mmdb")
	t.Setenv("GEOIP_DB_RELOAD", "1h")
	cfg, err = New()
	require.NoError(t, err)
	require.Equal(t, "/opt/geo/GeoLite2-ASN.mmdb", cfg.Service.GeoIPASNFile)
	require.Equal(t, "/opt/geo/GeoLite2-City.mmdb", cfg.Service.GeoIPCountryFile)
	require.Equal(t, time.Hour, cfg.Service.GeoIPReloadEvery)
}

// specRef: api-endpoint-behaviour.md J22 — an invalid reload interval stops startup.
func TestGeoIPReloadConfig_RejectsInvalid(t *testing.T) {
	for _, v := range []string{"soon", "0s", "-1h"} {
		t.Run(v, func(t *testing.T) {
			setRequiredEnv(t)
			t.Setenv("GEOIP_DB_RELOAD", v)
			_, err := New()
			require.Error(t, err)
			require.Contains(t, err.Error(), "GEOIP_DB_RELOAD")
		})
	}
}
