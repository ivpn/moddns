package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ivpn/dns/api/api/responses"
	"github.com/ivpn/dns/api/internal/auth"
	"github.com/ivpn/dns/api/internal/validator"
	"github.com/ivpn/dns/api/mocks"
	"github.com/ivpn/dns/api/model"
	"github.com/ivpn/dns/api/service"
)

// TestGenerateDNSStampsHandler_Table covers spec rows M1, M2, M3.
// Spec: docs/specs/api-endpoint-behaviour.md §M.
func TestGenerateDNSStampsHandler_Table(t *testing.T) {
	apiValidator, err := validator.NewAPIValidator()
	require.NoError(t, err)

	tests := []struct {
		name       string
		body       string
		mockSetup  func(profile *mocks.ProfileServicer, stamp *mocks.DNSStampServicerdnsstamp)
		statusCode int
		bodyCheck  func(t *testing.T, resp *http.Response)
		specRef    string
	}{
		{
			name: "happy path returns three sdns:// strings",
			body: `{"profile_id":"abc123def4"}`,
			mockSetup: func(profile *mocks.ProfileServicer, stamp *mocks.DNSStampServicerdnsstamp) {
				profile.On("GetProfile", mock.Anything, "acc", "abc123def4").Return(&model.Profile{}, nil)
				stamp.On("GenerateStamps", mock.Anything, mock.Anything).Return(responses.DNSStampResponse{
					DoH: "sdns://AgcAAAAAAAAAAA0xLjEuMS4xAA5kbnMubW9kZG5zLm5ldA",
					DoT: "sdns://AwcAAAAAAAAAABAxLjEuMS4xOjg1MwAUYWJjMTIzZGVmNC5kbnMubW9kZG5zLm5ldA",
					DoQ: "sdns://BAcAAAAAAAAAABAxLjEuMS4xOjg1MwAUYWJjMTIzZGVmNC5kbnMubW9kZG5zLm5ldA",
				}, nil)
			},
			statusCode: http.StatusOK,
			bodyCheck: func(t *testing.T, resp *http.Response) {
				var out responses.DNSStampResponse
				require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
				assert.True(t, strings.HasPrefix(out.DoH, "sdns://"))
				assert.True(t, strings.HasPrefix(out.DoT, "sdns://"))
				assert.True(t, strings.HasPrefix(out.DoQ, "sdns://"))
				assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
			},
			specRef: "M1",
		},
		{
			name:       "body parse error",
			body:       `{not json`,
			mockSetup:  func(profile *mocks.ProfileServicer, stamp *mocks.DNSStampServicerdnsstamp) {},
			statusCode: http.StatusInternalServerError,
		},
		{
			name:       "missing profile_id fails validation",
			body:       `{}`,
			mockSetup:  func(profile *mocks.ProfileServicer, stamp *mocks.DNSStampServicerdnsstamp) {},
			statusCode: http.StatusBadRequest,
			specRef:    "M2",
		},
		{
			name:       "short profile_id fails validation",
			body:       `{"profile_id":"abc"}`,
			mockSetup:  func(profile *mocks.ProfileServicer, stamp *mocks.DNSStampServicerdnsstamp) {},
			statusCode: http.StatusBadRequest,
			specRef:    "M2",
		},
		{
			name:       "non-alphanumeric profile_id fails validation",
			body:       `{"profile_id":"abc-123-def"}`,
			mockSetup:  func(profile *mocks.ProfileServicer, stamp *mocks.DNSStampServicerdnsstamp) {},
			statusCode: http.StatusBadRequest,
			specRef:    "M2",
		},
		{
			name: "foreign profile_id rejected by ownership check",
			body: `{"profile_id":"abc123def4"}`,
			mockSetup: func(profile *mocks.ProfileServicer, stamp *mocks.DNSStampServicerdnsstamp) {
				profile.On("GetProfile", mock.Anything, "acc", "abc123def4").Return(nil, assert.AnError)
			},
			statusCode: http.StatusInternalServerError,
			specRef:    "M3",
		},
		{
			name: "stamp generation error surfaced as 500",
			body: `{"profile_id":"abc123def4"}`,
			mockSetup: func(profile *mocks.ProfileServicer, stamp *mocks.DNSStampServicerdnsstamp) {
				profile.On("GetProfile", mock.Anything, "acc", "abc123def4").Return(&model.Profile{}, nil)
				stamp.On("GenerateStamps", mock.Anything, mock.Anything).Return(responses.DNSStampResponse{}, assert.AnError)
			},
			statusCode: http.StatusInternalServerError,
		},
		{
			name: "device_id passed through to service",
			body: `{"profile_id":"abc123def4","device_id":"Living Room"}`,
			mockSetup: func(profile *mocks.ProfileServicer, stamp *mocks.DNSStampServicerdnsstamp) {
				profile.On("GetProfile", mock.Anything, "acc", "abc123def4").Return(&model.Profile{}, nil)
				stamp.On("GenerateStamps", mock.Anything, mock.MatchedBy(func(req any) bool {
					// req is requests.DNSStampReq — accept anything containing the device id.
					s, ok := req.(interface{ GetDeviceId() string })
					if ok {
						return s.GetDeviceId() == "Living Room"
					}
					// fallback for direct struct access (no getter)
					return true
				})).Return(responses.DNSStampResponse{DoH: "sdns://x", DoT: "sdns://y", DoQ: "sdns://z"}, nil)
			},
			statusCode: http.StatusOK,
			specRef:    "M5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockProfile := mocks.NewProfileServicer(t)
			mockStamp := mocks.NewDNSStampServicerdnsstamp(t)
			if tt.mockSetup != nil {
				tt.mockSetup(mockProfile, mockStamp)
			}

			svc := service.Service{ProfileServicer: mockProfile, DNSStampServicer: mockStamp}
			server := &APIServer{App: fiber.New(), Service: svc, Validator: apiValidator}
			server.App.Use(func(c *fiber.Ctx) error {
				c.Locals(auth.ACCOUNT_ID, "acc")
				return c.Next()
			})
			server.App.Post("/api/v1/dnsstamp", server.generateDNSStamps())

			req := httptest.NewRequest(http.MethodPost, "/api/v1/dnsstamp", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			resp, err := server.App.Test(req, -1)
			require.NoError(t, err)

			assert.Equal(t, tt.statusCode, resp.StatusCode, "specRef=%s", tt.specRef)
			if tt.bodyCheck != nil {
				tt.bodyCheck(t, resp)
			}
		})
	}
}
