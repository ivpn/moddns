package account

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"errors"
	"strings"
	"time"

	dbErrors "github.com/ivpn/dns/api/db/errors"
	"github.com/ivpn/dns/api/internal/auth"
	"github.com/ivpn/dns/api/internal/utils"
	"github.com/ivpn/dns/api/model"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"github.com/rs/zerolog/log"
)

const (
	emailVerificationOTPLength  = 6
	emailVerificationOTPExpires = 15 * time.Minute
)

// RequestEmailVerificationOTP generates and sends a new email verification OTP.
func (a *AccountService) RequestEmailVerificationOTP(ctx context.Context, accountId string) error {
	acc, err := a.AccountRepository.GetAccountById(ctx, accountId)
	if err != nil {
		return err
	}
	if acc.EmailVerified {
		return ErrEmailAlreadyVerified
	}
	// Rate limiting (resend)
	idLimiter := utils.IDLimiter{Cache: a.Cache, ID: acc.ID.Hex(), Label: "email_verify_resend", Max: a.ServiceCfg.IdLimiterMax, Exp: a.ServiceCfg.IdLimiterExpiration}
	if err := idLimiter.Tick(); err != nil {
		return err
	}
	if !idLimiter.IsAllowed() {
		return ErrEmailOTPRateLimited
	}

	// Remove existing OTP tokens
	filtered := make([]model.Token, 0, len(acc.Tokens))
	for _, t := range acc.Tokens {
		if t.Type != "email_verification_otp" {
			filtered = append(filtered, t)
		}
	}

	// Generate a random secret for TOTP generation
	secretBytes := make([]byte, 20) // 160-bit secret
	if _, err := rand.Read(secretBytes); err != nil {
		return err
	}
	// Base32 encode (without padding) as required by TOTP library
	secret := strings.TrimRight(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(secretBytes), "=")
	// Use a custom period matching our expiry window (15 minutes) and 6 digits
	code, genErr := totp.GenerateCodeCustom(secret, time.Now(), totp.ValidateOpts{Period: uint(emailVerificationOTPExpires.Seconds()), Digits: otp.DigitsSix})
	if genErr != nil {
		return genErr
	}
	otp := code
	filtered = append(filtered, model.Token{Type: "email_verification_otp", Value: otp, ExpiresAt: time.Now().Add(emailVerificationOTPExpires)})
	acc.Tokens = filtered
	if _, err = a.AccountRepository.UpdateAccount(ctx, acc); err != nil {
		return err
	}

	if err := a.Mailer.SendEmailVerificationOTP(ctx, acc.Email, otp); err != nil {
		log.Err(err).Msg("Failed to send email verification OTP")
		return ErrSendOTP
	}

	return nil
}

// VerifyEmailOTP verifies the provided OTP and marks email as verified.
func (a *AccountService) VerifyEmailOTP(ctx context.Context, accountId, otp string) error {
	if otp == "" {
		return ErrEmailOTPMissing
	}
	acc, err := a.AccountRepository.GetAccountById(ctx, accountId)
	if err != nil {
		return err
	}
	if acc.EmailVerified {
		return nil
	}

	var target *model.Token
	var others []model.Token
	now := time.Now()
	for _, t := range acc.Tokens {
		if t.Type == "email_verification_otp" {
			if now.After(t.ExpiresAt) {
				continue
			}
			target = &t
			continue
		}
		others = append(others, t)
	}
	if target == nil {
		return ErrInvalidVerificationToken
	}
	if target.Value != otp {
		failLimiter := utils.IDLimiter{Cache: a.Cache, ID: acc.ID.Hex(), Label: "email_verify_fail", Max: a.ServiceCfg.IdLimiterMax, Exp: a.ServiceCfg.IdLimiterExpiration}
		if err := failLimiter.Tick(); err != nil {
			return err
		}
		if !failLimiter.IsAllowed() {
			return ErrEmailOTPManyAttempts
		}
		return ErrIncorrectOTP
	}
	acc.EmailVerified = true
	acc.Tokens = others
	if _, err = a.AccountRepository.UpdateAccount(ctx, acc); err != nil {
		if errors.Is(err, dbErrors.ErrAccountNotFound) {
			return ErrInvalidVerificationToken
		}
		return err
	}
	return nil
}

// VerifyPasswordReset checks if the password reset token is valid and updates the password
func (a *AccountService) VerifyPasswordReset(ctx context.Context, tokenValue, newPassword string, mfa *model.MfaData) error {
	acc, err := a.AccountRepository.GetAccountByToken(ctx, tokenValue, auth.TokenTypePasswordReset)
	if err != nil {
		log.Warn().Err(err).Msg("error getting account by token")
		return ErrInvalidVerificationToken
	}
	if !acc.EmailVerified {
		log.Warn().Str("email", acc.Email).Msg("Email not verified")
	}
	if err := a.MfaCheck(ctx, acc, mfa); err != nil {
		return err
	}

	var others []model.Token
	var found bool
	for _, token := range acc.Tokens {
		if token.Type == auth.TokenTypePasswordReset && token.Value == tokenValue {
			if time.Now().After(token.ExpiresAt) {
				return ErrTokenExpired
			}
			found = true
			continue // exclude the used token
		}
		others = append(others, token)
	}
	if !found {
		return ErrInvalidVerificationToken
	}

	if err := acc.SetPassword(newPassword); err != nil {
		return err
	}
	acc.Tokens = others
	_, err = a.AccountRepository.UpdateAccount(ctx, acc)
	if err != nil {
		return err
	}

	return nil
}
