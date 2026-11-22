package store

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/KOHANTIC/SentryAtlas/backend/internal/adapters"
	"github.com/KOHANTIC/SentryAtlas/backend/internal/event"
)

// fetcher is a fake source; fetch runs for every poll, counted in calls.
type fetcher struct {
	name  string
	calls atomic.Int64
	fetch func(ctx context.Context, n int64) ([]event.Event, error)
}

func (f *fetcher) Source() string { return f.name }
func (f *fetcher) FetchV2(ctx context.Context) ([]event.Event, error) {
	return f.fetch(ctx, f.calls.Add(1))
}

func fixed(name string, n int) *fetcher {
	return &fetcher{name: name, fetch: func(context.Context, int64) ([]event.Event, error) { return events(name, n, nil), nil }}
}

var fastConfig = Config{Timeout: time.Second, RetryAfter: 5 * time.Millisecond}

// eventually waits up to 5 s for cond.
func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(time.Millisecond)
	}
}

func run(t *testing.T, s *Store, polls ...Poll) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	wait := Run(ctx, s, polls, fastConfig)
	t.Cleanup(func() { cancel(); wait() })
}

func TestReadsNeverCallASource(t *testing.T) {
	t.Parallel()
	usgs, noaa := fixed("usgs", 3), fixed("noaa", 2)
	s := New(Source{Name: "usgs"}, Source{Name: "noaa"})
	run(t, s, Poll{usgs, time.Hour}, Poll{noaa, time.Hour})
	eventually(t, "ready", s.Ready)
	before := usgs.calls.Load() + noaa.calls.Load()

	var wg sync.WaitGroup
	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if n := len(s.Read(time.Now())[0].Events); n != 3 {
				t.Errorf("read %d usgs events, want 3", n)
			}
		}()
	}
	wg.Wait()
	if after := usgs.calls.Load() + noaa.calls.Load(); after != before {
		t.Errorf("100 reads made %d source calls, want none", after-before)
	}
}

func TestOneFetchAtATime(t *testing.T) {
	t.Parallel()
	var inFlight, most atomic.Int64
	var polls []Poll
	var names []Source
	for _, name := range []string{"usgs", "eonet", "noaa", "gdacs"} {
		f := &fetcher{name: name, fetch: func(context.Context, int64) ([]event.Event, error) {
			n := inFlight.Add(1)
			for m := most.Load(); n > m && !most.CompareAndSwap(m, n); m = most.Load() {
			}
			time.Sleep(2 * time.Millisecond)
			inFlight.Add(-1)
			return events(name, 1, nil), nil
		}}
		polls = append(polls, Poll{f, 3 * time.Millisecond})
		names = append(names, Source{Name: name})
	}
	s := New(names...)
	run(t, s, polls...)
	eventually(t, "several rounds", func() bool { return polls[3].Fetcher.(*fetcher).calls.Load() >= 3 })
	if m := most.Load(); m != 1 {
		t.Errorf("%d fetches ran at once, want 1: two decoded responses at once can exceed the memory limit", m)
	}
}

func TestAFailedPollKeepsTheCopyAndRetriesSooner(t *testing.T) {
	t.Parallel()
	// Fails twice, then answers with 7 events.
	gate := make(chan struct{})
	f := &fetcher{name: "eonet", fetch: func(_ context.Context, n int64) ([]event.Event, error) {
		if n == 2 {
			<-gate // hold the second failure until the first is checked
		}
		if n <= 2 {
			return nil, &adapters.StatusError{Code: 503}
		}
		return events("eonet", 7, nil), nil
	}}
	s := New(Source{Name: "eonet"})
	s.Put("eonet", events("eonet", 5, nil), time.Now())
	// The interval is an hour, so only the retry wait can bring the third poll.
	run(t, s, Poll{f, time.Hour})

	eventually(t, "the failure", func() bool { return f.calls.Load() == 2 })
	if c := s.Read(time.Now())[0]; c.Status.OK || !c.Status.Stale || c.Status.Error != "answered HTTP 503" || c.Status.Count != 5 {
		t.Errorf("after a failure: %+v, want the last 5 events, stale, with the reason", c.Status)
	}
	close(gate)
	eventually(t, "the retry to answer", func() bool { return s.Read(time.Now())[0].Status.OK })
	if c := s.Read(time.Now())[0]; c.Status.Count != 7 || c.Status.Stale {
		t.Errorf("after the retries: %+v, want the third poll's 7 events", c.Status)
	}
}

func TestAPanicIsTheSourcesFailure(t *testing.T) {
	t.Parallel()
	f := &fetcher{name: "noaa", fetch: func(context.Context, int64) ([]event.Event, error) {
		var ring [][]float64
		_ = ring[0][1] // the old NWS centroid bug
		return nil, nil
	}}
	s := New(Source{Name: "noaa"})
	run(t, s, Poll{f, time.Hour})
	eventually(t, "the failure", s.Ready)
	if c := s.Read(time.Now())[0]; c.Status.OK || c.Status.Error != "failed inside SentryAtlas" {
		t.Errorf("status = %+v, want a failure inside SentryAtlas", c.Status)
	}
	eventually(t, "a retry after the panic", func() bool { return f.calls.Load() >= 2 })
}

func TestShutdownStopsEveryPoller(t *testing.T) {
	t.Parallel()
	started := make(chan struct{}, 1)
	// One fetch that blocks until it is cancelled, as a hung source would.
	hung := &fetcher{name: "gdacs", fetch: func(ctx context.Context, _ int64) ([]event.Event, error) {
		started <- struct{}{}
		<-ctx.Done()
		return nil, ctx.Err()
	}}
	s := New(Source{Name: "usgs"}, Source{Name: "gdacs"})
	ctx, cancel := context.WithCancel(context.Background())
	wait := Run(ctx, s, []Poll{{hung, time.Hour}, {fixed("usgs", 1), time.Hour}}, Config{Timeout: time.Hour, RetryAfter: time.Hour})
	<-started
	cancel()
	done := make(chan struct{})
	go func() { wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("pollers still running 2 s after shutdown")
	}
	if c := s.Read(time.Now())[1]; c.Status.Error == "timed out" || c.Status.Error == "could not be reached" {
		t.Errorf("gdacs status = %+v; a shutdown is not the source's failure", c.Status)
	}
}

func TestBackoff(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		n    int
		want time.Duration
	}{{1, 30 * time.Second}, {2, time.Minute}, {3, 2 * time.Minute}, {4, 4 * time.Minute}, {5, 5 * time.Minute}, {60, 5 * time.Minute}} {
		if got := backoff(30*time.Second, c.n, 5*time.Minute); got != c.want {
			t.Errorf("backoff after %d failures = %v, want %v", c.n, got, c.want)
		}
	}
}

func TestJitterStaysWithinATenth(t *testing.T) {
	t.Parallel()
	for _, r := range []float64{0, 0.5, 0.999999} {
		got := jitter(10*time.Minute, r)
		if got < 9*time.Minute || got >= 11*time.Minute {
			t.Errorf("jitter(10m, %v) = %v, want within 9m to 11m", r, got)
		}
	}
	if got := jitter(10*time.Minute, 0.5); got != 10*time.Minute {
		t.Errorf("jitter at the middle = %v, want the interval", got)
	}
}
