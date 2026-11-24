package utils_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ivpn/dns/api/internal/utils"
	"github.com/ivpn/dns/api/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestIDLimiter_Tick(t *testing.T) {
	tests := []struct {
		name    string
		label   string
		id      string
		incrErr error
		wantErr bool
	}{
		{
			name:    "successful tick",
			label:   "rate_limits",
			id:      "user1",
			incrErr: nil,
			wantErr: false,
		},
		{
			name:    "cache error propagated",
			label:   "rate_limits",
			id:      "user1",
			incrErr: errors.New("redis connection refused"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCache := mocks.NewCacheutils(t)
			mockCache.On("Incr", context.Background(), tt.label+":"+tt.id, mock.AnythingOfType("time.Duration")).Return(int64(1), tt.incrErr)

			limiter := utils.IDLimiter{
				ID:    tt.id,
				Label: tt.label,
				Max:   10,
				Exp:   time.Minute,
				Cache: mockCache,
			}

			err := limiter.Tick()

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestIDLimiter_IsAllowed(t *testing.T) {
	tests := []struct {
		name     string
		max      int
		getVal   string
		getErr   error
		expected bool
	}{
		{
			name:     "allowed when count is zero",
			max:      5,
			getVal:   "0",
			getErr:   nil,
			expected: true,
		},
		{
			name:     "allowed when count equals max",
			max:      5,
			getVal:   "5",
			getErr:   nil,
			expected: true,
		},
		{
			name:     "not allowed when count exceeds max",
			max:      5,
			getVal:   "6",
			getErr:   nil,
			expected: false,
		},
		{
			name:     "allowed when cache returns error (defaults to 0)",
			max:      5,
			getVal:   "",
			getErr:   errors.New("key not found"),
			expected: true,
		},
		{
			name:     "allowed when cache returns non-numeric value (defaults to 0)",
			max:      5,
			getVal:   "not-a-number",
			getErr:   nil,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCache := mocks.NewCacheutils(t)
			mockCache.On("Get", context.Background(), "rate_limits:user1").Return(tt.getVal, tt.getErr)

			limiter := utils.IDLimiter{
				ID:    "user1",
				Label: "rate_limits",
				Max:   tt.max,
				Exp:   time.Minute,
				Cache: mockCache,
			}

			result := limiter.IsAllowed()

			assert.Equal(t, tt.expected, result)
		})
	}
}

// A success clears the count: the limit applies to consecutive failures.
func TestIDLimiter_Reset(t *testing.T) {
	mockCache := mocks.NewCacheutils(t)
	mockCache.On("Del", context.Background(), "totp_fails:user1").Return(nil)

	limiter := utils.IDLimiter{ID: "user1", Label: "totp_fails", Max: 5, Exp: time.Minute, Cache: mockCache}
	assert.NoError(t, limiter.Reset())
}

// specRef: api-endpoint-behaviour.md E12
func TestIDLimiter_Attempt(t *testing.T) {
	tests := []struct {
		name    string
		count   int64
		incrErr error
		allowed bool
		wantErr bool
	}{
		{name: "first attempt", count: 1, allowed: true},
		{name: "last allowed attempt", count: 5, allowed: true},
		{name: "over the limit", count: 6, allowed: false},
		{name: "cache error refuses", incrErr: errors.New("redis down"), allowed: false, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCache := mocks.NewCacheutils(t)
			mockCache.On("Incr", context.Background(), "totp_fails:user1", time.Minute).Return(tt.count, tt.incrErr)

			limiter := utils.IDLimiter{ID: "user1", Label: "totp_fails", Max: 5, Exp: time.Minute, Cache: mockCache}
			allowed, err := limiter.Attempt()
			assert.Equal(t, tt.allowed, allowed)
			assert.Equal(t, tt.wantErr, err != nil)
		})
	}
}
