package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// specRef: proxy-request-admission-behaviour.md #Q9
func TestLoadMaxGoroutines(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want uint
	}{
		{name: "default when unset", env: "", want: 10_000},
		{name: "override", env: "2500", want: 2500},
		{name: "zero disables the cap", env: "0", want: 0},
		{name: "non-numeric keeps default", env: "unbounded", want: 10_000},
		{name: "negative keeps default", env: "-5", want: 10_000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("MAX_GOROUTINES", tt.env)
			assert.Equal(t, tt.want, loadMaxGoroutines())
		})
	}
}

// specRef: proxy-request-admission-behaviour.md #Q13
func TestLoadProfileSettingsCacheSize(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want int
	}{
		{name: "default when unset", env: "", want: 100_000},
		{name: "override", env: "5000", want: 5000},
		{name: "zero keeps default", env: "0", want: 100_000},
		{name: "negative keeps default", env: "-1", want: 100_000},
		{name: "non-numeric keeps default", env: "lots", want: 100_000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("PROFILE_SETTINGS_CACHE_SIZE", tt.env)
			assert.Equal(t, tt.want, loadProfileSettingsCacheSize())
		})
	}
}
