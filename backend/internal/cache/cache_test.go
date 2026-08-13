package cache

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestGetSetRoundtrip(t *testing.T) {
	t.Parallel()
	c := New[string](time.Minute)
	defer c.Close()

	if _, ok := c.Get("missing"); ok {
		t.Error("Get on empty cache returned ok")
	}

	c.Set("k", "v")
	got, ok := c.Get("k")
	if !ok {
		t.Fatal("Get after Set returned !ok")
	}
	if got != "v" {
		t.Errorf("Get = %q, want %q", got, "v")
	}

	// Overwrite.
	c.Set("k", "v2")
	if got, _ := c.Get("k"); got != "v2" {
		t.Errorf("Get after overwrite = %q, want %q", got, "v2")
	}
}

func TestGetReturnsZeroValueWhenMissing(t *testing.T) {
	t.Parallel()
	c := New[[]int](time.Minute)
	defer c.Close()

	got, ok := c.Get("nope")
	if ok {
		t.Error("Get returned ok for missing key")
	}
	if got != nil {
		t.Errorf("Get = %v, want zero value (nil)", got)
	}
}

func TestExpiryAfterTTL(t *testing.T) {
	t.Parallel()
	c := New[int](20 * time.Millisecond)
	defer c.Close()

	c.Set("k", 42)
	if _, ok := c.Get("k"); !ok {
		t.Fatal("entry expired immediately")
	}

	time.Sleep(40 * time.Millisecond)
	if v, ok := c.Get("k"); ok {
		t.Errorf("Get after TTL returned ok with %v, want expired", v)
	}
}

func TestJanitorRemovesExpiredEntries(t *testing.T) {
	t.Parallel()
	c := New[int](20 * time.Millisecond)
	defer c.Close()

	c.Set("k", 1)

	// The janitor ticks every ttl/2; poll the internal map until the entry
	// is physically deleted, not just invisible to Get.
	deadline := time.Now().Add(2 * time.Second)
	for {
		c.mu.RLock()
		_, present := c.items["k"]
		c.mu.RUnlock()
		if !present {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("janitor never removed the expired entry")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestCloseIsIdempotent(t *testing.T) {
	t.Parallel()
	c := New[int](time.Minute)
	c.Close()
	c.Close() // must not panic on double close

	// The cache remains usable for reads/writes after Close; only the
	// janitor stops.
	c.Set("k", 7)
	if v, ok := c.Get("k"); !ok || v != 7 {
		t.Errorf("Get after Close = %v, %v; want 7, true", v, ok)
	}
}

func TestNewPanicsOnNonPositiveTTL(t *testing.T) {
	t.Parallel()
	for _, ttl := range []time.Duration{0, -time.Second} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("New(%v) did not panic", ttl)
				}
			}()
			New[int](ttl)
		}()
	}
}

func TestConcurrentGetSet(t *testing.T) {
	t.Parallel()
	c := New[int](50 * time.Millisecond) // short TTL so the janitor runs during the test
	defer c.Close()

	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				key := fmt.Sprintf("k%d", i%16)
				c.Set(key, g*1000+i)
				c.Get(key)
			}
		}(g)
	}
	wg.Wait()
}
