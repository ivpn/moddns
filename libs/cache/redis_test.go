package cache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDirectClient_CommandTimeoutApplied(t *testing.T) {
	c, err := NewDirectClient(&Config{Address: "127.0.0.1:1", CommandTimeout: 750 * time.Millisecond})
	require.NoError(t, err)
	defer c.Close()

	opts := c.Options()
	assert.Equal(t, 750*time.Millisecond, opts.DialTimeout)
	assert.Equal(t, 750*time.Millisecond, opts.ReadTimeout)
	assert.Equal(t, 750*time.Millisecond, opts.WriteTimeout)
	assert.Equal(t, 1, opts.MaxRetries)
}

func TestNewDirectClient_ZeroTimeoutKeepsClientDefaults(t *testing.T) {
	c, err := NewDirectClient(&Config{Address: "127.0.0.1:1"})
	require.NoError(t, err)
	defer c.Close()

	// go-redis fills its documented defaults in when nothing is set.
	opts := c.Options()
	assert.Equal(t, 5*time.Second, opts.DialTimeout)
	assert.Equal(t, 3*time.Second, opts.ReadTimeout)
	assert.Equal(t, 3*time.Second, opts.WriteTimeout)
	assert.Equal(t, 3, opts.MaxRetries)
}

func TestNewFailoverClient_CommandTimeoutApplied(t *testing.T) {
	c, err := NewFailoverClient(&Config{MasterName: "m", FailoverAddresses: []string{"127.0.0.1:1"}, CommandTimeout: 400 * time.Millisecond})
	require.NoError(t, err)
	defer c.Close()

	opts := c.Options()
	assert.Equal(t, 400*time.Millisecond, opts.DialTimeout)
	assert.Equal(t, 400*time.Millisecond, opts.ReadTimeout)
	assert.Equal(t, 400*time.Millisecond, opts.WriteTimeout)
	assert.Equal(t, 1, opts.MaxRetries)
}
