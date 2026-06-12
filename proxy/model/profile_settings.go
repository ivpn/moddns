package model

// ProfileSettings holds all profile settings fetched in a single batch.
type ProfileSettings struct {
	Privacy             map[string]string
	Logs                map[string]string
	DNSSEC              map[string]string
	RebindingProtection map[string]string
	Advanced            map[string]string

	// Per-key errors (nil means success). A missing key in Redis returns
	// an empty map (not an error), so these only fire on real Redis failures.
	PrivacyErr             error
	LogsErr                error
	DNSSECErr              error
	RebindingProtectionErr error
	AdvancedErr            error
}
