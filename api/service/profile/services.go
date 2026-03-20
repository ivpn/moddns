package profile

import (
	"context"
	"slices"
)

// EnableServices adds the given service IDs to settings.privacy.services.blocked for the profile.
func (p *ProfileService) EnableServices(ctx context.Context, accountId, profileId string, serviceIds []string) error {
	profile, err := p.validateProfileIdAffiliation(ctx, accountId, profileId)
	if err != nil {
		return err
	}
	if len(serviceIds) == 0 {
		return ErrInvalidServiceValue
	}
	blocked := make([]string, 0)
	if profile.Settings != nil && profile.Settings.Privacy != nil {
		blocked = profile.Settings.Privacy.Services
	}

	for _, id := range serviceIds {
		if id == "" {
			return ErrInvalidServiceValue
		}
		if slices.Contains(blocked, id) {
			return ErrServiceAlreadyEnabled
		}
	}

	if err := p.ProfileRepository.EnableServices(ctx, profileId, serviceIds); err != nil {
		return err
	}
	if err := p.Cache.AppendServicesBlockedToProfileSettings(ctx, profileId, serviceIds...); err != nil {
		return err
	}
	return nil
}

// DisableServices removes the given service IDs from settings.privacy.services.blocked for the profile.
func (p *ProfileService) DisableServices(ctx context.Context, accountId, profileId string, serviceIds []string) error {
	_, err := p.validateProfileIdAffiliation(ctx, accountId, profileId)
	if err != nil {
		return err
	}
	if len(serviceIds) == 0 {
		return ErrInvalidServiceValue
	}

	for _, id := range serviceIds {
		if id == "" {
			return ErrInvalidServiceValue
		}
	}

	if err := p.ProfileRepository.DisableServices(ctx, profileId, serviceIds); err != nil {
		return err
	}
	if err := p.Cache.RemoveServicesBlockedFromProfileSettings(ctx, profileId, serviceIds...); err != nil {
		return err
	}
	return nil
}
