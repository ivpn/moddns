package model

import (
	"encoding/json"
	"testing"

	"github.com/ivpn/dns/libs/statistics"
	"github.com/stretchr/testify/require"
)

// specRef: api-endpoint-behaviour.md J48 — the API values are exactly the shared ones the proxy routes by.
func TestStatisticsRetentionMatchesLibs(t *testing.T) {
	require.Equal(t, []StatisticsRetention{StatisticsRetention30d, StatisticsRetention90d, StatisticsRetention1y}, StatisticsRetentions())
	for _, r := range statistics.Retentions() {
		require.True(t, StatisticsRetention(r).Valid())
		require.Equal(t, r.Window(), StatisticsRetention(r).Window())
	}
	require.Equal(t, StatisticsRetention30d, StatisticsRetention("bogus").OrDefault())
}

// specRef: api-endpoint-behaviour.md J49 — clients always read a valid retention: an empty or
// unknown stored value is rendered as 30d; the stored value and the Redis field are not changed.
func TestStatisticsSettingsJSONNormalisesRetention(t *testing.T) {
	for stored, want := range map[StatisticsRetention]string{"": "30d", "bogus": "30d", StatisticsRetention90d: "90d", StatisticsRetention1y: "1y"} {
		b, err := json.Marshal(Profile{Settings: &ProfileSettings{Statistics: &StatisticsSettings{Enabled: true, Retention: stored}}})
		require.NoError(t, err)
		var out struct {
			Settings struct {
				Statistics struct {
					Retention string `json:"retention"`
				} `json:"statistics"`
			} `json:"settings"`
		}
		require.NoError(t, json.Unmarshal(b, &out))
		require.Equal(t, want, out.Settings.Statistics.Retention, "stored %q", stored)
	}
	s := StatisticsSettings{Retention: ""}
	_, err := json.Marshal(s)
	require.NoError(t, err)
	require.Equal(t, StatisticsRetention(""), s.Retention, "marshalling does not mutate the settings")
}
