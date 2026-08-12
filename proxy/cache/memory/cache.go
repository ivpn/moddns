package memory

import (
	"errors"

	"github.com/ivpn/dns/libs/cache"
	"github.com/ivpn/dns/proxy/requestcontext"
)

const (
	CacheTypeBigCache = "bigcache"
)

// ErrRequestCtxNotFound is returned by GetRequestCtx when no entry exists for
// the request id; a nil error never carries a nil context.
var ErrRequestCtxNotFound = errors.New("request context not found in cache")

// MemoryCache - interface
type MemoryCache interface {
	SetRequestCtx(requestId string, reqCtx *requestcontext.RequestContext) error
	GetRequestCtx(requestId string) (*requestcontext.RequestContext, error)
	Stats()
}

// NewCache creates a new in-app memory instance
func NewCache(cacheCfg *cache.Config, cacheType string) (MemoryCache, error) {
	switch cacheType { // nolint
	case CacheTypeBigCache:
		cache, err := NewBigcache(cacheCfg)
		if err != nil {
			return nil, err
		}
		return cache, nil
	}
	return nil, errors.New("unknown cache type")
}
