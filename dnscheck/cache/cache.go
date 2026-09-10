package cache

import (
	"errors"
	"time"
)

const CacheTypeBigCache = "bigcache"

// Cache is an interface for caching functionalities
type Cache interface {
	SaveQueryData(key string, value []byte) error
	GetQueryData(key string) ([]byte, error)
	DeleteQueryData(key string) error
}

// New creates a new Cache instance whose entries expire after ttl.
func New(cacheType string, ttl time.Duration) (Cache, error) {
	switch cacheType {
	case CacheTypeBigCache:
		return NewBigcache(ttl)
	}
	return nil, errors.New("unknown cache type")
}
