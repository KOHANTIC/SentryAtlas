package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// liveSources lists each source's recorded file, the list key in its
// response, and an adapter that reaches the real source through client.
var liveSources = []struct {
	source, key string
	adapter     func(client *http.Client) Adapter
}{
	{"usgs", "features", func(c *http.Client) Adapter { return NewUSGSAdapter(c) }},
	{"eonet", "events", func(c *http.Client) Adapter { return NewEONETAdapter(c) }},
	{"noaa", "features", func(c *http.Client) Adapter {
		return NewNOAAAdapter(c, "SentryAtlas/1.0 (github.com/KOHANTIC/SentryAtlas)")
	}},
	{"gdacs", "features", func(c *http.Client) Adapter { return NewGDACSAdapter(c) }},
}

// TestRecordedResponsesDecodeEveryRecord holds the adapters to what the
// sources really send: every record in testdata/live/ must become an event,
// none skipped as unreadable.
func TestRecordedResponsesDecodeEveryRecord(t *testing.T) {
	t.Parallel()
	for _, s := range liveSources {
		t.Run(s.source, func(t *testing.T) {
			t.Parallel()
			name := filepath.Join("live", s.source+".json")
			records := recordCount(t, filepath.Join("testdata", name), s.key)
			srv := serveFixture(t, name, nil)
			a := s.adapter(srv.Client())
			setBaseURL(a, srv.URL)

			events, err := a.FetchEvents(context.Background())
			if err != nil {
				t.Fatalf("FetchEvents: %v", err)
			}
			if len(events) != records {
				t.Errorf("got %d events from %d recorded records; some were skipped as unreadable", len(events), records)
			}
		})
	}
}

// recordCount is the number of records in the list under key in a JSON file.
func recordCount(t *testing.T, path, key string) int {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v (re-record with RECORD_LIVE=1)", path, err)
	}
	var envelope map[string][]json.RawMessage
	if err := json.Unmarshal(data, &envelope); err != nil {
		// Other top-level keys hold objects; read only the list.
		var loose map[string]json.RawMessage
		if err := json.Unmarshal(data, &loose); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		var list []json.RawMessage
		if err := json.Unmarshal(loose[key], &list); err != nil {
			t.Fatalf("parse %s list %q: %v", path, key, err)
		}
		return len(list)
	}
	return len(envelope[key])
}

// setBaseURL points an adapter at a test server.
func setBaseURL(a Adapter, url string) {
	switch x := a.(type) {
	case *USGSAdapter:
		x.baseURL = url
	case *EONETAdapter:
		x.baseURL = url
	case *NOAAAdapter:
		x.baseURL = url
	case *GDACSAdapter:
		x.baseURL = url
	}
}

// TestRecordLive re-records testdata/live/ from the real sources, through the
// adapters' own requests, keeping a small set of records that covers each
// kind the source sent. It needs the network, so it runs only when asked:
//
//	RECORD_LIVE=1 go test ./internal/adapters -run RecordLive -count=1
func TestRecordLive(t *testing.T) {
	if os.Getenv("RECORD_LIVE") == "" {
		t.Skip("set RECORD_LIVE=1 to re-record testdata/live/ from the real sources")
	}
	for _, s := range liveSources {
		t.Run(s.source, func(t *testing.T) {
			rt := &recordingTransport{}
			client := &http.Client{Timeout: 2 * time.Minute, Transport: rt}
			if _, err := s.adapter(client).FetchEvents(context.Background()); err != nil {
				t.Fatalf("FetchEvents: %v", err)
			}

			// A paged source sends several responses; their records are joined
			// under the first one's envelope. An empty body is a 204.
			var envelope map[string]any
			var records []any
			received := 0
			for _, body := range rt.bodies {
				received += len(body)
				if len(body) == 0 {
					continue
				}
				var page map[string]any
				if err := json.Unmarshal(body, &page); err != nil {
					t.Fatalf("parse response: %v", err)
				}
				if envelope == nil {
					envelope = page
				}
				list, _ := page[s.key].([]any)
				records = append(records, list...)
			}
			kept := trimRecords(s.source, records)
			envelope[s.key] = kept

			out, err := json.MarshalIndent(envelope, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join("testdata", "live", s.source+".json")
			if err := os.WriteFile(path, append(out, '\n'), 0o644); err != nil {
				t.Fatal(err)
			}
			t.Logf("%s: kept %d of %d records (%d bytes received in %d responses)", s.source, len(kept), len(records), received, len(rt.bodies))
		})
	}
}

// recordingTransport keeps a copy of every response body it passes on.
type recordingTransport struct {
	mu     sync.Mutex
	bodies [][]byte
}

func (rt *recordingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	resp, err := http.DefaultTransport.RoundTrip(r)
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, err
	}
	rt.mu.Lock()
	rt.bodies = append(rt.bodies, data)
	rt.mu.Unlock()
	resp.Body = io.NopCloser(bytes.NewReader(data))
	return resp, nil
}

// trimRecords keeps, in order, the first record for each kind a source's
// records show (event class, geometry kind, status, unit and the like), so
// the fixture is small but covers what the source sends.
func trimRecords(source string, records []any) []any {
	var keys []func(map[string]any) string
	switch source {
	case "usgs":
		keys = []func(map[string]any) string{
			func(r map[string]any) string { return "type:" + str(dig(r, "properties", "type")) },
			func(r map[string]any) string { return "status:" + str(dig(r, "properties", "status")) },
			func(r map[string]any) string { return "alert:" + str(dig(r, "properties", "alert")) },
			func(r map[string]any) string { return "tsunami:" + str(dig(r, "properties", "tsunami")) },
			func(r map[string]any) string { return "felt:" + fmt.Sprint(dig(r, "properties", "felt") != nil) },
		}
	case "eonet":
		keys = []func(map[string]any) string{
			func(r map[string]any) string { return "category:" + str(dig(r, "categories", 0, "id")) },
			func(r map[string]any) string {
				g, _ := r["geometry"].([]any)
				return "track:" + fmt.Sprint(len(g) > 1)
			},
			func(r map[string]any) string { return "unit:" + str(dig(r, "geometry", 0, "magnitudeUnit")) },
			func(r map[string]any) string { return "geometry:" + str(dig(r, "geometry", 0, "type")) },
		}
	case "noaa":
		keys = []func(map[string]any) string{
			func(r map[string]any) string { return "geometry:" + str(dig(r, "geometry", "type")) },
			func(r map[string]any) string {
				return "instruction:" + fmt.Sprint(dig(r, "properties", "instruction") != nil)
			},
			func(r map[string]any) string { return "severity:" + str(dig(r, "properties", "severity")) },
			func(r map[string]any) string { return "event:" + str(dig(r, "properties", "event")) },
		}
	case "gdacs":
		keys = []func(map[string]any) string{
			func(r map[string]any) string { return "type:" + str(dig(r, "properties", "eventtype")) },
			func(r map[string]any) string { return "alert:" + str(dig(r, "properties", "alertlevel")) },
			func(r map[string]any) string { return "geometry:" + str(dig(r, "geometry", "type")) },
			func(r map[string]any) string {
				return "unit:" + str(dig(r, "properties", "severitydata", "severityunit"))
			},
			func(r map[string]any) string { return "current:" + str(dig(r, "properties", "iscurrent")) },
		}
	}

	seen := map[string]bool{}
	var kept []any
	for _, rec := range records {
		r, ok := rec.(map[string]any)
		if !ok {
			continue
		}
		fresh := false
		for _, key := range keys {
			if k := key(r); !seen[k] {
				seen[k] = true
				fresh = true
			}
		}
		if fresh {
			kept = append(kept, rec)
		}
	}
	return kept
}

// dig walks nested JSON objects (string keys) and arrays (int indexes).
func dig(v any, path ...any) any {
	for _, p := range path {
		switch k := p.(type) {
		case string:
			m, ok := v.(map[string]any)
			if !ok {
				return nil
			}
			v = m[k]
		case int:
			a, ok := v.([]any)
			if !ok || k >= len(a) {
				return nil
			}
			v = a[k]
		}
	}
	return v
}

func str(v any) string {
	if v == nil {
		return "-"
	}
	return fmt.Sprint(v)
}
