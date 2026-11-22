package handler

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/KOHANTIC/SentryAtlas/backend/internal/event"
	"github.com/KOHANTIC/SentryAtlas/backend/internal/store"
)

// v2Store holds a quake from yesterday, one from ten days ago, and an NWS
// alert from an hour ago with its long text.
func v2Store(t *testing.T) *store.Store {
	t.Helper()
	now := time.Now().UTC()
	at := func(ago time.Duration) event.Time {
		return event.Time{Started: now.Add(-ago), Updated: now.Add(-ago)}
	}
	center := event.Point{-122.4, 37.8}
	st := store.New(store.Source{Name: "usgs"}, store.Source{Name: "noaa"})
	if err := st.Put("usgs", []event.Event{
		{ID: "usgs-new", Source: "usgs", Type: "earthquake", Title: "M 4.1", Description: "Long USGS text", Time: at(24 * time.Hour),
			Location: &event.Location{Center: &center}, Details: event.Details{USGS: &event.USGS{Place: "here"}}},
		{ID: "usgs-old", Source: "usgs", Type: "earthquake", Title: "M 3.0", Time: at(10 * 24 * time.Hour),
			Location: &event.Location{Center: &center}, Details: event.Details{USGS: &event.USGS{Place: "there"}}},
	}, now); err != nil {
		t.Fatal(err)
	}
	if err := st.Put("noaa", []event.Event{
		{ID: "noaa-urn:oid:2.49.0.1.840.0.abc.001.1", Source: "noaa", Type: "flood", Title: "Flood Warning", Description: "Long NWS text", Time: at(time.Hour),
			Location: &event.Location{Zones: []string{"CAZ006"}},
			Details:  event.Details{NOAA: &event.NWS{Event: "Flood Warning", Instruction: "Move to higher ground."}}},
	}, now); err != nil {
		t.Fatal(err)
	}
	return st
}

func v2Router(h *V2Handler) http.Handler {
	r := chi.NewRouter()
	r.Get("/api/v2/snapshot", h.Snapshot)
	r.Get("/api/v2/events/{id}", h.Event)
	return r
}

func get(t *testing.T, h http.Handler, path string, headers ...string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// decodeSnapshot holds the body to the v2 model: no field the model lacks,
// and every event valid.
func decodeSnapshot(t *testing.T, body []byte) event.Snapshot {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	var s event.Snapshot
	if err := dec.Decode(&s); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}
	for _, e := range s.Events {
		if err := e.Validate(); err != nil {
			t.Errorf("%s: %v", e.ID, err)
		}
	}
	return s
}

func TestSnapshotWindowsAndBriefEvents(t *testing.T) {
	t.Parallel()
	r := v2Router(NewV2Handler(v2Store(t)))

	for window, want := range map[string]int{"": 2, "7d": 2, "30d": 3} {
		path := "/api/v2/snapshot"
		if window != "" {
			path += "?window=" + window
		}
		rec := get(t, r, path)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", path, rec.Code, rec.Body)
		}
		s := decodeSnapshot(t, rec.Body.Bytes())
		if len(s.Events) != want {
			t.Errorf("%s: %d events, want %d", path, len(s.Events), want)
		}
		if w := map[string]string{"": "7d"}[window]; s.Window != w && s.Window != window {
			t.Errorf("%s: window %q", path, s.Window)
		}
		total := 0
		for _, st := range s.Sources {
			total += st.Count
		}
		if total != want {
			t.Errorf("%s: source counts add to %d, want this snapshot's %d", path, total, want)
		}
		for _, e := range s.Events {
			if e.Description != "" || (e.Details.NOAA != nil && e.Details.NOAA.Instruction != "") {
				t.Errorf("%s: %s carries its long text; only the detail endpoint does", path, e.ID)
			}
		}
	}

	// The store's own events keep their text.
	rec := get(t, r, "/api/v2/events/noaa-urn:oid:2.49.0.1.840.0.abc.001.1")
	if !strings.Contains(rec.Body.String(), "Move to higher ground.") || !strings.Contains(rec.Body.String(), "Long NWS text") {
		t.Errorf("detail lost the long text after a snapshot: %s", rec.Body)
	}
}

func TestSnapshotIsBuiltOncePerPoll(t *testing.T) {
	t.Parallel()
	st := v2Store(t)
	h := NewV2Handler(st)
	r := v2Router(h)

	first := get(t, r, "/api/v2/snapshot", "Accept-Encoding", "gzip")
	etag := first.Header().Get("ETag")
	if first.Header().Get("Content-Encoding") != "gzip" || !strings.HasPrefix(etag, `W/"`) {
		t.Fatalf("headers = %v, want gzip and a weak ETag", first.Header())
	}
	zr, err := gzip.NewReader(first.Body)
	if err != nil {
		t.Fatal(err)
	}
	unzipped, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	plain := get(t, r, "/api/v2/snapshot")
	if plain.Header().Get("Content-Encoding") != "" || !bytes.Equal(unzipped, plain.Body.Bytes()) {
		t.Error("the gzip body is not the plain one compressed")
	}
	for range 10 {
		get(t, r, "/api/v2/snapshot")
	}
	if h.builds != 1 {
		t.Errorf("%d builds for 12 requests with no new poll, want 1", h.builds)
	}

	if rec := get(t, r, "/api/v2/snapshot", "If-None-Match", etag); rec.Code != http.StatusNotModified || rec.Body.Len() != 0 {
		t.Errorf("If-None-Match %s: %d with %d bytes, want 304 and no body", etag, rec.Code, rec.Body.Len())
	}

	// A rebuild with nothing new keeps the tag; generated alone moves.
	h.mu.Lock()
	h.built["7d"].at = time.Now().Add(-2 * rebuildAfter)
	h.mu.Unlock()
	if again := get(t, r, "/api/v2/snapshot"); h.builds != 2 || again.Header().Get("ETag") != etag {
		t.Errorf("after an idle rebuild: %d builds, ETag %s, want 2 and %s", h.builds, again.Header().Get("ETag"), etag)
	}

	st.Fail("noaa", "timed out")
	if changed := get(t, r, "/api/v2/snapshot"); changed.Header().Get("ETag") == etag {
		t.Error("the ETag did not change after a poll changed a source's status")
	}
}

func TestSnapshotRefusesOtherParameters(t *testing.T) {
	t.Parallel()
	r := v2Router(NewV2Handler(v2Store(t)))
	for _, q := range []string{"?window=1d", "?window=7d&types=flood", "?types=flood", "?window=7d&window=30d"} {
		if rec := get(t, r, "/api/v2/snapshot"+q); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: %d, want 400: the snapshot has fixed variants only", q, rec.Code)
		}
	}
}

func TestV2IsNotServedOrCachedBeforeReady(t *testing.T) {
	t.Parallel()
	st := store.New(store.Source{Name: "usgs"}, store.Source{Name: "noaa"})
	if err := st.Put("usgs", []event.Event{}, time.Now()); err != nil {
		t.Fatal(err)
	}
	r := v2Router(NewV2Handler(st))
	for _, path := range []string{"/api/v2/snapshot", "/api/v2/events/usgs-1"} {
		rec := get(t, r, path)
		if rec.Code != http.StatusServiceUnavailable || rec.Header().Get("Cache-Control") != "no-store" {
			t.Errorf("%s before ready: %d, Cache-Control %q; want 503 no-store", path, rec.Code, rec.Header().Get("Cache-Control"))
		}
	}
}

func TestEventDetail(t *testing.T) {
	t.Parallel()
	r := v2Router(NewV2Handler(v2Store(t)))

	rec := get(t, r, "/api/v2/events/usgs-new")
	if rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != v2CacheControl {
		t.Fatalf("detail: %d, %v", rec.Code, rec.Header())
	}
	var body struct {
		Event  event.Event        `json:"event"`
		Source event.SourceStatus `json:"source"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Event.ID != "usgs-new" || body.Event.Description != "Long USGS text" || body.Source.Source != "usgs" || !body.Source.OK {
		t.Errorf("detail = %+v", body)
	}
	if rec := get(t, r, "/api/v2/events/usgs-nope"); rec.Code != http.StatusNotFound {
		t.Errorf("unknown id: %d, want 404", rec.Code)
	}
}

func TestETagMatchesAndGzipAcceptance(t *testing.T) {
	t.Parallel()
	for header, want := range map[string]bool{`W/"abc"`: true, `"abc"`: true, `"x", W/"abc"`: true, `*`: true, `"abd"`: false, ``: false} {
		if got := etagMatches(header, `W/"abc"`); got != want {
			t.Errorf("If-None-Match %q: %v, want %v", header, got, want)
		}
	}
	for header, want := range map[string]bool{"gzip": true, "br, gzip;q=0.8": true, "gzip;q=0": false, "identity": false, "": false, "*": true} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept-Encoding", header)
		if got := acceptsGzip(req); got != want {
			t.Errorf("Accept-Encoding %q: %v, want %v", header, got, want)
		}
	}
}
