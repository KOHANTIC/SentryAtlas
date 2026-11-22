package store

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"runtime/debug"
	"sync"
	"time"

	"github.com/KOHANTIC/SentryAtlas/backend/internal/adapters"
	"github.com/KOHANTIC/SentryAtlas/backend/internal/event"
)

// Fetcher is a source's adapter as a poller uses it.
type Fetcher interface {
	Source() string
	FetchV2(ctx context.Context) ([]event.Event, error)
}

// Poll is one source to poll and how often.
type Poll struct {
	Fetcher  Fetcher
	Interval time.Duration
}

// Config sets how every poller fetches.
type Config struct {
	// Timeout bounds one fetch.
	Timeout time.Duration
	// RetryAfter is the first wait after a failed poll; it doubles with
	// each failure in a row, up to the source's interval.
	RetryAfter time.Duration
}

// Run starts one poller per source, each polling at once and then on its
// interval, with jitter, until ctx ends. Fetches take turns, so only one
// response is decoded at a time. The returned function waits for every
// poller to stop.
func Run(ctx context.Context, s *Store, polls []Poll, cfg Config) (wait func()) {
	turn := make(chan struct{}, 1)
	var wg sync.WaitGroup
	for _, p := range polls {
		wg.Add(1)
		go func() {
			defer wg.Done()
			poll(ctx, s, p, cfg, turn)
		}()
	}
	return wg.Wait
}

func poll(ctx context.Context, s *Store, p Poll, cfg Config, turn chan struct{}) {
	name := p.Fetcher.Source()
	failures := 0
	var delay time.Duration
	for {
		if delay > 0 {
			t := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				t.Stop()
				return
			case <-t.C:
			}
		}
		select {
		case turn <- struct{}{}:
		case <-ctx.Done():
			return
		}
		start := time.Now()
		events, err := fetchOnce(ctx, p.Fetcher, cfg.Timeout)
		<-turn
		if ctx.Err() != nil {
			// Stopped mid-fetch: a shutdown, not the source's failure.
			return
		}

		if err != nil {
			s.Fail(name, adapters.Reason(err))
		} else {
			err = s.Put(name, valid(name, events), time.Now())
		}
		if err != nil {
			failures++
			delay = backoff(cfg.RetryAfter, failures, p.Interval)
			slog.Warn("poll failed", "source", name, "error", err, "failures_in_a_row", failures, "retry_in", delay.String())
			continue
		}
		failures = 0
		delay = jitter(p.Interval, rand.Float64())
		slog.Info("polled", "source", name, "events", len(events), "took_ms", time.Since(start).Milliseconds())
	}
}

// fetchOnce runs one fetch with its timeout. A panic in the adapter, which
// would otherwise end the process, becomes the source's failure.
func fetchOnce(ctx context.Context, f Fetcher, timeout time.Duration) (events []event.Event, err error) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("adapter panicked", "source", f.Source(), "panic", fmt.Sprint(r), "stack", string(debug.Stack()))
			err = fmt.Errorf("%s: %w: %v", f.Source(), adapters.ErrInternal, r)
		}
	}()
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	events, err = f.FetchV2(ctx)
	if err == nil && events == nil {
		// A nil list from a fetch that succeeded is an empty one.
		events = []event.Event{}
	}
	return events, err
}

// valid leaves out the events that break the v2 model, which no reader
// should get, and logs how many with the first reason.
func valid(source string, events []event.Event) []event.Event {
	kept := events[:0:0]
	dropped := 0
	var first error
	for _, e := range events {
		if err := e.Validate(); err != nil {
			if dropped == 0 {
				first = fmt.Errorf("%s: %w", e.ID, err)
			}
			dropped++
			continue
		}
		kept = append(kept, e)
	}
	if dropped > 0 {
		slog.Warn("left out events that break the model", "source", source, "dropped", dropped, "kept", len(kept), "first_error", first.Error())
	}
	return kept
}

// backoff is the wait after the nth failure in a row: first, doubled each
// time, never longer than interval.
func backoff(first time.Duration, n int, interval time.Duration) time.Duration {
	d := first
	for i := 1; i < n && d < interval; i++ {
		d *= 2
	}
	return min(d, interval)
}

// jitter spreads an interval by up to 10% either way, r in [0, 1), so the
// pollers do not keep calling their sources in step.
func jitter(interval time.Duration, r float64) time.Duration {
	return time.Duration(float64(interval) * (0.9 + 0.2*r))
}
