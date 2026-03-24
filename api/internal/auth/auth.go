package auth

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/ivpn/dns/api/model"
	"github.com/spf13/cast"
)

func GetAccountID(c *fiber.Ctx) string {
	if c.Locals(ACCOUNT_ID) != nil {
		accId := cast.ToString(c.Locals(ACCOUNT_ID))
		return accId
	}
	return ""
}

func GetMfaCode(c *fiber.Ctx) string {
	return c.Get("x-mfa-code")
}

func GetMfaMethods(c *fiber.Ctx) string {
	return c.Get("x-mfa-methods")
}

func GetMfaData(c *fiber.Ctx) *model.MfaData {
	return &model.MfaData{
		OTP:     GetMfaCode(c),
		Methods: []string{GetMfaMethods(c)},
	}
}

func GetHeaderSessionsRemove(c *fiber.Ctx) bool {
	remove := c.Get("x-sessions-remove")
	if remove == "" {
		return false
	}
	return cast.ToBool(remove)
}

const (
	TokenTypePasswordReset           string        = "password_reset"          //nolint:gosec
	TokenTypeReauthEmailChange       string        = "reauth_email_change"     //nolint:gosec
	TokenTypeReauthAccountDeletion   string        = "reauth_account_deletion" //nolint:gosec
	passwordResetTokenLength         int           = 48
	passwordResetTokenExpiration     time.Duration = time.Hour
	reauthEmailChangeTokenLength     int           = 48
	reauthEmailChangeExpiration      time.Duration = time.Minute * 5
	reauthAccountDeletionTokenLength int           = 48
	reauthAccountDeletionExpiration  time.Duration = time.Minute * 5
)

// NewToken creates a new token and assigns it an expiration date to it
func NewToken(tokenType string) (*model.Token, error) {
	var expiresAt time.Time
	var tokenLen int
	switch tokenType {
	case TokenTypePasswordReset:
		expiresAt = time.Now().Add(passwordResetTokenExpiration)
		tokenLen = passwordResetTokenLength
	case TokenTypeReauthEmailChange:
		expiresAt = time.Now().Add(reauthEmailChangeExpiration)
		tokenLen = reauthEmailChangeTokenLength
	case TokenTypeReauthAccountDeletion:
		expiresAt = time.Now().Add(reauthAccountDeletionExpiration)
		tokenLen = reauthAccountDeletionTokenLength
	default:
		return nil, model.ErrInvalidTokenType
	}
	token, err := GenerateSecureToken(tokenLen)
	if err != nil {
		return nil, err
	}
	return &model.Token{
		Value:     token,
		Type:      tokenType,
		ExpiresAt: expiresAt,
	}, nil
}
