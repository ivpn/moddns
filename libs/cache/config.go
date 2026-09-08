package cache

import "time"

// Config represents the cache configuration
type Config struct {
	Address               string
	FailoverAddresses     []string
	Username              string
	Password              string
	FailoverUsername      string
	FailoverPassword      string
	MasterName            string
	TLSEnabled            bool
	CertFile              string
	KeyFile               string
	CACertFile            string
	TLSInsecureSkipVerify bool // Only for testing & development, use false in production
	// CommandTimeout, when > 0, bounds every dial, read and write and limits
	// go-redis to a single retry. Zero keeps the go-redis defaults
	// (5s dial, 3s read/write, 3 retries), which suit bulk writers but not a
	// per-query read path.
	CommandTimeout time.Duration
}
