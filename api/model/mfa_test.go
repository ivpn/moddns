package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// The account response carries the 2FA state the app needs and never a secret
// or a code digest.
// specRef: api-endpoint-behaviour.md E14
func TestTotpSettingsJSON(t *testing.T) {
	cases := map[string]struct {
		in   TotpSettings
		want string
	}{
		"off": {
			in:   TotpSettings{},
			want: `{"enabled":false,"backup_codes_remaining":0,"backup_codes_legacy":false}`,
		},
		"current codes": {
			in:   TotpSettings{Enabled: true, SecretSealed: "v1.kid.x", BackupCodeHashes: []string{"h1", "h2", "h3"}},
			want: `{"enabled":true,"backup_codes_remaining":3,"backup_codes_legacy":false}`,
		},
		"legacy codes": {
			in:   TotpSettings{Enabled: true, SecretSealed: "v1.kid.x", BackupCodeHashes: []string{"h1"}, BackupCodesLegacy: true},
			want: `{"enabled":true,"backup_codes_remaining":1,"backup_codes_legacy":true}`,
		},
		"plaintext record": {
			in:   TotpSettings{Enabled: true, Secret: "S", BackupCodes: []string{"c1", "c2"}},
			want: `{"enabled":true,"backup_codes_remaining":2,"backup_codes_legacy":true}`,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			b, err := json.Marshal(tc.in)
			require.NoError(t, err)
			require.JSONEq(t, tc.want, string(b))

			b, err = json.Marshal(Account{MFA: MFASettings{TOTP: tc.in}})
			require.NoError(t, err)
			var acc struct {
				MFA struct {
					TOTP json.RawMessage `json:"totp"`
				} `json:"mfa"`
			}
			require.NoError(t, json.Unmarshal(b, &acc))
			require.JSONEq(t, tc.want, string(acc.MFA.TOTP))
		})
	}
}
