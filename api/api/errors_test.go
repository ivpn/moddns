package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gofiber/fiber/v2"
	dbErrors "github.com/ivpn/dns/api/db/errors"
	"github.com/ivpn/dns/api/service/account"
	"github.com/ivpn/dns/api/service/profile"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/mongo"
)

func TestHandleError(t *testing.T) {
	tests := []struct {
		name            string
		err             error
		errMsg          string
		details         []string
		expectedDetails []string
		expectedStatus  int
		expectedError   string
	}{
		{
			name:           "Syntax Error",
			err:            strconv.ErrSyntax,
			errMsg:         "test error",
			expectedStatus: 400,
			expectedError:  strconv.ErrSyntax.Error(),
		},
		{
			name:           "Not Found Resources",
			err:            dbErrors.ErrAccountNotFound,
			errMsg:         "Resource not found",
			expectedStatus: 404,
			expectedError:  ErrResourceNotFound.Error(),
		},
		{
			name:           "Duplicate Blocklist (generic 11000)",
			err:            mongo.WriteException{WriteErrors: []mongo.WriteError{{Code: 11000, Message: "E11000 duplicate key error collection: dns.blocklists index: blocklist_id dup key: { blocklist_id: \"ads\" }"}}},
			errMsg:         "Duplicate blocklist entry",
			expectedStatus: 400,
			expectedError:  ErrBlocklistAlreadyExists.Error(),
		},
		{
			name:           "Duplicate Email Index",
			err:            mongo.WriteException{WriteErrors: []mongo.WriteError{{Code: 11000, Message: "E11000 duplicate key error collection: dns.accounts index: email dup key: { email: \"user@example.com\" }"}}},
			errMsg:         "Failed to update account",
			expectedStatus: 400,
			expectedError:  "Unable to complete your request. Please try a different email address.",
		},
		{
			name:           "Generic Duplicate Key Other Index",
			err:            mongo.WriteException{WriteErrors: []mongo.WriteError{{Code: 11000, Message: "E11000 duplicate key error collection: dns.other index: something dup key: { something: \"x\" }"}}},
			errMsg:         "Failed to update other",
			expectedStatus: 400,
			expectedError:  "duplicate key error",
		},
		{
			name:           "Invalid Request Body",
			err:            ErrInvalidRequestBody,
			errMsg:         ErrInvalidRequestBody.Error(),
			expectedStatus: 400,
			expectedError:  ErrInvalidRequestBody.Error(),
		},
		{
			name:           "Unhandled Error",
			err:            errors.New("Unhandled error"),
			errMsg:         "An unexpected error occurred",
			expectedStatus: 500,
			expectedError:  "An unexpected error occurred",
		},
		{
			name:            "Single Detail",
			err:             errors.New("test error"),
			errMsg:          "An error occurred",
			details:         []string{"Detail1"},
			expectedDetails: []string{"Detail1"},
			expectedStatus:  500,
			expectedError:   "An error occurred",
		},
		{
			name:           "Empty Details",
			err:            errors.New("test error"),
			errMsg:         "Test error message",
			expectedStatus: 500,
			expectedError:  "Test error message",
		},
		{
			name:           "Email OTP Rate Limited",
			err:            account.ErrEmailOTPRateLimited,
			errMsg:         "failed to request email verification otp",
			expectedStatus: 429,
			expectedError:  account.ErrEmailOTPRateLimited.Error(),
		},
		{
			name:           "Same Email Address",
			err:            account.ErrSameEmailAddress,
			errMsg:         "failed to update email",
			expectedStatus: 400,
			expectedError:  account.ErrSameEmailAddress.Error(),
		},
		{
			// Unified reauth: bad password is mapped to 400. The session is
			// still valid; the failure is a piece of the request body. (We
			// deliberately avoid 401 here because the global axios interceptor
			// treats any 401 as session expiry and triggers a global logout.)
			name:           "Invalid Current Password",
			err:            account.ErrInvalidCurrentPassword,
			errMsg:         "failed to update email",
			expectedStatus: 400,
			expectedError:  account.ErrInvalidCurrentPassword.Error(),
		},
		{
			name:           "Invalid New Email",
			err:            account.ErrInvalidNewEmail,
			errMsg:         "failed to update email",
			expectedStatus: 400,
			expectedError:  account.ErrInvalidNewEmail.Error(),
		},
		{
			name:           "Missing Email Update Fields",
			err:            account.ErrMissingEmailUpdateFields,
			errMsg:         "failed to update email",
			expectedStatus: 400,
			expectedError:  account.ErrMissingEmailUpdateFields.Error(),
		},
		{
			name:           "Profile: Unsupported Schema Version",
			err:            profile.ErrUnsupportedSchemaVersion,
			errMsg:         "import failed",
			expectedStatus: 400,
			expectedError:  profile.ErrUnsupportedSchemaVersion.Error(),
		},
		{
			name:           "Profile: Invalid Export Kind",
			err:            profile.ErrInvalidExportKind,
			errMsg:         "import failed",
			expectedStatus: 400,
			expectedError:  profile.ErrInvalidExportKind.Error(),
		},
		{
			name:           "Profile: Empty Import Payload",
			err:            profile.ErrEmptyImportPayload,
			errMsg:         "import failed",
			expectedStatus: 400,
			expectedError:  profile.ErrEmptyImportPayload.Error(),
		},
		{
			name:           "Profile: Max Profiles Exceeded (direct sentinel)",
			err:            profile.ErrMaxProfilesExceeded,
			errMsg:         "import failed",
			expectedStatus: 400,
			expectedError:  profile.ErrMaxProfilesExceeded.Error(),
		},
		{
			name:           "Profile: Max Profiles Exceeded (wrapped with %w augmentation)",
			err:            fmt.Errorf("%w: would exceed limit of 100; have 98, payload has 5", profile.ErrMaxProfilesExceeded),
			errMsg:         "import failed",
			expectedStatus: 400,
			expectedError:  profile.ErrMaxProfilesExceeded.Error() + ": would exceed limit of 100; have 98, payload has 5",
		},
		{
			name:           "Profile: Max Custom Rules Reached",
			err:            profile.ErrMaxCustomRulesReached,
			errMsg:         "create custom rules failed",
			expectedStatus: 400,
			expectedError:  profile.ErrMaxCustomRulesReached.Error(),
		},
		{
			name:           "Profile: Too Many Profile IDs",
			err:            profile.ErrTooManyProfileIds,
			errMsg:         "export failed",
			expectedStatus: 400,
			expectedError:  profile.ErrTooManyProfileIds.Error(),
		},
		{
			name:           "Profile: Unsupported Import Mode",
			err:            profile.ErrUnsupportedImportMode,
			errMsg:         "import failed",
			expectedStatus: 400,
			expectedError:  profile.ErrUnsupportedImportMode.Error(),
		},
		{
			name:           "Profile: Invalid Export Scope",
			err:            profile.ErrInvalidExportScope,
			errMsg:         "export failed",
			expectedStatus: 400,
			expectedError:  profile.ErrInvalidExportScope.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New()
			app.Get("/test", func(c *fiber.Ctx) error {
				return HandleError(c, tt.err, tt.errMsg, tt.details...)
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			resp, err := app.Test(req)

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedStatus, resp.StatusCode)

			var errResp ErrResponse
			err = json.NewDecoder(resp.Body).Decode(&errResp)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedError, errResp.Error)
			if len(tt.details) > 0 {
				assert.Equal(t, tt.expectedDetails, errResp.Details)
			} else {
				assert.Empty(t, errResp.Details)
			}
		})
	}
}
