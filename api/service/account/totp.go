package account

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"

	dbErrors "github.com/ivpn/dns/api/db/errors"
	"github.com/ivpn/dns/api/internal/totpcrypt"
	"github.com/ivpn/dns/api/internal/utils"
	"github.com/ivpn/dns/api/model"
	"github.com/pquerna/otp/totp"
	"github.com/rs/zerolog/log"
)

const (
	backupCodeCount = 8
	// 25 characters over the 32-symbol uppercase alphabet is 125 bits, above the
	// 112-bit floor for look-up secrets stored with a plain hash
	// (NIST SP 800-63B-4 §3.1.2.2).
	backupCodeLength = 25
	// Stored codes may also be 8, 16 or 20 mixed-case characters long.
	minBackupCodeLength = 8
	// Longest 2FA input looked at: a grouped backup code with stray whitespace.
	maxCodeInputLength = 64
)

// errTOTPStorageUnavailable means the service has no usable TOTP storage key.
var errTOTPStorageUnavailable = errors.New("totp storage key unavailable")

// errTOTPSecretMissing means TOTP is enabled but no secret is stored.
var errTOTPSecretMissing = errors.New("totp enabled without a stored secret")

func (a *AccountService) keeper() (*totpcrypt.Keeper, error) {
	if a.totpKeeper == nil {
		return nil, errTOTPStorageUnavailable
	}
	return a.totpKeeper, nil
}

// TotpEnable enables TOTP for the given account
func (a *AccountService) TotpEnable(ctx context.Context, accountId string) (*model.TOTPNew, error) {
	keeper, err := a.keeper()
	if err != nil {
		return nil, err
	}
	acc, err := a.AccountRepository.GetAccountById(ctx, accountId)
	if err != nil {
		return nil, err
	}
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "modDNS",
		AccountName: acc.Email,
		SecretSize:  20, // Use 20 bytes (160 bits) - RFC 4226 minimum, generates clean 32-char base32 without padding
	})
	if err != nil {
		return nil, err
	}

	log.Ctx(ctx).Trace().Msg("TOTP key created")

	sealed, err := keeper.SealPendingSecret(acc.ID.Hex(), key.Secret())
	if err != nil {
		return nil, err
	}
	if err = a.Cache.SetTOTPSecret(ctx, accountId, sealed, a.ServiceCfg.OTPExpirationTime); err != nil {
		return nil, err
	}

	log.Ctx(ctx).Info().Msg("TOTP setup initiated")

	return &model.TOTPNew{
		Secret:  key.Secret(),
		URI:     key.URL(),
		Account: acc.Email,
	}, nil
}

// TotpConfirm confirms the TOTP code sent by the user
func (a *AccountService) TotpConfirm(ctx context.Context, accountId, otp string) (*model.TOTPBackup, error) {
	keeper, err := a.keeper()
	if err != nil {
		return nil, err
	}
	pending, err := a.Cache.GetTOTPSecret(ctx, accountId)
	if err != nil {
		return nil, err
	}

	acc, err := a.AccountRepository.GetAccountById(ctx, accountId)
	if err != nil {
		return nil, err
	}
	if acc.MFA.TOTP.Enabled {
		log.Ctx(ctx).Warn().Msg("TOTP already enabled")
		return nil, ErrTOTPAlreadyConfigured
	}

	limiter := a.totpLimiter(accountId, "totp_confirm_fails")
	allowed, err := limiter.Attempt()
	if err != nil {
		return nil, err
	}
	if !allowed {
		log.Ctx(ctx).Warn().Msg("TOTP confirmation: too many failed attempts")
		return nil, ErrTOTPTooManyAttempts
	}

	secret := pending
	if totpcrypt.IsSealed(pending) {
		if secret, err = keeper.OpenPendingSecret(acc.ID.Hex(), pending); err != nil {
			log.Ctx(ctx).Error().Err(err).Msg("TOTP confirm: cannot open pending secret")
			return nil, fmt.Errorf("open pending totp secret: %w", err)
		}
	}

	if !totp.Validate(normalizeCode(otp), secret) {
		log.Ctx(ctx).Info().Msg("Invalid TOTP code")
		return nil, ErrIncorrectOTP
	}

	codes := make([]string, 0, backupCodeCount)
	hashes := make([]string, 0, backupCodeCount)
	for range backupCodeCount {
		code, err := utils.RandomString(backupCodeLength, utils.AlphaNumericUserFriendlyUppercase)
		if err != nil {
			return nil, err
		}
		codes = append(codes, code)
		hashes = append(hashes, keeper.HashBackupCode(acc.ID.Hex(), code))
	}

	sealed, err := keeper.SealSecret(acc.ID.Hex(), secret)
	if err != nil {
		return nil, err
	}
	if err = a.AccountRepository.EnableTOTP(ctx, acc.ID.Hex(), sealed, hashes); err != nil {
		if errors.Is(err, dbErrors.ErrTOTPStateChanged) {
			log.Ctx(ctx).Warn().Msg("TOTP enabled concurrently")
			return nil, ErrTOTPAlreadyConfigured
		}
		return nil, err
	}

	log.Ctx(ctx).Info().Msg("TOTP enabled successfully, backup codes generated")
	// The pending secret is single-use; its TTL is only the fallback.
	_ = a.Cache.DeleteTOTPSecret(ctx, accountId)
	_ = limiter.Reset()

	return &model.TOTPBackup{BackupCodes: codes}, nil
}

func (a *AccountService) TotpDisable(ctx context.Context, accountId, otp string) (*model.Account, error) {
	acc, err := a.VerifyTotp(ctx, accountId, otp, "disable")
	if err != nil {
		return nil, err
	}

	if err = a.AccountRepository.DisableTOTP(ctx, acc.ID.Hex()); err != nil {
		return nil, err
	}
	acc.MFA.TOTP = model.TotpSettings{Enabled: false}

	log.Ctx(ctx).Info().Msg("TOTP disabled successfully")

	return acc, nil
}

func (a *AccountService) VerifyTotp(ctx context.Context, accountId, otp, action string) (*model.Account, error) {
	acc, err := a.AccountRepository.GetAccountById(ctx, accountId)
	if err != nil {
		return nil, err
	}
	switch action {
	case "login":
		if !acc.MFA.TOTP.Enabled {
			log.Ctx(ctx).Warn().Msg("TOTP is not configured")
			return nil, ErrTOTPNotConfigured
		}
	case "disable":
		if !acc.MFA.TOTP.Enabled {
			log.Ctx(ctx).Warn().Msg("TOTP already disabled")
			return nil, ErrTOTPAlreadyDisabled
		}
	default:
		return nil, errors.New("verify totp: invalid action")
	}

	// Each attempt is counted before any code is matched; past the limit every
	// code is refused and nothing is consumed (NIST SP 800-63B-4 §3.2.2,
	// RFC 6238 §5.2). A success clears the count.
	limiter := a.totpLimiter(acc.ID.Hex(), "totp_fails")
	allowed, err := limiter.Attempt()
	if err != nil {
		return nil, err
	}
	if !allowed {
		log.Ctx(ctx).Warn().Msgf("2FA %s: too many failed attempts", action)
		return nil, ErrTOTPTooManyAttempts
	}

	if len(otp) > maxCodeInputLength {
		return nil, ErrInvalidTOTPCode
	}
	otp = normalizeCode(otp)

	secret, err := a.storedTotpSecret(acc)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("2FA: cannot read stored TOTP secret")
		return nil, err
	}

	if totp.Validate(otp, secret) {
		log.Ctx(ctx).Trace().Msg("2FA verification successful")
		_ = limiter.Reset()
		return acc, nil
	}
	log.Ctx(ctx).Debug().Msgf("2FA %s: invalid TOTP code", action)

	ok, err := a.consumeBackupCode(ctx, acc, otp)
	var codeErr *TOTPError
	if err != nil && !errors.As(err, &codeErr) {
		log.Ctx(ctx).Err(err).Msg("error consuming TOTP backup code")
		return nil, err
	}
	if ok {
		log.Ctx(ctx).Info().Msg("TOTP backup code consumed successfully")
		_ = limiter.Reset()
		return acc, nil
	}

	if codeErr == nil {
		return nil, ErrInvalidTOTPCode
	}
	return nil, codeErr
}

func (a *AccountService) totpLimiter(accountID, label string) utils.IDLimiter {
	return utils.IDLimiter{
		ID:    accountID,
		Label: label,
		Max:   a.ServiceCfg.IdLimiterMax,
		Exp:   a.ServiceCfg.IdLimiterExpiration,
		Cache: a.Cache,
	}
}

// storedTotpSecret returns the TOTP secret of an enabled account: decrypted,
// or the plaintext field until the TOTP storage migration has converted it.
// An enabled account without a secret is an error: RFC 6238 §4 requires a
// non-empty shared secret.
func (a *AccountService) storedTotpSecret(acc *model.Account) (string, error) {
	t := acc.MFA.TOTP
	if t.SecretSealed == "" {
		if t.Secret == "" {
			return "", errTOTPSecretMissing
		}
		return t.Secret, nil
	}
	keeper, err := a.keeper()
	if err != nil {
		return "", err
	}
	return keeper.OpenSecret(acc.ID.Hex(), t.SecretSealed)
}

// consumeBackupCode uses up the backup code otp. A *TOTPError reports a code
// that is not, or no longer, valid; any other error is a storage failure.
func (a *AccountService) consumeBackupCode(ctx context.Context, acc *model.Account, otp string) (bool, error) {
	if len(otp) < minBackupCodeLength || len(otp) > backupCodeLength {
		return false, ErrInvalidTOTPCode
	}
	id := acc.ID.Hex()
	t := acc.MFA.TOTP

	if len(t.BackupCodes) > 0 {
		ok, err := a.AccountRepository.ConsumeLegacyBackupCode(ctx, id, otp)
		if err != nil || ok {
			return ok, err
		}
	}

	keeper, err := a.keeper()
	if err != nil {
		return false, err
	}
	// Current codes are uppercase and accepted in any case. Codes of earlier
	// formats are mixed case and match only as typed.
	if !t.BackupCodesLegacy {
		otp = upperASCII(otp)
	}
	ok, err := a.AccountRepository.ConsumeBackupCode(ctx, id, keeper.HashBackupCode(id, otp))
	if err != nil || ok {
		return ok, err
	}
	log.Ctx(ctx).Warn().Msg("2FA: invalid backup code")
	return false, ErrInvalidTOTPCode
}

// normalizeCode drops the group separators of a displayed code ("123 456",
// "ABCDE-FGHJK-..."). Neither a hyphen nor whitespace is in either code alphabet.
func normalizeCode(otp string) string {
	return strings.Map(func(r rune) rune {
		if r == '-' || unicode.IsSpace(r) {
			return -1
		}
		return r
	}, otp)
}

// upperASCII folds only a-z, so no character outside the code alphabet can
// become one of its letters.
func upperASCII(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' {
			return r - ('a' - 'A')
		}
		return r
	}, s)
}
