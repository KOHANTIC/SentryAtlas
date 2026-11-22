package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/KOHANTIC/SentryAtlas/backend/internal/adapters"
	"github.com/KOHANTIC/SentryAtlas/backend/internal/handler"
	"github.com/KOHANTIC/SentryAtlas/backend/internal/service"
	"github.com/KOHANTIC/SentryAtlas/backend/internal/store"
)

// The v1 contract. -update rewrites the recorded responses, for a deliberate
// change of values reviewed in the diff. -update-shape rewrites shape.txt, the
// fields and their types: a change there is a change to the public API.
var (
	update      = flag.Bool("update", false, "rewrite testdata/v1/*.golden")
	updateShape = flag.Bool("update-shape", false, "rewrite testdata/v1/shape.txt, the v1 API's fields and types")
)

// fixtureFiles maps each upstream host to its recorded response in the adapters' testdata.
var fixtureFiles = map[string]string{
	"earthquake.usgs.gov": "usgs.json",
	"eonet.gsfc.nasa.gov": "eonet.json",
	"api.weather.gov":     "noaa.json",
	"www.gdacs.org":       "gdacs.json",
}

var allHosts = []string{"earthquake.usgs.gov", "eonet.gsfc.nasa.gov", "api.weather.gov", "www.gdacs.org"}

// fixtureTransport answers the adapters' upstream requests from the recorded
// files, and with a 500 for the hosts in failing, so the whole chain runs
// without the network.
type fixtureTransport struct{ failing map[string]bool }

func (t fixtureTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	file, ok := fixtureFiles[r.URL.Host]
	if !ok {
		return nil, fmt.Errorf("no fixture for upstream host %q", r.URL.Host)
	}
	if t.failing[r.URL.Host] {
		return &http.Response{
			StatusCode: http.StatusInternalServerError,
			Status:     "500 Internal Server Error",
			Header:     http.Header{},
			Body:       io.NopCloser(strings.NewReader("")),
			Request:    r,
		}, nil
	}
	body, err := os.ReadFile(filepath.Join("..", "adapters", "testdata", file))
	if err != nil {
		return nil, err
	}
	return &http.Response{
		StatusCode:    http.StatusOK,
		Status:        "200 OK",
		Header:        http.Header{"Content-Type": {"application/json"}},
		Body:          io.NopCloser(bytes.NewReader(body)),
		ContentLength: int64(len(body)),
		Request:       r,
	}, nil
}

// goldenFetchedAt is when the golden copies were fetched.
var goldenFetchedAt = time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)

// newV1Handler wires the production chain (adapters, store, service,
// handler) to the fixture transport, with the given hosts failing. The
// store is filled by one round of the adapters' fetches, as the pollers'
// first round would, at goldenFetchedAt.
func newV1Handler(t *testing.T, failing []string) *handler.EventsHandler {
	t.Helper()
	down := map[string]bool{}
	for _, h := range failing {
		down[h] = true
	}
	client := &http.Client{Transport: fixtureTransport{failing: down}, Timeout: 5 * time.Second}
	list := []adapters.Adapter{
		adapters.NewUSGSAdapter(client),
		adapters.NewEONETAdapter(client),
		adapters.NewNOAAAdapter(client, "SentryAtlas-test"),
		adapters.NewGDACSAdapter(client),
	}
	var sources []store.Source
	for _, a := range list {
		sources = append(sources, store.Source{Name: a.Source()})
	}
	st := store.New(sources...)
	for _, a := range list {
		events, err := a.(store.Fetcher).FetchV2(context.Background())
		if err != nil {
			st.Fail(a.Source(), adapters.Reason(err))
		} else if err := st.Put(a.Source(), events, goldenFetchedAt); err != nil {
			t.Fatal(err)
		}
	}
	return handler.NewEventsHandler(service.NewEventsService(list, st))
}

// recorded is one response in canonical form, as stored in a golden file.
type recorded struct {
	Status       int     `json:"status"`
	ContentType  string  `json:"content_type"`
	CacheControl string  `json:"cache_control,omitempty"`
	Body         any     `json:"body,omitempty"`
	Frames       []frame `json:"frames,omitempty"`
}

type frame struct {
	Event string `json:"event"`
	Data  any    `json:"data"`
}

var v1Cases = []struct {
	name    string
	query   string
	failing []string
}{
	{"geojson", "", nil},
	{"json", "format=json", nil},
	{"sse", "format=sse", nil},
	// Four events match the types and box; the limit cuts the fourth, where
	// no two events share a start time.
	{"geojson-filtered", "types=earthquake,flood,storm&bbox=-180,0,0,90&limit=3", nil},
	{"json-since", "format=json&since=2026-08-09", nil},
	{"bad-type", "types=meteor", nil},
	{"bad-bbox", "bbox=1,2,3", nil},
	{"json-all-failed", "format=json", allHosts},
	{"sse-all-failed", "format=sse", allHosts},
	{"geojson-noaa-failed", "", []string{"api.weather.gov"}},
	{"json-noaa-failed", "format=json", []string{"api.weather.gov"}},
}

func TestV1Golden(t *testing.T) {
	// USGS times are converted in the server's local zone; production runs in
	// UTC, so the recorded responses do too, whatever machine runs the test.
	local := time.Local
	time.Local = time.UTC
	t.Cleanup(func() { time.Local = local })

	shape := map[string]map[string]bool{}

	for _, c := range v1Cases {
		t.Run(c.name, func(t *testing.T) {
			h := newV1Handler(t, c.failing)
			req := httptest.NewRequest(http.MethodGet, "/api/v1/events?"+c.query, nil)
			rec := httptest.NewRecorder()
			h.GetEvents(rec, req)

			got := recorded{
				Status:       rec.Code,
				ContentType:  rec.Header().Get("Content-Type"),
				CacheControl: rec.Header().Get("Cache-Control"),
			}
			if strings.HasPrefix(got.ContentType, "text/event-stream") {
				got.Frames = sseFrames(t, rec.Body.String())
				for _, f := range got.Frames {
					addShape(shape, "sse."+f.Event, f.Data)
				}
			} else {
				got.Body = decode(t, rec.Body.Bytes())
				normalize(t, got.Body)
				addShape(shape, shapeRoot(got), got.Body)
			}

			out, err := json.MarshalIndent(got, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			compareGolden(t, filepath.Join("testdata", "v1", c.name+".golden"), append(out, '\n'), *update, "-update")
		})
	}

	compareGolden(t, filepath.Join("testdata", "v1", "shape.txt"), []byte(renderShape(shape)), *updateShape, "-update-shape")
}

// shapeRoot names the response kind a body's fields are recorded under.
func shapeRoot(r recorded) string {
	switch {
	case r.Status >= 400:
		return "error"
	case strings.HasPrefix(r.ContentType, "application/geo+json"):
		return "geojson"
	default:
		return "json"
	}
}

// decode parses a JSON body keeping every number exactly as written.
func decode(t *testing.T, data []byte) any {
	t.Helper()
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	var v any
	if err := d.Decode(&v); err != nil {
		t.Fatalf("body is not JSON: %v: %q", err, data)
	}
	return v
}

// sseFrames splits a stream into its frames, dropping keep-alive comments,
// requiring done to come last, and ordering the features frames by source,
// since sources answer in any order.
func sseFrames(t *testing.T, body string) []frame {
	t.Helper()
	var frames []frame
	for _, block := range strings.Split(body, "\n\n") {
		var f frame
		var data string
		for _, line := range strings.Split(block, "\n") {
			switch {
			case strings.HasPrefix(line, "event: "):
				f.Event = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				data = strings.TrimPrefix(line, "data: ")
			}
		}
		if f.Event == "" {
			continue
		}
		f.Data = decode(t, []byte(data))
		normalize(t, f.Data)
		frames = append(frames, f)
	}
	if len(frames) == 0 || frames[len(frames)-1].Event != "done" {
		t.Fatalf("stream does not end with a done frame: %q", body)
	}
	for _, f := range frames[:len(frames)-1] {
		if f.Event != "features" {
			t.Fatalf("unexpected %q frame before done", f.Event)
		}
	}
	batches := frames[:len(frames)-1]
	sort.SliceStable(batches, func(i, j int) bool { return frameSource(batches[i]) < frameSource(batches[j]) })
	return frames
}

// frameSource is the source of a features frame's first event.
func frameSource(f frame) string {
	features, _ := f.Data.(map[string]any)["features"].([]any)
	if len(features) == 0 {
		return ""
	}
	props, _ := features[0].(map[string]any)["properties"].(map[string]any)
	s, _ := props["source"].(string)
	return s
}

// normalize removes what varies between identical requests: the order of
// sources (the order they answered), the wording of upstream errors, and the
// order of events that start at the same moment (the sort is not stable).
// It first checks that events run newest first, so a broken sort still fails.
func normalize(t *testing.T, v any) {
	t.Helper()
	m, ok := v.(map[string]any)
	if !ok {
		return
	}
	if sources, ok := m["sources"].([]any); ok {
		for _, s := range sources {
			if sm, ok := s.(map[string]any); ok {
				if _, has := sm["error"]; has {
					sm["error"] = "<upstream error text>"
				}
			}
		}
		sort.SliceStable(sources, func(i, j int) bool {
			return sources[i].(map[string]any)["source"].(string) < sources[j].(map[string]any)["source"].(string)
		})
	}
	for _, key := range []string{"features", "events"} {
		list, ok := m[key].([]any)
		if !ok {
			continue
		}
		event := func(i int) map[string]any {
			e := list[i].(map[string]any)
			if p, ok := e["properties"].(map[string]any); ok {
				return p
			}
			return e
		}
		started := func(i int) time.Time {
			s, _ := event(i)["started_at"].(string)
			ts, err := time.Parse(time.RFC3339Nano, s)
			if err != nil {
				t.Fatalf("%s[%d].started_at %q: %v", key, i, s, err)
			}
			return ts
		}
		for i := 1; i < len(list); i++ {
			if started(i).After(started(i - 1)) {
				t.Fatalf("%s are not newest first at index %d", key, i)
			}
		}
		times := make([]time.Time, len(list))
		ids := make([]string, len(list))
		for i := range list {
			times[i] = started(i)
			ids[i], _ = event(i)["id"].(string)
		}
		idx := make([]int, len(list))
		for i := range idx {
			idx[i] = i
		}
		sort.SliceStable(idx, func(a, b int) bool {
			if !times[idx[a]].Equal(times[idx[b]]) {
				return times[idx[a]].After(times[idx[b]])
			}
			return ids[idx[a]] < ids[idx[b]]
		})
		ordered := make([]any, len(list))
		for i, j := range idx {
			ordered[i] = list[j]
		}
		m[key] = ordered
	}
}

// addShape records the JSON type of every path in v under prefix, with "[]"
// standing for any element of an array.
func addShape(shape map[string]map[string]bool, prefix string, v any) {
	kind := "null"
	switch x := v.(type) {
	case map[string]any:
		kind = "object"
		for k, child := range x {
			addShape(shape, prefix+"."+k, child)
		}
	case []any:
		kind = "array"
		for _, child := range x {
			addShape(shape, prefix+"[]", child)
		}
	case string:
		kind = "string"
	case json.Number:
		kind = "number"
	case bool:
		kind = "bool"
	}
	if shape[prefix] == nil {
		shape[prefix] = map[string]bool{}
	}
	shape[prefix][kind] = true
}

// renderShape writes one sorted line per path: "path: type|type".
func renderShape(shape map[string]map[string]bool) string {
	paths := make([]string, 0, len(shape))
	for p := range shape {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	var b strings.Builder
	for _, p := range paths {
		kinds := make([]string, 0, len(shape[p]))
		for k := range shape[p] {
			kinds = append(kinds, k)
		}
		sort.Strings(kinds)
		fmt.Fprintf(&b, "%s: %s\n", p, strings.Join(kinds, "|"))
	}
	return b.String()
}

// compareGolden checks got against the file at path, or rewrites the file
// when write is set.
func compareGolden(t *testing.T, path string, got []byte, write bool, flagName string) {
	t.Helper()
	if write {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (create it with go test ./internal/handler -run Golden %s)", err, flagName)
	}
	if bytes.Equal(got, want) {
		return
	}
	gotLines := strings.Split(string(got), "\n")
	wantLines := strings.Split(string(want), "\n")
	for i := 0; i < len(gotLines) || i < len(wantLines); i++ {
		var g, w string
		if i < len(gotLines) {
			g = gotLines[i]
		}
		if i < len(wantLines) {
			w = wantLines[i]
		}
		if g != w {
			t.Errorf("%s differs at line %d\n want: %s\n  got: %s\nIf the change is intended, rerun with %s and review the diff.", path, i+1, w, g, flagName)
			return
		}
	}
}
