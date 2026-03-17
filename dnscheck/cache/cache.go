package cache

import (
	"errors"
)

const CacheTypeBigCache = "bigcache"

// Cache is an interface for caching functionalities
type Cache interface {
	SaveQueryData(key string, value []byte) error
	GetQueryData(key string) ([]byte, error)
	DeleteQueryData(key string) error
}

// New creates a new Cache instance
func New(cacheType string) (Cache, error) {
	switch cacheType {
	case CacheTypeBigCache:
		return NewBigcache()
	}
	return nil, errors.New("unknown cache type")
}
