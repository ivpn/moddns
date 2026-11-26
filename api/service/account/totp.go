package account

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode"

	"github.com/ivpn/dns/api/api/requests"
	dbErrors "github.com/ivpn/dns/api/db/errors"
	"github.com/ivpn/dns/api/internal/auth"
	"github.com/ivpn/dns/api/internal/reauth"
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

// errTOTPRecordNotConverted means a 2FA change was requested on a record the
// TOTP storage migration has not converted.
var errTOTPRecordNotConverted = errors.New("totp record not converted to protected storage")

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

	codes, hashes, err := newBackupCodes(keeper, acc.ID.Hex())
	if err != nil {
		return nil, err
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

// TotpDisable turns 2FA off, authorised by reauth and then an authenticator or
// backup code.
func (a *AccountService) TotpDisable(ctx context.Context, accountId string, req requests.TotpDisableReq) (*model.Account, error) {
	if err := a.reauthTotpChange(ctx, accountId, auth.TokenTypeReauthTotpDisable, req.CurrentPassword, req.ReauthToken); err != nil {
		return nil, err
	}
	acc, proof, done, err := a.checkTotp(ctx, accountId, req.OTP, "disable")
	if err != nil {
		return nil, err
	}
	if err = a.applyVerifiedTotp(proof, func(p model.TOTPProof) error {
		return a.AccountRepository.DisableTOTP(ctx, acc.ID.Hex(), p)
	}); err != nil {
		return nil, err
	}
	done()
	acc.MFA.TOTP = model.TotpSettings{Enabled: false}

	log.Ctx(ctx).Info().Msg("TOTP disabled successfully")

	return acc, nil
}

// TotpRegenerateBackupCodes replaces every backup code with a new set,
// authorised by reauth and then an authenticator or backup code. The new codes
// are returned only here.
func (a *AccountService) TotpRegenerateBackupCodes(ctx context.Context, accountId string, req requests.TotpRegenerateReq) (*model.TOTPBackup, error) {
	keeper, err := a.keeper()
	if err != nil {
		return nil, err
	}
	if err = a.reauthTotpChange(ctx, accountId, auth.TokenTypeReauthBackupCodesRegen, req.CurrentPassword, req.ReauthToken); err != nil {
		return nil, err
	}
	acc, proof, done, err := a.checkTotp(ctx, accountId, req.OTP, "regenerate")
	if err != nil {
		return nil, err
	}
	codes, hashes, err := newBackupCodes(keeper, acc.ID.Hex())
	if err != nil {
		return nil, err
	}
	if err = a.applyVerifiedTotp(proof, func(p model.TOTPProof) error {
		return a.AccountRepository.ReplaceBackupCodes(ctx, acc.ID.Hex(), p, hashes)
	}); err != nil {
		return nil, err
	}
	done()

	log.Ctx(ctx).Info().Msg("TOTP backup codes regenerated")

	return &model.TOTPBackup{BackupCodes: codes}, nil
}

// reauthTotpChange proves fresh control of the account before a change to its
// second factor: the current password, or a single-use passkey reauth token of
// the change's own type (NIST SP 800-63B-4 §4). It runs before the 2FA code
// is checked, so a session alone neither tests codes nor counts toward the E12
// lockout. The code itself is checked by the caller and used up in its write.
func (a *AccountService) reauthTotpChange(ctx context.Context, accountId, tokenType string, password, token *string) error {
	_, err := reauth.Verify(ctx, a.AccountRepository, nil, reauth.Params{
		AccountId:        accountId,
		TokenType:        tokenType,
		Password:         password,
		ReauthToken:      token,
		PersistOnConsume: true,
	})
	return err
}

// VerifyTotp checks a 2FA code for login and the account checks that require
// one, and uses up a backup code.
func (a *AccountService) VerifyTotp(ctx context.Context, accountId, otp string) (*model.Account, error) {
	acc, proof, done, err := a.checkTotp(ctx, accountId, otp, "verification")
	if err != nil {
		return nil, err
	}
	if proof.legacyBackupCode != "" {
		ok, err := a.AccountRepository.ConsumeLegacyBackupCode(ctx, acc.ID.Hex(), proof.legacyBackupCode)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, ErrInvalidTOTPCode
		}
	}
	if proof.BackupCodeHash != "" {
		ok, err := a.AccountRepository.ConsumeBackupCode(ctx, acc.ID.Hex(), proof.BackupCodeHash)
		if err != nil {
			log.Ctx(ctx).Err(err).Msg("error consuming TOTP backup code")
			return nil, err
		}
		if !ok {
			return nil, ErrInvalidTOTPCode
		}
	}
	if proof.isBackupCode() {
		log.Ctx(ctx).Info().Msg("TOTP backup code consumed successfully")
	}
	done()
	return acc, nil
}

// totpCheck is a 2FA code that matched but whose backup code, if any, is not
// used up yet.
type totpCheck struct {
	model.TOTPProof
	// legacyBackupCode is a plaintext backup code of a record the storage
	// migration has not converted.
	legacyBackupCode string
}

func (c totpCheck) isBackupCode() bool {
	return c.BackupCodeHash != "" || c.legacyBackupCode != ""
}

// checkTotp verifies otp without using a backup code up; the caller does that
// in the write the code authorises, then calls done to clear the E12 count.
func (a *AccountService) checkTotp(ctx context.Context, accountId, otp, action string) (*model.Account, totpCheck, func(), error) {
	acc, err := a.AccountRepository.GetAccountById(ctx, accountId)
	if err != nil {
		return nil, totpCheck{}, nil, err
	}
	if !acc.MFA.TOTP.Enabled {
		switch action {
		case "disable":
			log.Ctx(ctx).Warn().Msg("TOTP already disabled")
			return nil, totpCheck{}, nil, ErrTOTPAlreadyDisabled
		default:
			log.Ctx(ctx).Warn().Msg("TOTP is not configured")
			return nil, totpCheck{}, nil, ErrTOTPNotConfigured
		}
	}

	// Each attempt is counted before any code is matched; past the limit every
	// code is refused and nothing is consumed (NIST SP 800-63B-4 §3.2.2,
	// RFC 4226 §7.3). A success clears the count.
	limiter := a.totpLimiter(acc.ID.Hex(), "totp_fails")
	allowed, err := limiter.Attempt()
	if err != nil {
		return nil, totpCheck{}, nil, err
	}
	if !allowed {
		log.Ctx(ctx).Warn().Msgf("2FA %s: too many failed attempts", action)
		return nil, totpCheck{}, nil, ErrTOTPTooManyAttempts
	}
	done := func() { _ = limiter.Reset() }

	if len(otp) > maxCodeInputLength {
		return nil, totpCheck{}, nil, ErrInvalidTOTPCode
	}
	otp = normalizeCode(otp)

	secret, err := a.storedTotpSecret(acc)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("2FA: cannot read stored TOTP secret")
		return nil, totpCheck{}, nil, err
	}
	check := totpCheck{TOTPProof: model.TOTPProof{SecretSealed: acc.MFA.TOTP.SecretSealed}}

	if totp.Validate(otp, secret) {
		log.Ctx(ctx).Trace().Msg("2FA verification successful")
		return acc, check, done, nil
	}
	log.Ctx(ctx).Debug().Msgf("2FA %s: invalid TOTP code", action)

	if len(otp) < minBackupCodeLength || len(otp) > backupCodeLength {
		return nil, totpCheck{}, nil, ErrInvalidTOTPCode
	}
	t := acc.MFA.TOTP
	if slices.Contains(t.BackupCodes, otp) {
		check.legacyBackupCode = otp
		return acc, check, done, nil
	}
	keeper, err := a.keeper()
	if err != nil {
		return nil, totpCheck{}, nil, err
	}
	// Current codes are uppercase and accepted in any case. Codes of earlier
	// formats are mixed case and match only as typed.
	if !t.BackupCodesLegacy {
		otp = upperASCII(otp)
	}
	if hash := keeper.HashBackupCode(acc.ID.Hex(), otp); slices.Contains(t.BackupCodeHashes, hash) {
		check.BackupCodeHash = hash
		return acc, check, done, nil
	}
	log.Ctx(ctx).Warn().Msg("2FA: invalid backup code")
	return nil, totpCheck{}, nil, ErrInvalidTOTPCode
}

// applyVerifiedTotp runs a write conditioned on the checked state; a state
// changed since the check means the code is no longer valid.
func (a *AccountService) applyVerifiedTotp(check totpCheck, write func(model.TOTPProof) error) error {
	if check.SecretSealed == "" || check.legacyBackupCode != "" {
		return errTOTPRecordNotConverted
	}
	err := write(check.TOTPProof)
	if errors.Is(err, dbErrors.ErrTOTPStateChanged) {
		return ErrInvalidTOTPCode
	}
	return err
}

// newBackupCodes draws a set of backup codes and their digests (E5).
func newBackupCodes(keeper *totpcrypt.Keeper, accountID string) ([]string, []string, error) {
	codes := make([]string, 0, backupCodeCount)
	hashes := make([]string, 0, backupCodeCount)
	for range backupCodeCount {
		code, err := utils.RandomString(backupCodeLength, utils.AlphaNumericUserFriendlyUppercase)
		if err != nil {
			return nil, nil, err
		}
		codes = append(codes, code)
		hashes = append(hashes, keeper.HashBackupCode(accountID, code))
	}
	return codes, hashes, nil
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
