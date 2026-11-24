package model

import "encoding/json"

// MFASettings represents the settings for multi-factor authentication.
type MFASettings struct {
	TOTP TotpSettings `json:"totp" bson:"totp"`
}

// TotpSettings represents the settings for TOTP.
type TotpSettings struct {
	Enabled bool `json:"enabled" bson:"enabled"` // Indicates if TOTP is enabled.

	// SecretSealed is the TOTP secret encrypted with totpcrypt.Keeper.SealSecret.
	SecretSealed string `json:"-" bson:"secret_sealed,omitempty"`
	// BackupCodeHashes are the unused backup codes, as totpcrypt.Keeper.HashBackupCode digests.
	BackupCodeHashes []string `json:"-" bson:"backup_code_hashes,omitempty"`
	// BackupCodesLegacy marks a code set the account holder should replace.
	BackupCodesLegacy bool `json:"-" bson:"backup_codes_legacy,omitempty"`

	// Plaintext fields, read until the TOTP storage migration converts them.
	// Never written.
	Secret      string   `json:"-" bson:"secret,omitempty"`
	BackupCodes []string `json:"-" bson:"backup_codes,omitempty"`
}

type TOTPNew struct {
	Secret  string `json:"secret"` //nolint:gosec // G117 - intentional sensitive field
	Account string `json:"account"`
	URI     string `json:"uri"`
}

type TOTPBackup struct {
	BackupCodes []string `json:"backup_codes"`
}

// MarshalJSON renders BackupCodes as an empty JSON array ([]) instead of null
// when nil, so the API always returns a list for this field.
func (b TOTPBackup) MarshalJSON() ([]byte, error) {
	type alias TOTPBackup
	a := alias(b)
	if a.BackupCodes == nil {
		a.BackupCodes = []string{}
	}
	return json.Marshal(a)
}

// MfaData represents the data required for multi-factor authentication sent in HTTP headers.
type MfaData struct {
	OTP     string   `json:"otp"`
	Methods []string `json:"methods"`
}

// MarshalJSON renders Methods as an empty JSON array ([]) instead of null when
// nil, so the API always returns a list for this field.
func (m MfaData) MarshalJSON() ([]byte, error) {
	type alias MfaData
	a := alias(m)
	if a.Methods == nil {
		a.Methods = []string{}
	}
	return json.Marshal(a)
}
