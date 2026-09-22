package cache

import (
	"context"
	"time"

	"github.com/allegro/bigcache/v3"
)

type BigCache struct {
	cache *bigcache.BigCache
}

// NewBigcache creates a new BigCache instance with the given entry lifetime.
func NewBigcache(ttl time.Duration) (*BigCache, error) {
	queriesCache := &BigCache{}
	cache, err := bigcache.New(context.Background(), bigcache.DefaultConfig(ttl))
	if err != nil {
		return nil, err
	}
	queriesCache.cache = cache
	return queriesCache, nil
}

func (c *BigCache) SaveQueryData(key string, value []byte) error {
	return c.cache.Set(key, value)
}

func (c *BigCache) GetQueryData(key string) ([]byte, error) {
	return c.cache.Get(key)
}

func (c *BigCache) DeleteQueryData(key string) error {
	return c.cache.Delete(key)
}
