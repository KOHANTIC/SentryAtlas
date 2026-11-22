package handler

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/KOHANTIC/SentryAtlas/backend/internal/event"
	"github.com/KOHANTIC/SentryAtlas/backend/internal/group"
	"github.com/KOHANTIC/SentryAtlas/backend/internal/store"
)

// snapshotWindows are the only snapshots there are, so every visitor asks
// for one of a few identical responses that an edge can keep.
var snapshotWindows = map[string]time.Duration{
	"7d":  7 * 24 * time.Hour,
	"30d": 30 * 24 * time.Hour,
}

const (
	defaultWindow = "7d"
	// rebuildAfter bounds how long a snapshot is reused with no new poll,
	// so events that expire meanwhile leave it.
	rebuildAfter = time.Minute
	// v2CacheControl lets browsers and the edge reuse an answer briefly;
	// the fastest source is polled every 2 minutes.
	v2CacheControl = "public, max-age=30"
)

// V2Handler serves v2 from the store: one snapshot per window, built and
// compressed once per poll, and each event in full.
type V2Handler struct {
	store *store.Store

	mu     sync.Mutex
	built  map[string]*builtSnapshot
	builds int // how many snapshots were built, for tests
}

type builtSnapshot struct {
	version uint64
	at      time.Time
	etag    string
	raw, gz []byte
}

func NewV2Handler(st *store.Store) *V2Handler {
	return &V2Handler{store: st, built: map[string]*builtSnapshot{}}
}

// Snapshot answers GET /api/v2/snapshot?window=7d|30d.
func (h *V2Handler) Snapshot(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	window := q.Get("window")
	if window == "" {
		window = defaultWindow
	}
	length, ok := snapshotWindows[window]
	if !ok || len(q) > 1 || (len(q) == 1 && len(q["window"]) != 1) {
		writeError(w, http.StatusBadRequest, "the snapshot takes only window=7d or window=30d")
		return
	}
	if !h.ready(w) {
		return
	}

	b, err := h.snapshot(window, length)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to build the snapshot")
		return
	}
	w.Header().Set("ETag", b.etag)
	w.Header().Set("Cache-Control", v2CacheControl)
	w.Header().Set("Vary", "Accept-Encoding")
	if etagMatches(r.Header.Get("If-None-Match"), b.etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	body := b.raw
	if acceptsGzip(r) {
		// Set before the status, so the compress middleware leaves it as is.
		w.Header().Set("Content-Encoding", "gzip")
		body = b.gz
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

// Event answers GET /api/v2/events/{id} with the event in full and its
// source's status.
func (h *V2Handler) Event(w http.ResponseWriter, r *http.Request) {
	if !h.ready(w) {
		return
	}
	id := chi.URLParam(r, "id")
	for _, c := range h.store.Read(time.Now()) {
		for _, e := range c.Events {
			if e.ID != id {
				continue
			}
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", v2CacheControl)
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(struct {
				Event  event.Event        `json:"event"`
				Source event.SourceStatus `json:"source"`
			}{e, c.Status})
			return
		}
	}
	writeError(w, http.StatusNotFound, "no event with that id is being served")
}

// ready answers 503 until every source has been polled once; that answer
// must never be kept by a cache.
func (h *V2Handler) ready(w http.ResponseWriter) bool {
	if h.store.Ready() {
		return true
	}
	w.Header().Set("Cache-Control", "no-store")
	writeError(w, http.StatusServiceUnavailable, "starting: not every source has been polled yet")
	return false
}

// snapshot returns the window's snapshot, building it again when a poll has
// been recorded since, or when it is older than rebuildAfter.
func (h *V2Handler) snapshot(window string, length time.Duration) (*builtSnapshot, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	now := time.Now()
	version := h.store.Version()
	if b := h.built[window]; b != nil && b.version == version && now.Sub(b.at) < rebuildAfter {
		return b, nil
	}

	snap := event.Snapshot{Generated: now.UTC(), Window: window, Sources: []event.SourceStatus{}, Events: []event.Event{}}
	cutoff := now.Add(-length)
	for _, c := range h.store.Read(now) {
		st := c.Status
		st.Count = 0
		for _, e := range c.Events {
			if e.Time.Updated.Before(cutoff) {
				continue
			}
			snap.Events = append(snap.Events, brief(e))
			st.Count++
		}
		snap.Sources = append(snap.Sources, st)
	}
	// Worked out on every build, on the snapshot's own copies.
	group.Assign(snap.Events)

	raw, err := json.Marshal(snap)
	if err != nil {
		return nil, err
	}
	// The tag leaves out generated, so a rebuild with nothing new keeps it.
	content, err := json.Marshal(struct {
		Window  string
		Sources []event.SourceStatus
		Events  []event.Event
	}{snap.Window, snap.Sources, snap.Events})
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(content)
	var gz bytes.Buffer
	zw := gzip.NewWriter(&gz)
	if _, err := zw.Write(raw); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	b := &builtSnapshot{version: version, at: now, etag: `W/"` + hex.EncodeToString(sum[:12]) + `"`, raw: raw, gz: gz.Bytes()}
	h.built[window] = b
	h.builds++
	return b, nil
}

// brief is the event as the snapshot carries it, without the long text that
// only the detail endpoint serves. The store's event is left untouched.
func brief(e event.Event) event.Event {
	e.Description = ""
	if e.Details.NOAA != nil && e.Details.NOAA.Instruction != "" {
		nws := *e.Details.NOAA
		nws.Instruction = ""
		e.Details.NOAA = &nws
	}
	return e
}

// etagMatches reads If-None-Match, which may list several tags or be "*",
// comparing weakly as a GET may.
func etagMatches(header, etag string) bool {
	want := strings.TrimPrefix(etag, "W/")
	for _, t := range strings.Split(header, ",") {
		t = strings.TrimSpace(t)
		if t == "*" || strings.TrimPrefix(t, "W/") == want {
			return true
		}
	}
	return false
}

// acceptsGzip says whether the request's Accept-Encoding takes gzip,
// honoring an explicit q=0.
func acceptsGzip(r *http.Request) bool {
	for _, part := range strings.Split(r.Header.Get("Accept-Encoding"), ",") {
		name, params, _ := strings.Cut(strings.TrimSpace(part), ";")
		if strings.TrimSpace(name) != "gzip" && strings.TrimSpace(name) != "*" {
			continue
		}
		q := strings.ReplaceAll(strings.TrimSpace(params), " ", "")
		return q != "q=0" && q != "q=0.0" && q != "q=0.00" && q != "q=0.000"
	}
	return false
}
