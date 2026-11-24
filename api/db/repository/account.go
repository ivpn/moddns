package repository

import (
	"context"
	"time"

	"github.com/ivpn/dns/api/model"
)

// AccountRepository represents a Account repository
type AccountRepository interface {
	CreateAccount(ctx context.Context, email, password, accountId, profileId string) (*model.Account, error)
	UpdateAccount(ctx context.Context, account *model.Account) (*model.Account, error)
	GetAccountById(ctx context.Context, accountId string) (*model.Account, error)
	GetAccountByEmail(ctx context.Context, email string) (*model.Account, error)
	GetAccountByToken(ctx context.Context, token, tokenType string) (*model.Account, error)
	DeleteAccountById(ctx context.Context, accountId string) error
	UpdateDeletionCode(ctx context.Context, accountId string, code string, expiresAt time.Time) error
	GetAccount(ctx context.Context, accountId string) (*model.Account, error)
	AddProfileToAccount(ctx context.Context, accountId string, profileId string) error
	RemoveProfileFromAccount(ctx context.Context, accountId string, profileId string) error

	// EnableTOTP stores the sealed secret and backup-code digests and enables
	// TOTP; errors.ErrTOTPStateChanged when TOTP is already enabled.
	EnableTOTP(ctx context.Context, accountId, sealedSecret string, backupCodeHashes []string) error
	// DisableTOTP disables TOTP and removes every stored secret and backup code.
	DisableTOTP(ctx context.Context, accountId string) error
	// ConsumeBackupCode atomically moves a digest from unused to used; false
	// when the account has no such unused code.
	ConsumeBackupCode(ctx context.Context, accountId, backupCodeHash string) (bool, error)
	// ConsumeLegacyBackupCode is ConsumeBackupCode for plaintext codes of
	// accounts not yet converted by the TOTP storage migration.
	ConsumeLegacyBackupCode(ctx context.Context, accountId, backupCode string) (bool, error)
}
