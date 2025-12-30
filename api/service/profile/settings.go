package profile

import (
	"context"

	"github.com/ivpn/dns/api/model"
)

// createSettings creates a new settings and associates it with given profile ID.
func (p *ProfileService) createSettings(ctx context.Context, profileId string) (*model.ProfileSettings, error) {
	// get list of default blocklists
	fltr := map[string]any{"default": true}
	defaultBlocklists, err := p.BlocklistService.GetBlocklist(ctx, fltr, "")
	if err != nil {
		return nil, err
	}

	// create default settings for the profile
	settings := model.NewSettings()
	settings.ProfileId = profileId
	for _, blocklist := range defaultBlocklists {
		settings.Privacy.Blocklists = append(settings.Privacy.Blocklists, blocklist.BlocklistID)
	}
	// save settings to cache
	if err := p.Cache.CreateOrUpdateProfileSettings(ctx, settings, true); err != nil {
		return nil, err
	}

	return settings, nil
}
