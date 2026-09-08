package model

import "errors"

// ErrSettingsNotFound marks a settings hash that the store answered for but
// that is empty: the profile, or that settings group, does not exist. Every
// other store error is an infrastructure failure. Defined here so both the
// cache and the server can use it without an import cycle.
var ErrSettingsNotFound = errors.New("settings not found")

// ProfileSettings holds everything the proxy needs to know about one profile,
// fetched in a single batch and cached in process. The filter stages read
// their per-profile inputs from here; only blocklist membership is looked up
// live.
type ProfileSettings struct {
	Privacy             map[string]string
	Logs                map[string]string
	DNSSEC              map[string]string
	RebindingProtection map[string]string
	Advanced            map[string]string
	Statistics          map[string]string

	// Blocklists is the subscribed blocklist IDs in subscription order.
	Blocklists []string
	// Services is the blocked service IDs.
	Services []string
	// CustomRules is every custom rule hash of the profile; each map is the
	// rule's Redis hash (value, action, syntax, ...).
	CustomRules []map[string]string

	// Per-key errors (nil means success). An empty hash wraps
	// cache.ErrSettingsNotFound; anything else is a store failure. Lists and
	// sets are never "not found": a missing key reads as empty.
	PrivacyErr             error
	LogsErr                error
	DNSSECErr              error
	RebindingProtectionErr error
	AdvancedErr            error
	StatisticsErr          error
	BlocklistsErr          error
	ServicesErr            error
	CustomRulesErr         error
}

// StoreError returns the first per-key error that is a store failure rather
// than an absent hash, or nil when every group was read (present or not).
// Privacy is excluded: its absence is the profile-existence signal and is
// handled by the caller.
func (s *ProfileSettings) StoreError() error {
	for _, err := range []error{s.LogsErr, s.DNSSECErr, s.RebindingProtectionErr, s.AdvancedErr, s.StatisticsErr, s.BlocklistsErr, s.ServicesErr, s.CustomRulesErr} {
		if err != nil && !errors.Is(err, ErrSettingsNotFound) {
			return err
		}
	}
	return nil
}
