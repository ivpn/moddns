package api

import (
	"errors"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	dbErrors "github.com/ivpn/dns/api/db/errors"
	"github.com/ivpn/dns/api/model"
	"github.com/ivpn/dns/api/service"
	"github.com/ivpn/dns/api/service/account"
	"github.com/ivpn/dns/api/service/passkey"
	"github.com/ivpn/dns/api/service/profile"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/mongo"
)

var (
	ErrInvalidRequestBody         = errors.New("invalid request")
	ErrValidationFailed           = errors.New("validation failed")
	ErrFailedToRegisterAccount    = errors.New("failed to register account")
	ErrFailedToUpdateAccount      = errors.New("failed to update account")
	ErrFailedToDeleteAccount      = errors.New("failed to delete account")
	ErrFailedToCreateCustomRule   = errors.New("failed to create custom rule")
	ErrFailedToDeleteCustomRule   = errors.New("failed to delete custom rule")
	ErrFailedToEnableBlocklists   = errors.New("failed to enable blocklists")
	ErrFailedToDisableBlocklists  = errors.New("failed to disable blocklists")
	ErrFailedToEnableServices     = errors.New("failed to enable services")
	ErrFailedToDisableServices    = errors.New("failed to disable services")
	ErrInvalidBlocklistValue      = errors.New("invalid blocklist value")
	ErrInvalidServiceValue        = errors.New("invalid service value")
	ErrResourceNotFound           = errors.New("resource not found")
	ErrFailedToCreateProfile      = errors.New("failed to create profile")
	ErrFailedToUpdateProfile      = errors.New("failed to update profile")
	ErrFailedToGetSubscription    = errors.New("failed to get subscription data")
	ErrFailedToUpdateSubscription = errors.New("failed to update subscription")
	ErrFailedToGetQueryLogs       = errors.New("failed to get profile query logs")
	ErrFailedToGetStatistics      = errors.New("failed to get profile statistics")
	ErrFailedToDeleteQueryLogs    = errors.New("failed to delete profile query logs")
	ErrFailedToGetAccount         = errors.New("failed to get account data")
	ErrFailedToVerifyEmail        = errors.New("failed to verify email")
	ErrEmailVerificationRequired  = errors.New("email address not verified")
	ErrFailedToAddBlocklist       = errors.New("failed to add blocklist")
	ErrBlocklistAlreadyExists     = errors.New("blocklist with this ID already exists")
	ErrEmailAlreadyExists         = errors.New("Unable to complete your request. Please try a different email address.")
	ErrTooManyDetails             = errors.New("too many details passed to HandleError function")
	ErrFailedToEnable2FA          = errors.New("failed to enable 2FA")
	ErrFailedToConfirm2FA         = errors.New("failed to confirm 2FA")
	ErrFailedToDisable2FA         = errors.New("failed to disable 2FA")
	ErrDisableTotpSuccess         = errors.New("2FA is disabled")
	// ErrTotpRequired                 = errors.New("TOTP is required")
	ErrInvalidTotpCode              = errors.New("invalid 2FA code")
	ErrInvalidCustomRuleSyntax      = errors.New("the rule needs to be a valid domain name, IPv4 or IPv6 address, or ASN")
	ErrFailedToGenerateMobileConfig = errors.New("failed to generate .mobileconfig")
	ErrFailedToGenerateDNSStamp     = errors.New("failed to generate DNS stamp")
	ErrGetSession                   = errors.New("could not get session")
	ErrSaveSession                  = errors.New("could not save session")
	ErrDeleteSession                = errors.New("could not delete session")
	ErrSessionsLimitReached         = errors.New("maximum number of active sessions reached")
	// WebAuthn specific errors
	ErrUnauthorized           = errors.New("unauthorized")
	ErrWebAuthnNotImplemented = errors.New("webauthn feature not fully implemented")
	ErrWebAuthnUnavailable    = errors.New("webauthn service unavailable")
)

type ErrResponse struct {
	Error   string   `json:"error"`
	Details []string `json:"details,omitempty"`
}

func HandleError(c *fiber.Ctx, err error, errMsg string, details ...string) error {
	log.Error().Err(err).Msg(errMsg)
	resp := new(ErrResponse)

	if len(details) > 0 {
		resp.Details = details
	}
	if errors.Is(err, strconv.ErrSyntax) {
		resp.Error = err.Error()
		return c.Status(400).JSON(resp)
	}

	if e, ok := err.(mongo.WriteException); ok {
		for _, we := range e.WriteErrors {
			if we.Code == 11000 {
				msg := we.Message
				switch {
				case strings.Contains(msg, "index: email"):
					// Duplicate account email
					resp.Error = ErrEmailAlreadyExists.Error()
				case strings.Contains(msg, "blocklist") || strings.Contains(msg, "blocklists"):
					// Duplicate blocklist id/name
					resp.Error = ErrBlocklistAlreadyExists.Error()
				default:
					// Generic duplicate key fallback
					resp.Error = "duplicate key error"
				}
				return c.Status(400).JSON(resp)
			}
		}
	}

	switch e := err.(type) {
	case *account.TOTPError:
		log.Printf("2FA Error: %s, Code: %s", e.Message, e.Code)
		resp.Error = err.Error()
		return c.Status(401).JSON(resp)
	case *account.ServiceAccountError:
		log.Printf("Service Account Error: %s, Code: %s", e.Message, e.Code)
		if errors.Is(err, account.ErrEmailOTPRateLimited) || errors.Is(err, account.ErrReauthRateLimited) {
			resp.Error = err.Error()
			return c.Status(429).JSON(resp)
		}
		resp.Error = err.Error()
		return c.Status(400).JSON(resp)
	case *passkey.PasskeyError:
		log.Printf("Passkey Error: %s, Code: %s", e.Message, e.Code)
		resp.Error = err.Error()
		return c.Status(400).JSON(resp)
	case *service.CredentialError:
		log.Printf("Credential Error: %s, Code: %s", e.Message, e.Code)
		resp.Error = e.Error()
		return c.Status(400).JSON(resp)
	}

	switch err {
	case ErrValidationFailed:
		resp.Error = err.Error()
		resp.Details = details
		return c.Status(400).JSON(resp)
	case account.ErrAccountAlreadyExists:
		errMsg := "Account with given email address already exists"
		resp.Error = errMsg
		return c.Status(400).JSON(resp)
	case dbErrors.ErrAccountNotFound, account.ErrAccountIdMissing, dbErrors.ErrProfileNotFound, dbErrors.ErrCustomRuleNotFound, dbErrors.ErrSubscriptionNotFound:
		resp.Error = ErrResourceNotFound.Error()
		return c.Status(404).JSON(resp)
	case ErrInvalidRequestBody, model.ErrInvalidCustomRuleAction, account.ErrEmailAlreadyVerified, account.ErrPasswordTooSimple, account.ErrEmailNotVerified, account.ErrInvalidVerificationToken, account.ErrTokenExpired, account.ErrPasswordsDoNotMatch, profile.ErrProfileNameAlreadyExists, model.ErrInvalidRetention, profile.ErrProfileNameCannotBeEmpty, profile.ErrDefaultRuleInvalid, profile.ErrBlocklistNotFound, profile.ErrProfileNameEmpty, profile.ErrCustomRuleAlreadyExists, ErrInvalidCustomRuleSyntax, profile.ErrLastProfileInAccount, profile.ErrMaxProfilesLimitReached, profile.ErrInvalidServiceValue, profile.ErrServiceAlreadyEnabled:
		resp.Error = err.Error()
		return c.Status(400).JSON(resp)
	case ErrSessionsLimitReached:
		resp.Error = err.Error()
		return c.Status(429).JSON(resp)
	case ErrUnauthorized:
		resp.Error = err.Error()
		return c.Status(401).JSON(resp)
	case profile.ErrQueryLogsRateLimited:
		resp.Error = err.Error()
		return c.Status(429).JSON(resp)
	default:
		resp.Error = errMsg
		return c.Status(500).JSON(resp)
	}
}
