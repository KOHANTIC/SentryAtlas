package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/KOHANTIC/SentryAtlas/backend/internal/adapters"
	"github.com/KOHANTIC/SentryAtlas/backend/internal/cache"
	"github.com/KOHANTIC/SentryAtlas/backend/internal/models"
	"github.com/KOHANTIC/SentryAtlas/backend/internal/service"
)

var baseTime = time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)

// fakeAdapter implements adapters.Adapter locally for handler tests.
type fakeAdapter struct {
	source string
	types  []string
	events []models.Event
	err    error
	gate   chan struct{} // if non-nil, FetchEvents blocks until closed (or ctx is done)

	mu    sync.Mutex
	calls int
	got   []adapters.FetchParams
}

func (f *fakeAdapter) FetchEvents(ctx context.Context, params adapters.FetchParams) ([]models.Event, error) {
	f.mu.Lock()
	f.calls++
	f.got = append(f.got, params)
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

func (f *fakeAdapter) SupportedTypes() []string {
	if f.types != nil {
		return f.types
	}
	return models.EventTypes
}

func (f *fakeAdapter) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func (f *fakeAdapter) lastParams(t *testing.T) adapters.FetchParams {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.got) == 0 {
		t.Fatal("adapter was never called")
	}
	return f.got[len(f.got)-1]
}

func newTestHandler(t *testing.T, adps ...adapters.Adapter) *EventsHandler {
	t.Helper()
	c := cache.New[[]models.Event](time.Minute)
	t.Cleanup(c.Close)
	svc := service.NewEventsService(adps, c, 5*time.Second)
	return NewEventsHandler(svc)
}

func makeEvents(n int, source string) []models.Event {
	events := make([]models.Event, n)
	for i := range events {
		events[i] = models.Event{
			ID:        fmt.Sprintf("%s-%d", source, i),
			Title:     fmt.Sprintf("Event %d", i),
			EventType: "earthquake",
			Source:    source,
			Geometry:  models.Geometry{Type: "Point", Coordinates: []float64{float64(i%340) - 170, 10}},
			StartedAt: baseTime.Add(-time.Duration(i) * time.Minute),
			UpdatedAt: baseTime.Add(-time.Duration(i) * time.Minute),
		}
	}
	return events
}

func doGet(t *testing.T, h *EventsHandler, query string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/events"+query, nil)
	rec := httptest.NewRecorder()
	h.GetEvents(rec, req)
	return rec
}

func decodeError(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("error response is not valid JSON: %v; body: %s", err, rec.Body.String())
	}
	return body["error"]
}

func decodeFeatureCollection(t *testing.T, rec *httptest.ResponseRecorder) models.FeatureCollection {
	t.Helper()
	var fc models.FeatureCollection
	if err := json.Unmarshal(rec.Body.Bytes(), &fc); err != nil {
		t.Fatalf("response is not a FeatureCollection: %v; body: %s", err, rec.Body.String())
	}
	return fc
}

func TestGetEventsDefaultGeoJSON(t *testing.T) {
	t.Parallel()
	f := &fakeAdapter{source: "alpha", events: makeEvents(2, "alpha")}
	h := newTestHandler(t, f)

	rec := doGet(t, h, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/geo+json" {
		t.Errorf("Content-Type = %q, want application/geo+json", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "public, max-age=60" {
		t.Errorf("Cache-Control = %q, want public, max-age=60", got)
	}

	fc := decodeFeatureCollection(t, rec)
	if fc.Type != "FeatureCollection" {
		t.Errorf("type = %q, want FeatureCollection", fc.Type)
	}
	if len(fc.Features) != 2 {
		t.Errorf("got %d features, want 2", len(fc.Features))
	}
	if len(fc.Sources) != 1 || !fc.Sources[0].OK || fc.Sources[0].Source != "alpha" {
		t.Errorf("sources = %+v, want [{alpha true}]", fc.Sources)
	}
}

func TestGetEventsJSONFormat(t *testing.T) {
	t.Parallel()
	f := &fakeAdapter{source: "alpha", events: makeEvents(3, "alpha")}
	h := newTestHandler(t, f)

	rec := doGet(t, h, "?format=json")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}

	var resp models.EventsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response is not an EventsResponse: %v", err)
	}
	if resp.Total != 3 || len(resp.Events) != 3 {
		t.Errorf("total = %d, events = %d, want 3 each", resp.Total, len(resp.Events))
	}
	if len(resp.Sources) != 1 || !resp.Sources[0].OK {
		t.Errorf("sources = %+v", resp.Sources)
	}
	if resp.Events[0].Coordinates == nil {
		t.Error("flat event coordinates missing")
	}
}

func TestGetEventsBadRequests(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		query   string
		wantMsg string
	}{
		{"unknown type", "?types=sharknado", "invalid type"},
		{"unknown type lists valid ones", "?types=sharknado", "valid types are"},
		{"bbox wrong count", "?bbox=1,2,3", "expected 4 values"},
		{"bbox non-numeric", "?bbox=a,2,3,4", "not a valid number"},
		{"invalid since", "?since=yesterday", "invalid since"},
		{"limit zero", "?limit=0", "invalid limit"},
		{"limit negative", "?limit=-5", "invalid limit"},
		{"limit non-numeric", "?limit=abc", "invalid limit"},
		{"invalid format", "?format=xml", "invalid format"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := &fakeAdapter{source: "alpha", events: makeEvents(1, "alpha")}
			h := newTestHandler(t, f)

			rec := doGet(t, h, tc.query)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body: %s", rec.Code, rec.Body.String())
			}
			if got := rec.Header().Get("Content-Type"); got != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", got)
			}
			if msg := decodeError(t, rec); !strings.Contains(msg, tc.wantMsg) {
				t.Errorf("error = %q, want it to contain %q", msg, tc.wantMsg)
			}
			if f.callCount() != 0 {
				t.Errorf("adapter called %d times on a 400, want 0", f.callCount())
			}
		})
	}
}

func TestGetEventsTypesDedupAndTrim(t *testing.T) {
	t.Parallel()
	f := &fakeAdapter{source: "alpha", events: makeEvents(1, "alpha")}
	h := newTestHandler(t, f)

	rec := doGet(t, h, "?types=earthquake,%20flood%20,earthquake")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	got := f.lastParams(t).Types
	if len(got) != 2 || got[0] != "earthquake" || got[1] != "flood" {
		t.Errorf("upstream Types = %v, want [earthquake flood] (trimmed, deduped)", got)
	}
}

func TestGetEventsSinceFormats(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		query string
		want  time.Time
	}{
		{"RFC3339", "?since=2026-08-01T12:30:00Z", time.Date(2026, 8, 1, 12, 30, 0, 0, time.UTC)},
		{"date only", "?since=2026-08-01", time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := &fakeAdapter{source: "alpha", events: makeEvents(1, "alpha")}
			h := newTestHandler(t, f)

			rec := doGet(t, h, tc.query)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
			}
			if got := f.lastParams(t).Since; !got.Equal(tc.want) {
				t.Errorf("upstream Since = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestGetEventsBBoxFiltering(t *testing.T) {
	t.Parallel()
	events := []models.Event{
		{
			ID: "inside", Title: "inside", EventType: "earthquake", Source: "alpha",
			Geometry:  models.Geometry{Type: "Point", Coordinates: []float64{10, 10}},
			StartedAt: baseTime, UpdatedAt: baseTime,
		},
		{
			ID: "outside", Title: "outside", EventType: "earthquake", Source: "alpha",
			Geometry:  models.Geometry{Type: "Point", Coordinates: []float64{50, 50}},
			StartedAt: baseTime, UpdatedAt: baseTime,
		},
		{
			ID: "unlocated", Title: "unlocated", EventType: "earthquake", Source: "alpha",
			StartedAt: baseTime, UpdatedAt: baseTime,
		},
	}
	f := &fakeAdapter{source: "alpha", events: events}
	h := newTestHandler(t, f)

	rec := doGet(t, h, "?bbox=0,0,20,20")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	fc := decodeFeatureCollection(t, rec)
	if len(fc.Features) != 1 {
		t.Fatalf("got %d features, want 1 (only the in-box event)", len(fc.Features))
	}
	if id := fc.Features[0].Properties["id"]; id != "inside" {
		t.Errorf("feature id = %v, want inside", id)
	}
}

func TestGetEventsLimits(t *testing.T) {
	t.Parallel()

	t.Run("default 500 for geojson", func(t *testing.T) {
		t.Parallel()
		f := &fakeAdapter{source: "alpha", events: makeEvents(501, "alpha")}
		h := newTestHandler(t, f)

		rec := doGet(t, h, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d; body: %s", rec.Code, rec.Body.String())
		}
		if fc := decodeFeatureCollection(t, rec); len(fc.Features) != 500 {
			t.Errorf("got %d features, want default limit 500", len(fc.Features))
		}
	})

	t.Run("default 500 for json", func(t *testing.T) {
		t.Parallel()
		f := &fakeAdapter{source: "alpha", events: makeEvents(501, "alpha")}
		h := newTestHandler(t, f)

		rec := doGet(t, h, "?format=json")
		var resp models.EventsResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp.Total != 500 {
			t.Errorf("total = %d, want default limit 500", resp.Total)
		}
	})

	t.Run("explicit limit respected", func(t *testing.T) {
		t.Parallel()
		f := &fakeAdapter{source: "alpha", events: makeEvents(10, "alpha")}
		h := newTestHandler(t, f)

		rec := doGet(t, h, "?limit=3")
		if fc := decodeFeatureCollection(t, rec); len(fc.Features) != 3 {
			t.Errorf("got %d features, want 3", len(fc.Features))
		}
	})

	t.Run("limit above max capped to 1000", func(t *testing.T) {
		t.Parallel()
		f := &fakeAdapter{source: "alpha", events: makeEvents(1001, "alpha")}
		h := newTestHandler(t, f)

		rec := doGet(t, h, "?limit=5000")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (over-max limit is capped, not rejected)", rec.Code)
		}
		if fc := decodeFeatureCollection(t, rec); len(fc.Features) != 1000 {
			t.Errorf("got %d features, want cap 1000", len(fc.Features))
		}
	})
}

func TestGetEventsAllSourcesFail(t *testing.T) {
	t.Parallel()
	f := &fakeAdapter{source: "alpha", err: errors.New("upstream exploded")}
	h := newTestHandler(t, f)

	rec := doGet(t, h, "")
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502; body: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
	if msg := decodeError(t, rec); msg != "all upstream sources failed" {
		t.Errorf("error = %q, want %q", msg, "all upstream sources failed")
	}
}

func TestGetEventsPartialFailureReportsSources(t *testing.T) {
	t.Parallel()
	bad := &fakeAdapter{source: "bad", err: errors.New("down")}
	good := &fakeAdapter{source: "good", events: makeEvents(1, "good")}
	h := newTestHandler(t, bad, good)

	rec := doGet(t, h, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 on partial failure; body: %s", rec.Code, rec.Body.String())
	}
	fc := decodeFeatureCollection(t, rec)
	if len(fc.Features) != 1 {
		t.Errorf("got %d features, want 1", len(fc.Features))
	}
	if len(fc.Sources) != 2 {
		t.Fatalf("sources = %+v, want 2 entries", fc.Sources)
	}
	byName := map[string]models.SourceStatus{}
	for _, s := range fc.Sources {
		byName[s.Source] = s
	}
	if st := byName["bad"]; st.OK || st.Error != "down" {
		t.Errorf("bad status = %+v, want failure with message", st)
	}
	if st := byName["good"]; !st.OK {
		t.Errorf("good status = %+v, want OK", st)
	}
}

func TestWriteErrorSurvivesQuotesInMessage(t *testing.T) {
	t.Parallel()
	f := &fakeAdapter{source: "alpha"}
	h := newTestHandler(t, f)

	// %22 is a double quote inside the invalid type name; the error message
	// quotes it back, so the response must still be valid JSON.
	rec := doGet(t, h, "?types=ea%22quake")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	msg := decodeError(t, rec) // fails the test if the body is not valid JSON
	// The handler echoes untrusted input with %q, so the quote arrives
	// Go-escaped inside the decoded message — what matters is that the
	// body stayed valid JSON and the offending value is recognizable.
	if !strings.Contains(msg, `ea\"quake`) {
		t.Errorf("error = %q, want it to contain the %%q-escaped type", msg)
	}
}

func TestGetEventsClientGoneWritesNothing(t *testing.T) {
	t.Parallel()
	gate := make(chan struct{})
	t.Cleanup(func() { close(gate) })
	f := &fakeAdapter{source: "alpha", gate: gate, events: makeEvents(1, "alpha")}
	h := newTestHandler(t, f)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/events", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.GetEvents(rec, req)

	if rec.Body.Len() != 0 {
		t.Errorf("body = %q, want nothing written for a gone client", rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "" {
		t.Errorf("Content-Type = %q, want unset", got)
	}
}

// SSE tests

type sseFrame struct {
	event string
	data  string
}

func parseSSE(t *testing.T, body string) []sseFrame {
	t.Helper()
	var frames []sseFrame
	for _, block := range strings.Split(body, "\n\n") {
		block = strings.TrimSpace(block)
		if block == "" || strings.HasPrefix(block, ":") {
			continue // comment (keep-alive) or trailing separator
		}
		var f sseFrame
		for _, line := range strings.Split(block, "\n") {
			switch {
			case strings.HasPrefix(line, "event: "):
				f.event = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				f.data = strings.TrimPrefix(line, "data: ")
			default:
				t.Fatalf("unexpected SSE line %q", line)
			}
		}
		frames = append(frames, f)
	}
	return frames
}

type sseDone struct {
	Total   int                   `json:"total"`
	Sources []models.SourceStatus `json:"sources"`
}

func decodeDone(t *testing.T, frames []sseFrame) sseDone {
	t.Helper()
	if len(frames) == 0 {
		t.Fatal("no SSE frames at all")
	}
	last := frames[len(frames)-1]
	if last.event != "done" {
		t.Fatalf("last frame is %q, want terminal done frame", last.event)
	}
	var d sseDone
	if err := json.Unmarshal([]byte(last.data), &d); err != nil {
		t.Fatalf("done frame is not valid JSON: %v; data: %s", err, last.data)
	}
	return d
}

func TestSSEEndToEnd(t *testing.T) {
	t.Parallel()
	f := &fakeAdapter{source: "alpha", events: makeEvents(2, "alpha")}
	h := newTestHandler(t, f)

	rec := doGet(t, h, "?format=sse")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache", got)
	}
	if !rec.Flushed {
		t.Error("response was never flushed")
	}
	// Exact framing: named event followed by a data line.
	if !strings.Contains(rec.Body.String(), "event: features\ndata: ") {
		t.Errorf("body lacks 'event: features' framing:\n%s", rec.Body.String())
	}

	frames := parseSSE(t, rec.Body.String())
	if len(frames) != 2 {
		t.Fatalf("got %d frames, want features + done; frames: %+v", len(frames), frames)
	}
	if frames[0].event != "features" {
		t.Errorf("first frame = %q, want features", frames[0].event)
	}
	var fc models.FeatureCollection
	if err := json.Unmarshal([]byte(frames[0].data), &fc); err != nil {
		t.Fatalf("features frame is not a FeatureCollection: %v", err)
	}
	if len(fc.Features) != 2 {
		t.Errorf("features frame has %d features, want 2", len(fc.Features))
	}

	done := decodeDone(t, frames)
	if done.Total != 2 {
		t.Errorf("done total = %d, want 2", done.Total)
	}
	if len(done.Sources) != 1 || !done.Sources[0].OK || done.Sources[0].Source != "alpha" {
		t.Errorf("done sources = %+v, want [{alpha true}]", done.Sources)
	}
}

func TestSSEErrorAndEmptySources(t *testing.T) {
	t.Parallel()
	bad := &fakeAdapter{source: "bad", err: errors.New("down")}
	empty := &fakeAdapter{source: "empty"}
	good := &fakeAdapter{source: "good", events: makeEvents(1, "good")}
	h := newTestHandler(t, bad, empty, good)

	rec := doGet(t, h, "?format=sse")
	frames := parseSSE(t, rec.Body.String())

	// Only the source with events produces a features frame; the failed and
	// the empty source appear solely in the done frame's statuses.
	var featureFrames int
	for _, fr := range frames {
		if fr.event == "features" {
			featureFrames++
		}
	}
	if featureFrames != 1 {
		t.Errorf("got %d features frames, want 1", featureFrames)
	}

	done := decodeDone(t, frames)
	if done.Total != 1 {
		t.Errorf("done total = %d, want 1", done.Total)
	}
	if len(done.Sources) != 3 {
		t.Fatalf("done sources = %+v, want 3 entries", done.Sources)
	}
	byName := map[string]models.SourceStatus{}
	for _, s := range done.Sources {
		byName[s.Source] = s
	}
	if st := byName["bad"]; st.OK || st.Error != "down" {
		t.Errorf("bad status = %+v, want failure with message", st)
	}
	if st := byName["empty"]; !st.OK {
		t.Errorf("empty status = %+v, want OK (reachable, just no events)", st)
	}
	if st := byName["good"]; !st.OK {
		t.Errorf("good status = %+v, want OK", st)
	}
}

func TestSSELimitAcrossStream(t *testing.T) {
	t.Parallel()
	a := &fakeAdapter{source: "alpha", events: makeEvents(3, "alpha")}
	b := &fakeAdapter{source: "beta", events: makeEvents(3, "beta")}
	h := newTestHandler(t, a, b)

	rec := doGet(t, h, "?format=sse&limit=4")
	frames := parseSSE(t, rec.Body.String())
	done := decodeDone(t, frames)
	if done.Total != 4 {
		t.Errorf("done total = %d, want the limit 4", done.Total)
	}

	streamed := 0
	for _, fr := range frames {
		if fr.event != "features" {
			continue
		}
		var fc models.FeatureCollection
		if err := json.Unmarshal([]byte(fr.data), &fc); err != nil {
			t.Fatalf("features frame: %v", err)
		}
		streamed += len(fc.Features)
	}
	if streamed != 4 {
		t.Errorf("streamed %d features across frames, want 4", streamed)
	}
}

func TestSSEHasNoDefaultLimit(t *testing.T) {
	t.Parallel()
	f := &fakeAdapter{source: "alpha", events: makeEvents(501, "alpha")}
	h := newTestHandler(t, f)

	rec := doGet(t, h, "?format=sse")
	done := decodeDone(t, parseSSE(t, rec.Body.String()))
	if done.Total != 501 {
		t.Errorf("done total = %d, want 501 (SSE has no default limit)", done.Total)
	}
}

func TestSSEAllSourcesFailStillEndsWithDone(t *testing.T) {
	t.Parallel()
	f := &fakeAdapter{source: "alpha", err: errors.New("down")}
	h := newTestHandler(t, f)

	rec := doGet(t, h, "?format=sse")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (stream already started)", rec.Code)
	}
	frames := parseSSE(t, rec.Body.String())
	if len(frames) != 1 {
		t.Fatalf("got %d frames, want only the done frame; frames: %+v", len(frames), frames)
	}
	done := decodeDone(t, frames)
	if done.Total != 0 {
		t.Errorf("done total = %d, want 0", done.Total)
	}
	if len(done.Sources) != 1 || done.Sources[0].OK {
		t.Errorf("done sources = %+v, want single failed source", done.Sources)
	}
}

// noFlushWriter hides the recorder's Flush method so the ResponseWriter no
// longer satisfies http.Flusher.
type noFlushWriter struct {
	http.ResponseWriter
}

func TestSSEWithoutFlusherFails(t *testing.T) {
	t.Parallel()
	f := &fakeAdapter{source: "alpha", events: makeEvents(1, "alpha")}
	h := newTestHandler(t, f)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/events?format=sse", nil)
	h.GetEvents(noFlushWriter{rec}, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if msg := decodeError(t, rec); msg != "streaming not supported" {
		t.Errorf("error = %q, want %q", msg, "streaming not supported")
	}
}
