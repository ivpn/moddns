package cache

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

const (
	redisPingTimeout = 5 * time.Second
)

// NewRedisClient creates a new Redis client, configured according to the provided CacheConfig
func NewRedisClient(cfg *Config) (rdb *redis.Client, err error) {
	log.Info().Msg("Connecting to Redis")
	rdb, err = NewDirectClient(cfg)

	if cfg.MasterName != "" && len(cfg.FailoverAddresses) > 0 {
		rdb, err = NewFailoverClient(cfg)
	}
	if err != nil {
		return nil, err
	}

	// Test the Redis connection
	ctx, cancel := context.WithTimeout(context.Background(), redisPingTimeout)
	defer cancel()

	_, err = rdb.Ping(ctx).Result()
	if err != nil {
		log.Error().Err(err).Msg("Failed to ping Redis")
		return nil, err
	}

	log.Info().Msg("Connected to Redis")

	return rdb, nil
}

// NewDirectClient creates a client connected to a specific Redis address.
func NewDirectClient(cfg *Config) (*redis.Client, error) {
	options := &redis.Options{
		Addr:     cfg.Address,
		Username: cfg.Username,
		Password: cfg.Password,
	}
	cfg.applyCommandTimeout(&options.DialTimeout, &options.ReadTimeout, &options.WriteTimeout, &options.MaxRetries, &options.ContextTimeoutEnabled)

	return redis.NewClient(options), nil
}

// NewFailoverClient creates a sentinel-managed client.
func NewFailoverClient(cfg *Config) (*redis.Client, error) {
	log.Debug().Msg("Creating failover client")
	options := &redis.FailoverOptions{
		MasterName:       cfg.MasterName,
		SentinelAddrs:    cfg.FailoverAddresses,
		Username:         cfg.Username,
		Password:         cfg.Password,
		SentinelUsername: cfg.FailoverUsername,
		SentinelPassword: cfg.FailoverPassword,
		DB:               0,
	}
	cfg.applyCommandTimeout(&options.DialTimeout, &options.ReadTimeout, &options.WriteTimeout, &options.MaxRetries, &options.ContextTimeoutEnabled)

	if cfg.TLSEnabled {
		log.Debug().Msg("Using TLS to connect to Redis")
		cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load client certificate: %v", err)
		}

		caCert, err := os.ReadFile(cfg.CACertFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load CA certificate: %v", err)
		}

		caCertPool := x509.NewCertPool()
		if ok := caCertPool.AppendCertsFromPEM(caCert); !ok {
			return nil, fmt.Errorf("failed to append CA certificate")
		}

		options.TLSConfig = &tls.Config{
			Certificates:       []tls.Certificate{cert},
			RootCAs:            caCertPool,
			InsecureSkipVerify: cfg.TLSInsecureSkipVerify, //nolint:gosec // operator opt-in for dev/test only; false in production
		}
	}
	return redis.NewFailoverClient(options), nil
}

// applyCommandTimeout maps the single configured budget onto go-redis, which
// has no command-level timeout of its own: connecting, each read and each
// write are separate knobs, and retries multiply them. One value for all of
// them keeps the worst case per operation at CommandTimeout × 2 attempts.
// Context deadlines are only applied to socket I/O when ContextTimeoutEnabled
// is set, so a caller's per-query deadline can cut a hung read as well.
// Zero leaves the go-redis defaults untouched.
func (c *Config) applyCommandTimeout(dial, read, write *time.Duration, maxRetries *int, contextTimeoutEnabled *bool) {
	if c.CommandTimeout <= 0 {
		return
	}
	*dial, *read, *write = c.CommandTimeout, c.CommandTimeout, c.CommandTimeout
	*maxRetries = 1
	*contextTimeoutEnabled = true
}
