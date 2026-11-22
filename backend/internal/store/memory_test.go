package store

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/KOHANTIC/SentryAtlas/backend/internal/adapters"
)

// The full window on 2026-11-22.
var fullSize = []struct {
	host, file, key string
	n               int
	// setID makes record i distinct, so it is its own event.
	setID func(r map[string]any, i int)
}{
	{"earthquake.usgs.gov", "usgs.json", "features", 9444, func(r map[string]any, i int) { r["id"] = fmt.Sprint("us", i) }},
	{"eonet.gsfc.nasa.gov", "eonet.json", "events", 7257, func(r map[string]any, i int) { r["id"] = fmt.Sprint("EONET_", i) }},
	{"api.weather.gov", "noaa.json", "features", 346, func(r map[string]any, i int) {
		p := r["properties"].(map[string]any)
		p["id"] = fmt.Sprint("urn:oid:test.", i)
		p["references"] = []any{}
		p["parameters"] = map[string]any{}
	}},
	{"www.gdacs.org", "gdacs.json", "features", 1665, func(r map[string]any, i int) { r["properties"].(map[string]any)["eventid"] = i }},
}

// fullSizeTransport answers each source's request with its recorded records
// repeated to full size; GDACS gets them all on page 1, then 204.
type fullSizeTransport map[string][]byte

func (t fullSizeTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	body, ok := t[r.URL.Host]
	if !ok {
		return nil, fmt.Errorf("no fixture for %s", r.URL.Host)
	}
	if p := r.URL.Query().Get("pagenumber"); p != "" && p != "1" {
		return &http.Response{StatusCode: http.StatusNoContent, Body: http.NoBody, Request: r}, nil
	}
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(body)), Request: r}, nil
}

func fullSizeResponses(tb testing.TB) fullSizeTransport {
	tb.Helper()
	out := fullSizeTransport{}
	for _, s := range fullSize {
		data, err := os.ReadFile(filepath.Join("..", "adapters", "testdata", "live", s.file))
		if err != nil {
			tb.Fatal(err)
		}
		var envelope map[string]any
		if err := json.Unmarshal(data, &envelope); err != nil {
			tb.Fatal(err)
		}
		seed, _ := envelope[s.key].([]any)
		list := make([]any, s.n)
		for i := range list {
			var r map[string]any
			b, _ := json.Marshal(seed[i%len(seed)])
			if err := json.Unmarshal(b, &r); err != nil {
				tb.Fatal(err)
			}
			s.setID(r, i)
			list[i] = r
		}
		envelope[s.key] = list
		if out[s.host], err = json.Marshal(envelope); err != nil {
			tb.Fatal(err)
		}
	}
	return out
}

// pollFullSize runs one round of every poller against full-size responses
// and returns the store, the heap it keeps, and the heap's peak during the
// round, both above where it started.
func pollFullSize(tb testing.TB, responses fullSizeTransport) (s *Store, kept, peak uint64) {
	tb.Helper()
	client := &http.Client{Transport: responses}
	polls := []Poll{
		{adapters.NewUSGSAdapter(client), time.Hour},
		{adapters.NewEONETAdapter(client), time.Hour},
		{adapters.NewNOAAAdapter(client, "SentryAtlas-test"), time.Hour},
		{adapters.NewGDACSAdapter(client), time.Hour},
	}
	s = New(Source{Name: "usgs"}, Source{Name: "eonet"}, Source{Name: "noaa", MaxAge: time.Hour}, Source{Name: "gdacs"})

	var ms runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&ms)
	base := ms.HeapAlloc
	var high atomic.Uint64
	stop := make(chan struct{})
	sampled := make(chan struct{})
	go func() {
		defer close(sampled)
		var m runtime.MemStats
		for {
			runtime.ReadMemStats(&m)
			if m.HeapAlloc > high.Load() {
				high.Store(m.HeapAlloc)
			}
			select {
			case <-stop:
				return
			case <-time.After(2 * time.Millisecond):
			}
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	wait := Run(ctx, s, polls, Config{Timeout: time.Minute, RetryAfter: time.Hour})
	for !s.Ready() {
		time.Sleep(time.Millisecond)
	}
	cancel()
	wait()
	close(stop)
	<-sampled

	runtime.GC()
	runtime.ReadMemStats(&ms)
	// The responses were on the heap at the start; freed now, they would
	// hide what the store keeps.
	runtime.KeepAlive(responses)
	if ms.HeapAlloc > base {
		kept = ms.HeapAlloc - base
	}
	if h := high.Load(); h > base {
		peak = h - base
	}
	return s, kept, peak
}

// The instance has 0.5 GB, and until #22 v1's request path holds a second
// copy. Measured on 2026-11-22: 9 to 12 MB kept and 41 to 48 MB at the
// peak; the budgets leave room for real records larger than the recorded
// ones, and fail on a regression several times over.
const (
	keptBudget = 60 << 20
	peakBudget = 150 << 20
)

func TestAFullWindowFitsTheMemoryBudget(t *testing.T) {
	s, kept, peak := pollFullSize(t, fullSizeResponses(t))
	total := 0
	for _, c := range s.Read(time.Now()) {
		if !c.Status.OK {
			t.Errorf("%s: %+v, want ok", c.Status.Source, c.Status)
		}
		total += c.Status.Count
	}
	if total < 18_000 {
		t.Errorf("%d events stored, want the full window, about 18,600", total)
	}
	t.Logf("%d events: %.0f MB kept, %.0f MB peak while polling", total, float64(kept)/(1<<20), float64(peak)/(1<<20))
	if kept > keptBudget {
		t.Errorf("the store keeps %d MB, over the %d MB budget", kept>>20, keptBudget>>20)
	}
	if peak > peakBudget {
		t.Errorf("polling peaked at %d MB, over the %d MB budget", peak>>20, peakBudget>>20)
	}
	runtime.KeepAlive(s)
}

func BenchmarkFullSizeRound(b *testing.B) {
	responses := fullSizeResponses(b)
	defer slog.SetDefault(slog.Default())
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	b.ResetTimer()
	for range b.N {
		s, kept, peak := pollFullSize(b, responses)
		b.ReportMetric(float64(kept)/(1<<20), "MB-kept")
		b.ReportMetric(float64(peak)/(1<<20), "MB-peak")
		runtime.KeepAlive(s)
	}
}
