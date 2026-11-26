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
	// BackupCodesLegacy marks a code set of an earlier format, which the
	// account holder should replace.
	BackupCodesLegacy bool `json:"backup_codes_legacy" bson:"backup_codes_legacy,omitempty"`
	// BackupCodesRemaining is the number of unused backup codes; derived, not stored.
	BackupCodesRemaining int `json:"backup_codes_remaining" bson:"-"`

	// Plaintext fields, read until the TOTP storage migration converts them.
	// Never written.
	Secret      string   `json:"-" bson:"secret,omitempty"`
	BackupCodes []string `json:"-" bson:"backup_codes,omitempty"`
}

// MarshalJSON fills the derived fields: a count of the unused backup codes,
// never the codes, and whether they are of an earlier format.
func (t TotpSettings) MarshalJSON() ([]byte, error) {
	type alias TotpSettings
	a := alias(t)
	a.BackupCodesRemaining = len(t.BackupCodeHashes) + len(t.BackupCodes)
	a.BackupCodesLegacy = t.BackupCodesLegacy || len(t.BackupCodes) > 0
	return json.Marshal(a)
}

// TOTPProof is the 2FA state a code was verified against. A write that the
// code authorises applies only while that state is unchanged.
type TOTPProof struct {
	// SecretSealed is the stored, sealed secret of the verified account.
	SecretSealed string
	// BackupCodeHash is the digest of the backup code used; empty for an
	// authenticator code. The authorised write uses the code up.
	BackupCodeHash string
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
