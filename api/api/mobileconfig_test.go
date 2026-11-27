package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ivpn/dns/api/internal/auth"
	"github.com/ivpn/dns/api/internal/validator"
	"github.com/ivpn/dns/api/mocks"
	"github.com/ivpn/dns/api/model"
	"github.com/ivpn/dns/api/service"
	"github.com/ivpn/dns/api/service/apple"
)

func TestGenerateMobileConfigHandler_Table(t *testing.T) {
	apiValidator, err := validator.NewAPIValidator()
	require.NoError(t, err)

	tests := []struct {
		name        string
		body        string
		mockSetup   func(profile *mocks.ProfileServicer, appleSrv *mocks.AppleServicer)
		statusCode  int
		headerCheck func(t *testing.T, resp *http.Response)
		bodyCheck   func(t *testing.T, resp *http.Response)
	}{
		{
			name: "success",
			body: `{"profile_id":"p1","advanced_options":{"encryption_type":"https"}}`,
			mockSetup: func(profile *mocks.ProfileServicer, appleSrv *mocks.AppleServicer) {
				profile.On("GetProfile", mock.Anything, "acc", "p1").Return(&model.Profile{}, nil)
				appleSrv.On("GenerateMobileConfig", mock.Anything, mock.Anything, "acc", false).Return([]byte("mcdata"), "", nil)
			},
			statusCode: http.StatusCreated,
			headerCheck: func(t *testing.T, resp *http.Response) {
				assert.Equal(t, "application/x-apple-aspen-config", resp.Header.Get("Content-Type"))
				assert.Contains(t, resp.Header.Get("Content-Disposition"), "attachment")
				assert.Contains(t, resp.Header.Get("Content-Disposition"), "modDNS-p1.mobileconfig")
			},
			bodyCheck: func(t *testing.T, resp *http.Response) {
				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err)
				assert.Equal(t, "mcdata", string(body))
			},
		},
		{
			name:       "body parse error",
			body:       `{bad json`,
			mockSetup:  func(profile *mocks.ProfileServicer, appleSrv *mocks.AppleServicer) {},
			statusCode: http.StatusBadRequest,
		},
		{
			name:       "validation error",
			body:       `{"profile_id":"p1","advanced_options":{"encryption_type":"invalid"}}`,
			mockSetup:  func(profile *mocks.ProfileServicer, appleSrv *mocks.AppleServicer) {},
			statusCode: http.StatusBadRequest,
		},
		{
			name: "get profile error",
			body: `{"profile_id":"p1","advanced_options":{"encryption_type":"https"}}`,
			mockSetup: func(profile *mocks.ProfileServicer, appleSrv *mocks.AppleServicer) {
				profile.On("GetProfile", mock.Anything, "acc", "p1").Return(nil, assert.AnError)
			},
			statusCode: http.StatusInternalServerError,
		},
		{
			name: "generate error",
			body: `{"profile_id":"p1","advanced_options":{"encryption_type":"https"}}`,
			mockSetup: func(profile *mocks.ProfileServicer, appleSrv *mocks.AppleServicer) {
				profile.On("GetProfile", mock.Anything, "acc", "p1").Return(&model.Profile{}, nil)
				appleSrv.On("GenerateMobileConfig", mock.Anything, mock.Anything, "acc", false).Return(nil, "", assert.AnError)
			},
			statusCode: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockProfile := mocks.NewProfileServicer(t)
			mockApple := mocks.NewAppleServicer(t)
			if tt.mockSetup != nil {
				tt.mockSetup(mockProfile, mockApple)
			}

			svc := service.Service{ProfileServicer: mockProfile, AppleServicer: mockApple}
			server := &APIServer{App: fiber.New(), Service: svc, Validator: apiValidator}
			server.App.Use(func(c *fiber.Ctx) error {
				c.Locals(auth.ACCOUNT_ID, "acc")
				return c.Next()
			})
			server.App.Post("/api/v1/mobileconfig", server.generateMobileConfig())

			req := httptest.NewRequest(http.MethodPost, "/api/v1/mobileconfig", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			resp, err := server.App.Test(req, -1)
			require.NoError(t, err)

			assert.Equal(t, tt.statusCode, resp.StatusCode)
			if tt.headerCheck != nil {
				tt.headerCheck(t, resp)
			}
			if tt.bodyCheck != nil {
				tt.bodyCheck(t, resp)
			}
		})
	}
}

func TestGenerateMobileConfigShortLinkHandler_Table(t *testing.T) {
	apiValidator, err := validator.NewAPIValidator()
	require.NoError(t, err)

	tests := []struct {
		name       string
		body       string
		mockSetup  func(profile *mocks.ProfileServicer, appleSrv *mocks.AppleServicer)
		statusCode int
		bodyCheck  func(t *testing.T, resp *http.Response)
	}{
		{
			name: "success",
			body: `{"profile_id":"p1","advanced_options":{"encryption_type":"https"}}`,
			mockSetup: func(profile *mocks.ProfileServicer, appleSrv *mocks.AppleServicer) {
				profile.On("GetProfile", mock.Anything, "acc", "p1").Return(&model.Profile{}, nil)
				appleSrv.On("GenerateMobileConfig", mock.Anything, mock.Anything, "acc", true).Return([]byte("ignored"), "https://frontend/short/abc", nil)
			},
			statusCode: http.StatusOK,
			bodyCheck: func(t *testing.T, resp *http.Response) {
				var out map[string]string
				require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
				assert.Equal(t, "https://frontend/short/abc", out["link"])
				assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
			},
		},
		{
			name:       "body parse error",
			body:       `{bad json`,
			mockSetup:  func(profile *mocks.ProfileServicer, appleSrv *mocks.AppleServicer) {},
			statusCode: http.StatusBadRequest,
		},
		{
			name:       "validation error",
			body:       `{"profile_id":"p1","advanced_options":{"encryption_type":"invalid"}}`,
			mockSetup:  func(profile *mocks.ProfileServicer, appleSrv *mocks.AppleServicer) {},
			statusCode: http.StatusBadRequest,
		},
		{
			name: "get profile error",
			body: `{"profile_id":"p1","advanced_options":{"encryption_type":"https"}}`,
			mockSetup: func(profile *mocks.ProfileServicer, appleSrv *mocks.AppleServicer) {
				profile.On("GetProfile", mock.Anything, "acc", "p1").Return(nil, assert.AnError)
			},
			statusCode: http.StatusInternalServerError,
		},
		{
			name: "generate error",
			body: `{"profile_id":"p1","advanced_options":{"encryption_type":"https"}}`,
			mockSetup: func(profile *mocks.ProfileServicer, appleSrv *mocks.AppleServicer) {
				profile.On("GetProfile", mock.Anything, "acc", "p1").Return(&model.Profile{}, nil)
				appleSrv.On("GenerateMobileConfig", mock.Anything, mock.Anything, "acc", true).Return(nil, "", assert.AnError)
			},
			statusCode: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockProfile := mocks.NewProfileServicer(t)
			mockApple := mocks.NewAppleServicer(t)
			if tt.mockSetup != nil {
				tt.mockSetup(mockProfile, mockApple)
			}

			svc := service.Service{ProfileServicer: mockProfile, AppleServicer: mockApple}
			server := &APIServer{App: fiber.New(), Service: svc, Validator: apiValidator}
			server.App.Use(func(c *fiber.Ctx) error {
				c.Locals(auth.ACCOUNT_ID, "acc")
				return c.Next()
			})
			server.App.Post("/api/v1/mobileconfig/short", server.generateMobileConfigShortLink())

			req := httptest.NewRequest(http.MethodPost, "/api/v1/mobileconfig/short", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			resp, err := server.App.Test(req, -1)
			require.NoError(t, err)

			assert.Equal(t, tt.statusCode, resp.StatusCode)
			if tt.bodyCheck != nil {
				tt.bodyCheck(t, resp)
			}
		})
	}
}

func TestDownloadMobileConfigFromLink_Table(t *testing.T) {
	tests := []struct {
		name       string
		code       string
		cacheSetup func(cache *mocks.Cachecache)
		status     int
		body       string
		headers    map[string]string
	}{
		{
			name: "cache hit with profile",
			code: "abc",
			cacheSetup: func(cache *mocks.Cachecache) {
				cache.On("Get", mock.Anything, apple.MobileConfigCacheKey("abc")).Return("p1|mcdata", nil)
			},
			status: http.StatusOK,
			body:   "mcdata",
			headers: map[string]string{
				"Content-Type":        "application/x-apple-aspen-config",
				"Content-Disposition": "attachment; filename=modDNS-p1.mobileconfig",
			},
		},
		{
			name: "cache hit without delimiter",
			code: "abc",
			cacheSetup: func(cache *mocks.Cachecache) {
				cache.On("Get", mock.Anything, apple.MobileConfigCacheKey("abc")).Return("mcdata", nil)
			},
			status: http.StatusOK,
			body:   "mcdata",
			headers: map[string]string{
				"Content-Type":        "application/x-apple-aspen-config",
				"Content-Disposition": "attachment; filename=modDNS-profile.mobileconfig",
			},
		},
		{
			name: "cache hit empty profile prefix",
			code: "abc",
			cacheSetup: func(cache *mocks.Cachecache) {
				cache.On("Get", mock.Anything, apple.MobileConfigCacheKey("abc")).Return("|mcdata", nil)
			},
			status: http.StatusOK,
			body:   "|mcdata",
			headers: map[string]string{
				"Content-Type":        "application/x-apple-aspen-config",
				"Content-Disposition": "attachment; filename=modDNS-profile.mobileconfig",
			},
		},
		{
			name: "cache miss",
			code: "missing",
			cacheSetup: func(cache *mocks.Cachecache) {
				cache.On("Get", mock.Anything, apple.MobileConfigCacheKey("missing")).Return("", assert.AnError)
			},
			status: http.StatusNotFound,
			body:   "Configuration not found or expired",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCache := mocks.NewCachecache(t)
			if tt.cacheSetup != nil {
				tt.cacheSetup(mockCache)
			}

			server := &APIServer{Cache: mockCache, App: fiber.New()}
			server.App.Get("/api/v1/short/:code", server.downloadMobileConfigFromLink())

			req := httptest.NewRequest(http.MethodGet, "/api/v1/short/"+tt.code, nil)
			resp, err := server.App.Test(req, -1)
			require.NoError(t, err)

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			assert.Equal(t, tt.status, resp.StatusCode)
			assert.Equal(t, tt.body, string(body))
			for k, v := range tt.headers {
				assert.Equal(t, v, resp.Header.Get(k))
			}
		})
	}
}
