package requestcontext

import (
	"context"
	"testing"

	"github.com/AdguardTeam/dnsproxy/proxy"
	"github.com/ivpn/dns/libs/logging"
	"github.com/ivpn/dns/proxy/model"
	"github.com/stretchr/testify/assert"
)

// NewRequestContext carries every per-profile filter input from the settings
// batch so the filter stages need no store reads of their own.
func TestNewRequestContext_CarriesBatchInputs(t *testing.T) {
	logger := logging.NewDefaultFactory().ForRequest(logging.LoggingConfig{Enabled: true})
	rules := []map[string]string{{"value": "ads.example", "action": "block"}}
	settings := &model.ProfileSettings{
		Privacy:             map[string]string{"default_rule": "block"},
		Logs:                map[string]string{"enabled": "true"},
		DNSSEC:              map[string]string{"enabled": "true"},
		RebindingProtection: map[string]string{"enabled": "1"},
		Advanced:            map[string]string{"recursor": "knot"},
		Statistics:          map[string]string{"enabled": "false"},
		Blocklists:          []string{"bl1", "bl2"},
		Services:            []string{"google"},
		CustomRules:         rules,
	}

	rc := NewRequestContext(context.Background(), &proxy.Proxy{}, "pid", "did", settings, logger)

	assert.Equal(t, "pid", rc.ProfileId)
	assert.Equal(t, "did", rc.DeviceId)
	assert.Equal(t, settings.Privacy, rc.PrivacySettings)
	assert.Equal(t, settings.Logs, rc.LogsSettings)
	assert.Equal(t, settings.DNSSEC, rc.DNSSECSettings)
	assert.Equal(t, settings.RebindingProtection, rc.RebindingProtectionSettings)
	assert.Equal(t, settings.Advanced, rc.AdvancedSettings)
	assert.Equal(t, settings.Statistics, rc.StatisticsSettings)
	assert.Equal(t, []string{"bl1", "bl2"}, rc.Blocklists)
	assert.Equal(t, []string{"google"}, rc.BlockedServices)
	assert.Equal(t, rules, rc.CustomRules)
	assert.Equal(t, logger.Config(), rc.LoggerConfig)
}

func TestNewRequestContext_NilSettings(t *testing.T) {
	logger := logging.NewDefaultFactory().ForRequest(logging.LoggingConfig{Enabled: true})

	var rc *RequestContext
	assert.NotPanics(t, func() {
		rc = NewRequestContext(context.Background(), nil, "pid", "", nil, logger)
	})
	assert.Nil(t, rc.PrivacySettings)
	assert.Nil(t, rc.Blocklists)
	assert.Nil(t, rc.BlockedServices)
	assert.Nil(t, rc.CustomRules)
	assert.Nil(t, rc.StatisticsSettings)
}
