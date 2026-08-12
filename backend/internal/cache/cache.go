package cache

import (
	"fmt"
	"sync"
	"time"
)

type entry[V any] struct {
	value     V
	expiresAt time.Time
}

type Cache[V any] struct {
	mu        sync.RWMutex
	items     map[string]entry[V]
	ttl       time.Duration
	closeCh   chan struct{}
	closeOnce sync.Once
}

// New creates a cache whose entries expire after ttl. The ttl must be
// positive: it drives the janitor ticker, which panics opaquely deep in
// time.NewTicker otherwise.
func New[V any](ttl time.Duration) *Cache[V] {
	if ttl <= 0 {
		panic(fmt.Sprintf("cache: ttl must be positive, got %v", ttl))
	}
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

// Close stops the janitor goroutine. Safe to call more than once.
func (c *Cache[V]) Close() {
	c.closeOnce.Do(func() {
		close(c.closeCh)
	})
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
