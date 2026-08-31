package account

import (
	"context"
	"errors"

	"slices"

	"github.com/ivpn/dns/api/internal/utils"
	"github.com/ivpn/dns/api/model"
	"github.com/pquerna/otp/totp"
	"github.com/rs/zerolog/log"
)

// TotpEnable enables TOTP for the given account
func (a *AccountService) TotpEnable(ctx context.Context, accountId string) (*model.TOTPNew, error) {
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

	if err = a.Cache.SetTOTPSecret(ctx, accountId, key.Secret(), a.ServiceCfg.OTPExpirationTime); err != nil {
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
	secret, err := a.Cache.GetTOTPSecret(ctx, accountId)
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

	valid := totp.Validate(otp, secret)
	if !valid {
		log.Ctx(ctx).Info().Msg("Invalid TOTP code")

		// Rate limiting for TOTP confirmation attempts to prevent brute-force
		idLimiter := utils.IDLimiter{
			ID:    accountId,
			Label: "totp_confirm_fails",
			Max:   a.ServiceCfg.IdLimiterMax,
			Exp:   a.ServiceCfg.IdLimiterExpiration,
			Cache: a.Cache,
		}

		// Increment failed attempt counter
		err = idLimiter.Tick()
		if err != nil {
			log.Ctx(ctx).Err(err).Msg("error ticking ID limiter for TOTP confirm")
			return nil, err
		}

		// Check if rate limit exceeded
		if !idLimiter.IsAllowed() {
			log.Ctx(ctx).Error().Msg("TOTP confirmation: too many failed attempts")
			log.Ctx(ctx).Warn().Msg("TOTP confirmation rate limit exceeded")
			return nil, ErrIncorrectOTP
		}

		return nil, ErrIncorrectOTP
	}

	// generate backup codes
	for range 8 {
		acc.MFA.TOTP.BackupCodes = append(acc.MFA.TOTP.BackupCodes, utils.RandomString(16, utils.AlphaNumericUserFriendly))
	}

	acc.MFA.TOTP.Enabled = true
	acc.MFA.TOTP.Secret = secret
	_, err = a.AccountRepository.UpdateAccount(ctx, acc)
	if err != nil {
		// Don't return backup codes if database update fails
		return nil, err
	}

	log.Ctx(ctx).Info().Msg("TOTP enabled successfully, backup codes generated")

	return &model.TOTPBackup{
		BackupCodes: acc.MFA.TOTP.BackupCodes,
	}, nil
}

func (a *AccountService) TotpDisable(ctx context.Context, accountId, otp string) (*model.Account, error) {
	acc, err := a.VerifyTotp(ctx, accountId, otp, "disable")
	if err != nil {
		return nil, err
	}

	acc.MFA.TOTP.Enabled = false
	acc.MFA.TOTP.Secret = ""
	acc.MFA.TOTP.BackupCodes = []string{}
	acc.MFA.TOTP.BackupCodesUsed = []string{}
	acc, err = a.AccountRepository.UpdateAccount(ctx, acc)
	if err != nil {
		return nil, err
	}

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

	var backupErr error
	valid := totp.Validate(otp, acc.MFA.TOTP.Secret)
	if !valid {
		log.Ctx(ctx).Debug().Msgf("2FA %s: invalid TOTP code", action)
		valid, backupErr = a.verifyTotpBackup(acc, otp, action)
		if valid {
			// update the account in case backup code was used
			_, err = a.AccountRepository.UpdateAccount(ctx, acc)
			if err != nil {
				log.Ctx(ctx).Err(err).Msg("error updating account after TOTP backup code use")
				return nil, err
			}
			log.Ctx(ctx).Info().Msg("TOTP backup code consumed successfully")
			return acc, backupErr
		}
	}

	// Rate limiting for TOTP verification attempts
	// Apply rate limiting consistently for all failed attempts
	if !valid {
		idLimiter := utils.IDLimiter{
			ID:    acc.ID.Hex(),
			Label: "totp_fails",
			Max:   a.ServiceCfg.IdLimiterMax,
			Exp:   a.ServiceCfg.IdLimiterExpiration,
			Cache: a.Cache,
		}

		// Increment failed attempt counter
		err = idLimiter.Tick()
		if err != nil {
			log.Ctx(ctx).Err(err).Msg("error ticking ID limiter")
			return nil, err
		}

		// Check if rate limit exceeded
		if !idLimiter.IsAllowed() {
			log.Ctx(ctx).Error().Msg("error verifying TOTP: too many failed attempts")
			log.Ctx(ctx).Warn().Msg("TOTP verification rate limit exceeded")
			return nil, ErrInvalidTOTPCode
		}

		// Return the specific error if available (e.g., backup code already used)
		if backupErr != nil {
			return nil, backupErr
		}
		return nil, ErrInvalidTOTPCode
	}

	log.Ctx(ctx).Trace().Msg("2FA verification successful")
	return acc, nil
}

func (a *AccountService) verifyTotpBackup(acc *model.Account, otp, action string) (bool, error) {
	var backupCodeFound bool
	if slices.Contains(acc.MFA.TOTP.BackupCodesUsed, otp) {
		return false, ErrTotpBackupAlreadyUsed
	}

	for idx, code := range acc.MFA.TOTP.BackupCodes {
		if code == otp {
			acc.MFA.TOTP.BackupCodes = RemoveIndex(acc.MFA.TOTP.BackupCodes, idx)
			acc.MFA.TOTP.BackupCodesUsed = append(acc.MFA.TOTP.BackupCodesUsed, code)
			backupCodeFound = true
			break
		}
	}
	if !backupCodeFound {
		log.Warn().Msgf("2FA %s: invalid backup code", action)
		return false, ErrInvalidTOTPCode
	}
	return true, nil
}

func RemoveIndex(s []string, index int) []string {
	ret := make([]string, 0)
	ret = append(ret, s[:index]...)
	return append(ret, s[index+1:]...)
}
