package profile

import (
	"context"
	"slices"
)

func (p *ProfileService) EnableBlocklists(ctx context.Context, accountId, profileId string, blocklistIds []string) error {
	// Validate profile ownership
	profile, err := p.validateProfileIdAffiliation(ctx, accountId, profileId)
	if err != nil {
		return err
	}

	if len(blocklistIds) == 0 {
		return ErrInvalidBlocklistValue
	}

	for _, blocklistID := range blocklistIds {
		// check if blocklist already exists
		fltr := map[string]any{"blocklist_id": blocklistID}
		blocklists, err := p.BlocklistService.GetBlocklist(ctx, fltr, "")
		if err != nil {
			return err
		}
		if len(blocklists) == 0 {
			return ErrBlocklistNotFound
		}
		if slices.Contains(profile.Settings.Privacy.Blocklists, blocklistID) {
			return ErrBlocklistAlreadyEnabled
		}
	}

	// Enable blocklists atomically in repository
	if err := p.ProfileRepository.EnableBlocklists(ctx, profileId, blocklistIds); err != nil {
		return err
	}

	if err = p.Cache.AppendBlocklistsToProfileSettings(ctx, profileId, blocklistIds...); err != nil {
		return err
	}

	return nil
}

func (p *ProfileService) DisableBlocklists(ctx context.Context, accountId, profileId string, blocklistIds []string) error {
	// Validate profile ownership
	_, err := p.validateProfileIdAffiliation(ctx, accountId, profileId)
	if err != nil {
		return err
	}

	if len(blocklistIds) == 0 {
		return ErrInvalidBlocklistValue
	}

	for _, blocklistID := range blocklistIds {
		// check if blocklist already exists
		fltr := map[string]any{"blocklist_id": blocklistID}
		blocklists, err := p.BlocklistService.GetBlocklist(ctx, fltr, "")
		if err != nil {
			return err
		}
		if len(blocklists) == 0 {
			return ErrBlocklistNotFound
		}
	}

	// Disable blocklists atomically in repository
	if err := p.ProfileRepository.DisableBlocklists(ctx, profileId, blocklistIds); err != nil {
		return err
	}

	// Optionally: update cache here if you cache blocklists
	if err = p.Cache.RemoveBlocklistsFromProfileSettings(ctx, profileId, blocklistIds...); err != nil {
		return err
	}

	return nil
}
