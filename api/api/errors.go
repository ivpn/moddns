package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	dbErrors "github.com/ivpn/dns/api/db/errors"
	"github.com/ivpn/dns/api/internal/reauth"
	"github.com/ivpn/dns/api/model"
	"github.com/ivpn/dns/api/service"
	"github.com/ivpn/dns/api/service/account"
	"github.com/ivpn/dns/api/service/passkey"
	"github.com/ivpn/dns/api/service/profile"
	"github.com/ivpn/dns/api/service/subscription"
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
	ErrFailedToUpdateCustomRule   = errors.New("failed to update custom rule")
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

// validationErrorPrefix is prepended to every user-facing validation/bad-body
// message so the customer immediately understands the 400 is about their input
// (e.g. "Validation error: customRules must be at most 1000").
const validationErrorPrefix = "Validation error: "

// badRequestErrorText picks the response error text for a 400. For
// ErrInvalidRequestBody the caller passes the human-readable validation detail
// as errMsg (e.g. "customRules must be at most 1000"), which is preferred over
// the generic sentinel so the frontend can display something useful — prefixed
// with "Validation error: ". Other sentinel errors carry a server-side log
// message in errMsg, so they fall back to err.Error().
func badRequestErrorText(err error, errMsg string) string {
	if err == ErrInvalidRequestBody && errMsg != "" && errMsg != ErrInvalidRequestBody.Error() {
		return validationErrorPrefix + errMsg
	}
	return err.Error()
}

// HumanizeDecodeError turns a JSON-decode failure (from a strict
// DisallowUnknownFields decoder) into a user-facing message instead of leaking
// the raw encoding/json error text (e.g. `json: unknown field "_id"`).
func HumanizeDecodeError(err error) string {
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		field := typeErr.Field
		if i := strings.LastIndex(field, "."); i >= 0 {
			field = field[i+1:]
		}
		if field == "" {
			return fmt.Sprintf("A field has the wrong type (expected %s).", friendlyJSONType(typeErr.Type.String()))
		}
		return fmt.Sprintf("Field '%s' has the wrong type (expected %s).", field, friendlyJSONType(typeErr.Type.String()))
	}
	const unknownPrefix = "json: unknown field "
	if msg := err.Error(); strings.HasPrefix(msg, unknownPrefix) {
		field := strings.Trim(strings.TrimPrefix(msg, unknownPrefix), `"`)
		return fmt.Sprintf("Unknown field '%s' is not allowed.", field)
	}
	return "The file is not valid JSON."
}

// friendlyJSONType maps a Go type name to a user-facing description.
func friendlyJSONType(goType string) string {
	switch goType {
	case "bool":
		return "true or false"
	case "string":
		return "text"
	case "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64",
		"float32", "float64":
		return "a number"
	default:
		return "a different type"
	}
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

	// Unified reauth-credential failures map to 400. Checked BEFORE the
	// *ServiceAccountError type switch because the reauth sentinels (plus the
	// aliased ErrInvalidCurrentPassword) are plain errors.New(...) values and
	// would otherwise fall through to the default 500. Order also matters
	// against the lower switch: ErrInvalidCurrentPassword is aliased to
	// reauth.ErrInvalidPassword, so this single check covers both names.
	//
	// Rationale for 400 (not 401): the caller's session is valid — what failed
	// is a piece of the request body (wrong password, wrong reauth_token, or a
	// missing/wrong x-mfa-code). Returning 401 would trigger the global axios
	// interceptor's session-expired logout flow, which is the wrong UX for a
	// typo'd password. 400 lets the dialog surface the error inline.
	if errors.Is(err, reauth.ErrMissingAuthMethod) ||
		errors.Is(err, reauth.ErrMultipleAuthMethods) ||
		errors.Is(err, reauth.ErrInvalidPassword) ||
		errors.Is(err, reauth.ErrInvalidReauthToken) ||
		errors.Is(err, reauth.ErrReauthTokenExpired) {
		resp.Error = err.Error()
		return c.Status(400).JSON(resp)
	}

	{
		var profileErr *profile.ProfileError
		if errors.As(err, &profileErr) {
			log.Debug().Str("code", profileErr.Code).Msg(err.Error())
			resp.Error = err.Error()
			return c.Status(400).JSON(resp)
		}
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
	case ErrInvalidRequestBody, model.ErrInvalidCustomRuleAction, account.ErrEmailAlreadyVerified, account.ErrPasswordTooSimple, account.ErrEmailNotVerified, account.ErrInvalidVerificationToken, account.ErrTokenExpired, account.ErrPasswordsDoNotMatch, profile.ErrProfileNameAlreadyExists, model.ErrInvalidRetention, profile.ErrProfileNameCannotBeEmpty, profile.ErrDefaultRuleInvalid, profile.ErrBlocklistNotFound, profile.ErrProfileNameEmpty, profile.ErrCustomRuleAlreadyExists, ErrInvalidCustomRuleSyntax, model.ErrInvalidCustomRuleSyntax, profile.ErrLastProfileInAccount, profile.ErrMaxProfilesLimitReached, profile.ErrInvalidServiceValue, profile.ErrServiceAlreadyEnabled:
		resp.Error = badRequestErrorText(err, errMsg)
		return c.Status(400).JSON(resp)
	case subscription.ErrSubscriptionScheduledForDeletion:
		resp.Error = err.Error()
		return c.Status(409).JSON(resp)
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
