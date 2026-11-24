package profile

import (
	"context"

	"github.com/ivpn/dns/api/model"
)

// defaultSettings returns new profile settings with the default blocklists
// enabled. The profile ID is set when the profile is inserted.
func (p *ProfileService) defaultSettings(ctx context.Context) (*model.ProfileSettings, error) {
	// get list of default blocklists
	fltr := map[string]any{"default": true}
	defaultBlocklists, err := p.BlocklistService.GetBlocklist(ctx, fltr, "")
	if err != nil {
		return nil, err
	}

	// create default settings for the profile
	settings := model.NewSettings()
	for _, blocklist := range defaultBlocklists {
		settings.Privacy.Blocklists = append(settings.Privacy.Blocklists, blocklist.BlocklistID)
	}
	return settings, nil
}
