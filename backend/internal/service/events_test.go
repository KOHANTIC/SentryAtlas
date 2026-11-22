package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/KOHANTIC/SentryAtlas/backend/internal/adapters"
	"github.com/KOHANTIC/SentryAtlas/backend/internal/cache"
	"github.com/KOHANTIC/SentryAtlas/backend/internal/models"
)

var baseTime = time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)

// fakeAdapter is a configurable in-memory adapters.Adapter.
type fakeAdapter struct {
	source string
	types  []string
	events []models.Event
	err    error
	gate   chan struct{} // if non-nil, FetchEvents blocks until closed (or ctx is done)

	mu    sync.Mutex
	calls int
}

func (f *fakeAdapter) FetchEvents(ctx context.Context) ([]models.Event, error) {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()

	if f.gate != nil {
		select {
		case <-f.gate:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if f.err != nil {
		return nil, f.err
	}
	return f.events, nil
}

func (f *fakeAdapter) Source() string { return f.source }

func (f *fakeAdapter) SupportedTypes() []string { return f.types }

func (f *fakeAdapter) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func newTestService(t *testing.T, adps ...adapters.Adapter) *EventsService {
	t.Helper()
	c := cache.New[[]models.Event](time.Minute)
	t.Cleanup(c.Close)
	return NewEventsService(adps, c, 5*time.Second)
}

// evt builds a minimal event. Coordinates are optional: none means unlocated.
func evt(id, typ string, startedAt time.Time, coords ...float64) models.Event {
	return models.Event{
		ID:        id,
		Title:     id,
		EventType: typ,
		Source:    "test",
		Geometry:  models.Geometry{Type: "Point", Coordinates: coords},
		StartedAt: startedAt,
		UpdatedAt: startedAt,
	}
}

func ids(events []models.Event) []string {
	out := make([]string, 0, len(events))
	for _, e := range events {
		out = append(out, e.ID)
	}
	return out
}

func statusBySource(t *testing.T, statuses []models.SourceStatus, source string) models.SourceStatus {
	t.Helper()
	for _, st := range statuses {
		if st.Source == source {
			return st
		}
	}
	t.Fatalf("no status for source %q in %+v", source, statuses)
	return models.SourceStatus{}
}

func TestGetEventsMergesAndSortsNewestFirst(t *testing.T) {
	t.Parallel()
	a := &fakeAdapter{source: "alpha", types: []string{"earthquake"}, events: []models.Event{
		evt("a-new", "earthquake", baseTime.Add(2*time.Hour), 10, 10),
		evt("a-old", "earthquake", baseTime.Add(-2*time.Hour), 11, 11),
	}}
	b := &fakeAdapter{source: "beta", types: []string{"flood"}, events: []models.Event{
		evt("b-mid", "flood", baseTime, 12, 12),
	}}
	s := newTestService(t, a, b)

	events, statuses, err := s.GetEvents(context.Background(), adapters.FetchParams{})
	if err != nil {
		t.Fatalf("GetEvents: %v", err)
	}
	want := []string{"a-new", "b-mid", "a-old"}
	got := ids(events)
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v (newest first)", got, want)
		}
	}
	if len(statuses) != 2 {
		t.Fatalf("statuses = %+v, want 2", statuses)
	}
	if !statusBySource(t, statuses, "alpha").OK || !statusBySource(t, statuses, "beta").OK {
		t.Errorf("statuses = %+v, want both OK", statuses)
	}
}

func TestGetEventsLimitTruncatesAfterSort(t *testing.T) {
	t.Parallel()
	a := &fakeAdapter{source: "alpha", types: []string{"earthquake"}, events: []models.Event{
		evt("e1", "earthquake", baseTime.Add(1*time.Hour), 1, 1),
		evt("e2", "earthquake", baseTime.Add(2*time.Hour), 2, 2),
		evt("e3", "earthquake", baseTime.Add(3*time.Hour), 3, 3),
		evt("e4", "earthquake", baseTime.Add(4*time.Hour), 4, 4),
	}}
	s := newTestService(t, a)

	events, _, err := s.GetEvents(context.Background(), adapters.FetchParams{Limit: 2})
	if err != nil {
		t.Fatalf("GetEvents: %v", err)
	}
	got := ids(events)
	if len(got) != 2 || got[0] != "e4" || got[1] != "e3" {
		t.Errorf("got %v, want the 2 newest [e4 e3]", got)
	}
}

func TestGetEventsAllSourcesFailed(t *testing.T) {
	t.Parallel()
	a := &fakeAdapter{source: "alpha", types: []string{"earthquake"}, err: fmt.Errorf("alpha: %w", &adapters.StatusError{Code: 503})}
	b := &fakeAdapter{source: "beta", types: []string{"flood"}, err: errors.New(`Get "https://beta.example/feed": dial tcp: connection refused`)}
	s := newTestService(t, a, b)

	events, statuses, err := s.GetEvents(context.Background(), adapters.FetchParams{})
	if !errors.Is(err, ErrAllSourcesFailed) {
		t.Fatalf("err = %v, want ErrAllSourcesFailed", err)
	}
	if events != nil {
		t.Errorf("events = %v, want nil", events)
	}
	if len(statuses) != 2 {
		t.Fatalf("statuses = %+v, want 2", statuses)
	}
	stA := statusBySource(t, statuses, "alpha")
	if stA.OK || stA.Error != "answered HTTP 503" {
		t.Errorf("alpha status = %+v, want !OK with the short reason", stA)
	}
	stB := statusBySource(t, statuses, "beta")
	if stB.OK || stB.Error != "could not be reached" {
		t.Errorf("beta status = %+v, want !OK with the short reason and no address", stB)
	}
}

func TestGetEventsPartialFailureStillSucceeds(t *testing.T) {
	t.Parallel()
	a := &fakeAdapter{source: "alpha", types: []string{"earthquake"}, err: errors.New("alpha down")}
	b := &fakeAdapter{source: "beta", types: []string{"flood"}, events: []models.Event{
		evt("b-1", "flood", baseTime, 12, 12),
	}}
	s := newTestService(t, a, b)

	events, statuses, err := s.GetEvents(context.Background(), adapters.FetchParams{})
	if err != nil {
		t.Fatalf("GetEvents: %v, want success on partial failure", err)
	}
	if len(events) != 1 || events[0].ID != "b-1" {
		t.Errorf("events = %v, want [b-1]", ids(events))
	}
	if st := statusBySource(t, statuses, "alpha"); st.OK || st.Error == "" {
		t.Errorf("alpha status = %+v, want failure recorded", st)
	}
	if st := statusBySource(t, statuses, "beta"); !st.OK {
		t.Errorf("beta status = %+v, want OK", st)
	}
}

func TestGetEventsNoMatchingAdapters(t *testing.T) {
	t.Parallel()
	a := &fakeAdapter{source: "alpha", types: []string{"flood"}}
	s := newTestService(t, a)

	events, statuses, err := s.GetEvents(context.Background(), adapters.FetchParams{Types: []string{"iceberg"}})
	if err != nil {
		t.Fatalf("GetEvents: %v", err)
	}
	if events == nil || len(events) != 0 {
		t.Errorf("events = %v, want empty non-nil slice", events)
	}
	if statuses != nil {
		t.Errorf("statuses = %+v, want nil", statuses)
	}
	if a.callCount() != 0 {
		t.Errorf("adapter called %d times, want 0", a.callCount())
	}
}

func TestGetEventsContextCancellation(t *testing.T) {
	t.Parallel()
	gate := make(chan struct{})
	t.Cleanup(func() { close(gate) })
	a := &fakeAdapter{source: "alpha", types: []string{"earthquake"}, gate: gate}
	s := newTestService(t, a)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // client is already gone

	start := time.Now()
	_, _, err := s.GetEvents(ctx, adapters.FetchParams{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("GetEvents blocked %v after cancellation", elapsed)
	}
}

func TestGetEventsCacheHitSkipsUpstream(t *testing.T) {
	t.Parallel()
	a := &fakeAdapter{source: "alpha", types: []string{"earthquake"}, events: []models.Event{
		evt("a-1", "earthquake", baseTime, 1, 1),
	}}
	s := newTestService(t, a)
	params := adapters.FetchParams{Types: []string{"earthquake"}}

	for i := 0; i < 2; i++ {
		events, _, err := s.GetEvents(context.Background(), params)
		if err != nil {
			t.Fatalf("GetEvents #%d: %v", i+1, err)
		}
		if len(events) != 1 {
			t.Fatalf("GetEvents #%d: got %d events, want 1", i+1, len(events))
		}
	}
	if got := a.callCount(); got != 1 {
		t.Errorf("upstream called %d times, want 1 (second request served from cache)", got)
	}
}

func TestGetEventsSingleflightDedup(t *testing.T) {
	t.Parallel()
	gate := make(chan struct{})
	a := &fakeAdapter{source: "alpha", types: []string{"earthquake"}, gate: gate, events: []models.Event{
		evt("a-1", "earthquake", baseTime, 1, 1),
	}}
	s := newTestService(t, a)

	const n = 8
	var wg sync.WaitGroup
	errs := make([]error, n)
	counts := make([]int, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			events, _, err := s.GetEvents(context.Background(), adapters.FetchParams{})
			errs[i] = err
			counts[i] = len(events)
		}(i)
	}

	// Wait until the first fetch is in flight, give the rest time to pile
	// onto the singleflight, then release.
	deadline := time.Now().Add(2 * time.Second)
	for a.callCount() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("adapter never called")
		}
		time.Sleep(time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond)
	close(gate)
	wg.Wait()

	for i := 0; i < n; i++ {
		if errs[i] != nil {
			t.Fatalf("request %d: %v", i, errs[i])
		}
		if counts[i] != 1 {
			t.Errorf("request %d: got %d events, want 1", i, counts[i])
		}
	}
	if got := a.callCount(); got != 1 {
		t.Errorf("upstream called %d times, want 1 (singleflight dedup)", got)
	}
}

// The adapter returns its whole window; every part of the request is cut
// from it here, including the types and since the adapters used to apply.
func TestGetEventsCutsTheRequestFromTheWindow(t *testing.T) {
	t.Parallel()
	a := &fakeAdapter{source: "alpha", types: []string{"flood", "storm"}, events: []models.Event{
		evt("flood-new-in", "flood", baseTime, 10, 10),
		evt("flood-new-in-2", "flood", baseTime.Add(-time.Minute), 11, 11),
		evt("flood-new-out", "flood", baseTime, 50, 50),
		evt("flood-old-in", "flood", baseTime.Add(-48*time.Hour), 10, 10),
		evt("storm-new-in", "storm", baseTime, 10, 10),
	}}
	s := newTestService(t, a)

	events, _, err := s.GetEvents(context.Background(), adapters.FetchParams{
		Types: []string{"flood"},
		BBox:  &adapters.BBox{MinLon: 0, MinLat: 0, MaxLon: 20, MaxLat: 20},
		Since: baseTime.Add(-24 * time.Hour),
		Limit: 1,
	})
	if err != nil {
		t.Fatalf("GetEvents: %v", err)
	}
	if got := ids(events); len(got) != 1 || got[0] != "flood-new-in" {
		t.Errorf("events = %v, want [flood-new-in]: the type, box, since and limit all applied", got)
	}
}

// The cache key holds only the source, so requests that differ in every
// parameter still share one upstream fetch per source within the TTL.
func TestOneFetchPerSourceWhateverTheRequest(t *testing.T) {
	t.Parallel()
	alpha := &fakeAdapter{source: "alpha", types: []string{"earthquake"}, events: []models.Event{
		evt("a-1", "earthquake", baseTime, 1, 1),
	}}
	beta := &fakeAdapter{source: "beta", types: []string{"flood", "storm"}, events: []models.Event{
		evt("b-1", "flood", baseTime, 2, 2),
		evt("b-2", "storm", baseTime, 3, 3),
	}}
	s := newTestService(t, alpha, beta)

	typeSets := [][]string{nil, {"earthquake"}, {"flood"}, {"earthquake", "flood"}, {"storm", "flood"}}
	for i := 0; i < 20; i++ {
		params := adapters.FetchParams{
			Types: typeSets[i%len(typeSets)],
			// Like the map's since: a different millisecond on every visit.
			Since: baseTime.Add(-7*24*time.Hour + time.Duration(i)*time.Millisecond),
		}
		if i%2 == 0 {
			if _, _, err := s.GetEvents(context.Background(), params); err != nil {
				t.Fatalf("GetEvents #%d: %v", i, err)
			}
		} else {
			collectBatches(t, s, context.Background(), params)
		}
	}

	if got := alpha.callCount(); got != 1 {
		t.Errorf("alpha fetched %d times, want 1", got)
	}
	if got := beta.callCount(); got != 1 {
		t.Errorf("beta fetched %d times, want 1", got)
	}
}

func TestSelectAdaptersHonorsSupportedTypes(t *testing.T) {
	t.Parallel()
	quake := &fakeAdapter{source: "quake", types: []string{"earthquake"}}
	water := &fakeAdapter{source: "water", types: []string{"flood", "tsunami"}}
	s := newTestService(t, quake, water)

	cases := []struct {
		name  string
		types []string
		want  []string
	}{
		{"no filter selects all", nil, []string{"quake", "water"}},
		{"single match", []string{"flood"}, []string{"water"}},
		{"one shared type is enough", []string{"earthquake", "iceberg"}, []string{"quake"}},
		{"types both support", []string{"earthquake", "tsunami"}, []string{"quake", "water"}},
		{"unsupported everywhere", []string{"iceberg"}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			selected := s.selectAdapters(tc.types)
			got := make([]string, 0, len(selected))
			for _, a := range selected {
				got = append(got, a.Source())
			}
			if len(got) != len(tc.want) {
				t.Fatalf("selectAdapters(%v) = %v, want %v", tc.types, got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("selectAdapters(%v) = %v, want %v", tc.types, got, tc.want)
				}
			}
		})
	}
}

func TestFilterEvents(t *testing.T) {
	t.Parallel()
	events := []models.Event{
		evt("quake-in", "earthquake", baseTime, 10, 10),
		evt("quake-edge", "earthquake", baseTime, 0, 0),
		evt("quake-out", "earthquake", baseTime, 50, 50),
		evt("quake-unlocated", "earthquake", baseTime),
		evt("flood-old", "flood", baseTime.Add(-48*time.Hour), 10, 10),
		evt("flood-new", "flood", baseTime.Add(time.Hour), 12, 12),
	}
	box := &adapters.BBox{MinLon: 0, MinLat: 0, MaxLon: 20, MaxLat: 20}

	cases := []struct {
		name   string
		params adapters.FetchParams
		want   []string
	}{
		{
			name:   "no filters keep everything",
			params: adapters.FetchParams{},
			want:   []string{"quake-in", "quake-edge", "quake-out", "quake-unlocated", "flood-old", "flood-new"},
		},
		{
			name:   "type filter",
			params: adapters.FetchParams{Types: []string{"flood"}},
			want:   []string{"flood-old", "flood-new"},
		},
		{
			name:   "since filter drops older events",
			params: adapters.FetchParams{Since: baseTime.Add(-time.Hour)},
			want:   []string{"quake-in", "quake-edge", "quake-out", "quake-unlocated", "flood-new"},
		},
		{
			name:   "bbox excludes out-of-box and unlocated, keeps boundary",
			params: adapters.FetchParams{BBox: box},
			want:   []string{"quake-in", "quake-edge", "flood-old", "flood-new"},
		},
		{
			name: "combined filters",
			params: adapters.FetchParams{
				Types: []string{"earthquake"},
				Since: baseTime.Add(-time.Hour),
				BBox:  box,
			},
			want: []string{"quake-in", "quake-edge"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := ids(filterEvents(events, tc.params))
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("got %v, want %v", got, tc.want)
				}
			}
		})
	}
}

func TestSortEventsByDate(t *testing.T) {
	t.Parallel()
	events := []models.Event{
		evt("old", "earthquake", baseTime.Add(-time.Hour)),
		evt("newest", "earthquake", baseTime.Add(time.Hour)),
		evt("mid", "earthquake", baseTime),
	}
	sortEventsByDate(events)
	got := ids(events)
	want := []string{"newest", "mid", "old"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
}

func collectBatches(t *testing.T, s *EventsService, ctx context.Context, params adapters.FetchParams) []StreamBatch {
	t.Helper()
	ch := make(chan StreamBatch, 8)
	go func() {
		s.StreamEvents(ctx, params, ch)
		close(ch)
	}()
	var batches []StreamBatch
	timeout := time.After(5 * time.Second)
	for {
		select {
		case b, ok := <-ch:
			if !ok {
				return batches
			}
			batches = append(batches, b)
		case <-timeout:
			t.Fatal("timed out collecting stream batches")
		}
	}
}

func batchBySource(t *testing.T, batches []StreamBatch, source string) StreamBatch {
	t.Helper()
	for _, b := range batches {
		if b.Source == source {
			return b
		}
	}
	t.Fatalf("no batch for source %q", source)
	return StreamBatch{}
}

func TestStreamEventsBatchPerSource(t *testing.T) {
	t.Parallel()
	a := &fakeAdapter{source: "alpha", types: []string{"earthquake"}, events: []models.Event{
		evt("a-old", "earthquake", baseTime.Add(-time.Hour), 1, 1),
		evt("a-new", "earthquake", baseTime.Add(time.Hour), 2, 2),
	}}
	b := &fakeAdapter{source: "beta", types: []string{"flood"}, events: []models.Event{
		evt("b-1", "flood", baseTime, 3, 3),
	}}
	s := newTestService(t, a, b)

	batches := collectBatches(t, s, context.Background(), adapters.FetchParams{})
	if len(batches) != 2 {
		t.Fatalf("got %d batches, want 2 (one per source)", len(batches))
	}

	ba := batchBySource(t, batches, "alpha")
	if ba.Err != nil {
		t.Fatalf("alpha batch error: %v", ba.Err)
	}
	got := ids(ba.Events)
	if len(got) != 2 || got[0] != "a-new" || got[1] != "a-old" {
		t.Errorf("alpha batch = %v, want [a-new a-old] (sorted within batch)", got)
	}

	bb := batchBySource(t, batches, "beta")
	if bb.Err != nil || len(bb.Events) != 1 || bb.Events[0].ID != "b-1" {
		t.Errorf("beta batch = %+v, want single b-1", bb)
	}
}

func TestStreamEventsErrorBatch(t *testing.T) {
	t.Parallel()
	bad := &fakeAdapter{source: "bad", types: []string{"earthquake"}, err: errors.New("upstream down")}
	good := &fakeAdapter{source: "good", types: []string{"flood"}, events: []models.Event{
		evt("g-1", "flood", baseTime, 1, 1),
	}}
	s := newTestService(t, bad, good)

	batches := collectBatches(t, s, context.Background(), adapters.FetchParams{})
	if len(batches) != 2 {
		t.Fatalf("got %d batches, want 2", len(batches))
	}

	be := batchBySource(t, batches, "bad")
	if be.Err == nil {
		t.Fatal("bad batch has no error")
	}
	if be.Events != nil {
		t.Errorf("bad batch carries events %v alongside error", ids(be.Events))
	}
	if bg := batchBySource(t, batches, "good"); bg.Err != nil || len(bg.Events) != 1 {
		t.Errorf("good batch = %+v, want one event and no error", bg)
	}
}

func TestStreamEventsGlobalLimit(t *testing.T) {
	t.Parallel()
	a := &fakeAdapter{source: "alpha", types: []string{"earthquake"}, events: []models.Event{
		evt("a-1", "earthquake", baseTime.Add(1*time.Minute), 1, 1),
		evt("a-2", "earthquake", baseTime.Add(2*time.Minute), 2, 2),
		evt("a-3", "earthquake", baseTime.Add(3*time.Minute), 3, 3),
	}}
	b := &fakeAdapter{source: "beta", types: []string{"flood"}, events: []models.Event{
		evt("b-1", "flood", baseTime.Add(4*time.Minute), 4, 4),
		evt("b-2", "flood", baseTime.Add(5*time.Minute), 5, 5),
		evt("b-3", "flood", baseTime.Add(6*time.Minute), 6, 6),
	}}
	s := newTestService(t, a, b)

	batches := collectBatches(t, s, context.Background(), adapters.FetchParams{Limit: 4})
	if len(batches) != 2 {
		t.Fatalf("got %d batches, want 2 (a trimmed batch is still sent)", len(batches))
	}
	total := 0
	for _, batch := range batches {
		if batch.Err != nil {
			t.Fatalf("batch %q error: %v", batch.Source, batch.Err)
		}
		total += len(batch.Events)
	}
	if total != 4 {
		t.Errorf("total streamed events = %d, want exactly the limit 4", total)
	}
}

func TestStreamEventsEmptyBatchStillSent(t *testing.T) {
	t.Parallel()
	empty := &fakeAdapter{source: "empty", types: []string{"earthquake"}}
	s := newTestService(t, empty)

	batches := collectBatches(t, s, context.Background(), adapters.FetchParams{})
	if len(batches) != 1 {
		t.Fatalf("got %d batches, want 1", len(batches))
	}
	b := batches[0]
	if b.Source != "empty" || b.Err != nil || len(b.Events) != 0 {
		t.Errorf("batch = %+v, want empty success batch for source", b)
	}
}

func TestStreamEventsNoMatchingAdapters(t *testing.T) {
	t.Parallel()
	a := &fakeAdapter{source: "alpha", types: []string{"flood"}}
	s := newTestService(t, a)

	batches := collectBatches(t, s, context.Background(), adapters.FetchParams{Types: []string{"iceberg"}})
	if len(batches) != 0 {
		t.Errorf("got %d batches, want 0", len(batches))
	}
}

func TestStreamEventsContextCancelDoesNotHang(t *testing.T) {
	t.Parallel()
	a := &fakeAdapter{source: "alpha", types: []string{"earthquake"}, events: []models.Event{
		evt("a-1", "earthquake", baseTime, 1, 1),
	}}
	s := newTestService(t, a)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	ch := make(chan StreamBatch) // unbuffered and never read: sends must not block forever
	done := make(chan struct{})
	go func() {
		s.StreamEvents(ctx, adapters.FetchParams{}, ch)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("StreamEvents did not return after context cancellation")
	}
}
