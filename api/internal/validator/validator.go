package validator

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/ivpn/dns/libs/deviceid"
	"github.com/rs/zerolog/log"
)

// Pre-compiled regexes for password validation (avoid re-compilation on every call).
var (
	reUppercase   = regexp.MustCompile(`[A-Z]`)
	reLowercase   = regexp.MustCompile(`[a-z]`)
	reNumber      = regexp.MustCompile(`[0-9]`)
	reSpecialChar = regexp.MustCompile(`[!@#$%^&*(),;.?":{}\[\]|<>_-]`)
)

const (
	// TODO: implement IPv4 and IPv6 wildcard patterns
	// IPv4WildcardRegex = `^(\*|[0-9]+)\.(\*|[0-9]+)\.(\*|[0-9]+)\.(\*|[0-9]+)$`
	// IPv6WildcardRegex = `^(\*|[0-9a-fA-F:]+)(:\*|:[0-9a-fA-F]+)*$`
	FQDNWildcardRegex     = `^[a-zA-Z0-9-]*\*[a-zA-Z0-9-]*(\.[a-zA-Z0-9][-a-zA-Z0-9]*)*$`
	SuffixWildcardRegex   = `^[a-zA-Z0-9]([-a-zA-Z0-9]*[a-zA-Z0-9])?(\.[a-zA-Z0-9]([-a-zA-Z0-9]*[a-zA-Z0-9])?)*\.\*$`
	ContainsWildcardRegex = `^\*[a-zA-Z0-9][-a-zA-Z0-9.]*[a-zA-Z0-9]\*$`
)

var wildcardPatterns = map[string]string{
	// "ipv4": IPv4WildcardRegex,
	// "ipv6": IPv6WildcardRegex,
	"fqdn":     FQDNWildcardRegex,
	"suffix":   SuffixWildcardRegex,
	"contains": ContainsWildcardRegex,
}

type ErrorResponse struct {
	Error       bool
	FailedField string
	Tag         string
	Value       any
}

type APIValidator struct {
	Validator       *validator.Validate
	WildcardRegexes map[string]*regexp.Regexp
}

func NewAPIValidator() (*APIValidator, error) {
	validate := validator.New(validator.WithRequiredStructEnabled())
	apiValidator := &APIValidator{
		Validator:       validate,
		WildcardRegexes: make(map[string]*regexp.Regexp),
	}
	err := apiValidator.Validator.RegisterValidation("password", passwordValidation)
	if err != nil {
		return nil, err
	}
	err = apiValidator.Validator.RegisterValidation("fqdn_wildcard", apiValidator.wildcardValidation)
	if err != nil {
		return nil, err
	}
	err = apiValidator.Validator.RegisterValidation("asn", asnValidation)
	if err != nil {
		return nil, err
	}
	err = apiValidator.Validator.RegisterValidation("device_id", deviceIDValidation)
	if err != nil {
		return nil, err
	}
	for key, pattern := range wildcardPatterns {
		compiled, err := regexp.Compile(pattern)
		if err != nil {
			log.Error().Err(err).Msg("Error compiling pattern")
			return nil, err
		}
		apiValidator.WildcardRegexes[key] = compiled
	}
	return apiValidator, nil
}

func (v APIValidator) Validate(data any) []ErrorResponse {
	validationErrors := []ErrorResponse{}

	errs := v.Validator.Struct(data)
	if errs != nil {
		for _, err := range errs.(validator.ValidationErrors) {
			var elem ErrorResponse

			elem.FailedField = err.Field() // Export struct field name
			elem.Tag = err.Tag()           // Export struct tag
			elem.Value = err.Value()       // Export field value
			elem.Error = true

			validationErrors = append(validationErrors, elem)
		}
	}

	return validationErrors
}

// ValidateRequest validates the request payload according to the provided struct tags
func (v APIValidator) ValidateRequest(c *fiber.Ctx, payload any, errMsg string) []string {
	errMsgs := make([]string, 0)
	if errs := v.Validate(payload); len(errs) > 0 && errs[0].Error {

		for _, err := range errs {
			validationErr := fmt.Sprintf(
				"[%s]: Needs to implement '%s'",
				err.FailedField,
				err.Tag,
			)
			log.Error().Str("path", c.Route().Path).Str("tag", err.Tag).Err(errors.New("validation error")).Msg(validationErr)
			errMsgs = append(errMsgs, validationErr)
		}

		return errMsgs
	}
	return errMsgs
}

// Custom validation function for wildcard patterns in FQDN and IP addresses
func (v APIValidator) wildcardValidation(fl validator.FieldLevel) bool {
	value := fl.Field().String()

	// Support leading dot syntax by treating ".example.com" as "*.example.com" for validation
	if !strings.Contains(value, "*") && strings.HasPrefix(value, ".") {
		value = "*" + value
	}

	// If still no wildcard, skip validation (handled by other validators)
	if !strings.Contains(value, "*") {
		return false
	}

	// Check if the value matches any of the wildcard patterns
	for _, re := range v.WildcardRegexes {
		matched := re.MatchString(value)
		if matched {
			return true
		}
	}

	return false
}

func asnValidation(fl validator.FieldLevel) bool {
	value := strings.TrimSpace(fl.Field().String())
	if value == "" {
		return false
	}

	upper := strings.ToUpper(value)
	if strings.HasPrefix(upper, "AS") {
		if len(value) < 2 {
			return false
		}
		value = strings.TrimSpace(value[2:])
	}

	if value == "" {
		return false
	}

	parsed, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		return false
	}
	return parsed > 0
}

// passwordValidation validates the password according to the complexity criteria
func passwordValidation(fl validator.FieldLevel) bool {
	return ValidatePassword(fl.Field().String())
}

func ValidatePassword(password string) bool {
	if len(password) < 12 || len(password) > 64 {
		return false
	}

	if !reUppercase.MatchString(password) {
		return false
	}

	if !reLowercase.MatchString(password) {
		return false
	}

	if !reNumber.MatchString(password) {
		return false
	}

	if !reSpecialChar.MatchString(password) {
		return false
	}

	return true
}

// deviceIDValidation validates device identifiers using the shared deviceid package.
// It passes if the raw value equals its normalized form (only [A-Za-z0-9 -], max length).
func deviceIDValidation(fl validator.FieldLevel) bool {
	value := fl.Field().String()
	return value == deviceid.Normalize(value)
}
