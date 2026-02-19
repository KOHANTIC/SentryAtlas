package cache

import (
	"sync"
	"time"
)

type entry[V any] struct {
	value     V
	expiresAt time.Time
}

type Cache[V any] struct {
	mu      sync.RWMutex
	items   map[string]entry[V]
	ttl     time.Duration
	closeCh chan struct{}
}

func New[V any](ttl time.Duration) *Cache[V] {
	c := &Cache[V]{
		items:   make(map[string]entry[V]),
		ttl:     ttl,
		closeCh: make(chan struct{}),
	}
	go c.cleanup()
	return c
}

func (c *Cache[V]) Get(key string) (V, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	e, ok := c.items[key]
	if !ok || time.Now().After(e.expiresAt) {
		var zero V
		return zero, false
	}
	return e.value, true
}

func (c *Cache[V]) Set(key string, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[key] = entry[V]{
		value:     value,
		expiresAt: time.Now().Add(c.ttl),
	}
}

func (c *Cache[V]) Close() {
	close(c.closeCh)
}

func (c *Cache[V]) cleanup() {
	ticker := time.NewTicker(c.ttl / 2)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.mu.Lock()
			now := time.Now()
			for k, e := range c.items {
				if now.After(e.expiresAt) {
					delete(c.items, k)
				}
			}
			c.mu.Unlock()
		case <-c.closeCh:
			return
		}
	}
}
