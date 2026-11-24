package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/KOHANTIC/SentryAtlas/backend/internal/event"
)

// footprintServer serves one page of GDACS events at / and each event
// type's footprint fixture (testdata/gdacs, live on 2026-11-23) at
// /fp/<type>, counting the footprint requests per type. A type in fail
// answers 503.
type footprintServer struct {
	srv    *httptest.Server
	mu     sync.Mutex
	asked  map[string]int
	fail   map[string]bool
	events []string
}

func newFootprintServer(t *testing.T) *footprintServer {
	t.Helper()
	fs := &footprintServer{asked: map[string]int{}, fail: map[string]bool{}}
	fs.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if code, ok := strings.CutPrefix(r.URL.Path, "/fp/"); ok {
			fs.mu.Lock()
			fs.asked[code]++
			fail := fs.fail[code]
			fs.mu.Unlock()
			if fail {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			data, err := os.ReadFile(filepath.Join("testdata", "gdacs", "footprint_"+code+".json"))
			if err != nil {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Write(data)
			return
		}
		if r.URL.Query().Get("pagenumber") != "1" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		fs.mu.Lock()
		body := `{"type":"FeatureCollection","features":[` + strings.Join(fs.events, ",") + `]}`
		fs.mu.Unlock()
		w.Write([]byte(body))
	}))
	t.Cleanup(fs.srv.Close)
	return fs
}

// event lists one event of a GDACS type, with its footprint on this server
// unless link is given.
func (fs *footprintServer) event(code string, id, episode int, current bool, link string) {
	if link == "" {
		link = fs.srv.URL + "/fp/" + code
	}
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.events = append(fs.events, fmt.Sprintf(`{"geometry":{"type":"Point","coordinates":[10,20]},"properties":{"eventtype":%q,"eventid":%d,"episodeid":%d,"name":"An event","alertlevel":"Orange","alertscore":2,"iscurrent":%q,"country":"Somewhere","fromdate":"2026-11-20T00:00:00","todate":"2026-11-21T00:00:00","datemodified":"2026-11-21T00:00:00","severitydata":{"severity":0,"severitytext":"","severityunit":""},"url":{"report":"r","geometry":%q}}}`,
		code, id, episode, strconv.FormatBool(current), link))
}

func (fs *footprintServer) setFail(code string, fail bool) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.fail[code] = fail
}

func (fs *footprintServer) requests(code string) int {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return fs.asked[code]
}

func (fs *footprintServer) adapter(now time.Time) *GDACSAdapter {
	a := NewGDACSAdapter(fs.srv.Client())
	a.baseURL = fs.srv.URL
	a.now = func() time.Time { return now }
	return a
}

func fetchByID(t *testing.T, a *GDACSAdapter) map[string]event.Event {
	t.Helper()
	events, err := a.FetchV2(context.Background())
	if err != nil {
		t.Fatalf("FetchV2: %v", err)
	}
	byID := map[string]event.Event{}
	for _, e := range events {
		byID[e.ID] = e
	}
	return byID
}

// vertices counts a Polygon's or MultiPolygon's points.
func vertices(t *testing.T, g *event.Geometry) int {
	t.Helper()
	var polys [][][]event.Point
	if g.Type == "Polygon" {
		var rings [][]event.Point
		if err := json.Unmarshal(g.Coordinates, &rings); err != nil {
			t.Fatal(err)
		}
		polys = [][][]event.Point{rings}
	} else if err := json.Unmarshal(g.Coordinates, &polys); err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, p := range polys {
		for _, r := range p {
			n += len(r)
		}
	}
	return n
}

var footprintNow = time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)

func TestFootprintsDrawCyclonesFloodsAndDroughtsAsAreas(t *testing.T) {
	t.Parallel()
	fs := newFootprintServer(t)
	for i, code := range []string{"TC", "FL", "DR", "WF", "EQ", "VO"} {
		fs.event(code, i+1, 1, true, "")
	}
	byID := fetchByID(t, fs.adapter(footprintNow))

	for _, id := range []string{"gdacs-TC-1", "gdacs-FL-2", "gdacs-DR-3"} {
		e := byID[id]
		if err := e.Validate(); err != nil {
			t.Errorf("%s: %v", id, err)
		}
		g := e.Location.Geometry
		if g == nil || (g.Type != "Polygon" && g.Type != "MultiPolygon") {
			t.Fatalf("%s geometry = %+v, want its area", id, g)
		}
		if n := vertices(t, g); n > event.MaxVertices || n < 4 {
			t.Errorf("%s has %d vertices, want a simplified area under %d", id, n, event.MaxVertices)
		}
		if *e.Location.Center != (event.Point{10, 20}) {
			t.Errorf("%s center moved to %v; the mark stays where GDACS puts the event", id, *e.Location.Center)
		}
	}
	// The cyclone's area is its 60 km/h swath, which reaches 87.1° E; the
	// circles around its positions stay west of 81° E.
	var swath [][]event.Point
	if err := json.Unmarshal(byID["gdacs-TC-1"].Location.Geometry.Coordinates, &swath); err != nil {
		t.Fatalf("the cyclone's area is not one polygon: %v", err)
	}
	east := -180.0
	for _, p := range swath[0] {
		east = max(east, p[0])
	}
	if east < 87 {
		t.Errorf("the cyclone's area reaches %.2f° E, want the 60 km/h swath to 87.1° E", east)
	}
	for _, id := range []string{"gdacs-WF-4", "gdacs-EQ-5", "gdacs-VO-6"} {
		if g := byID[id].Location.Geometry; g == nil || g.Type != "Point" {
			t.Errorf("%s geometry = %+v, want its point", id, g)
		}
	}
	for _, code := range []string{"WF", "EQ", "VO"} {
		if n := fs.requests(code); n != 0 {
			t.Errorf("%s footprint asked %d times, want never", code, n)
		}
	}
}

func TestFootprintsAreFetchedOncePerEpisode(t *testing.T) {
	t.Parallel()
	fs := newFootprintServer(t)
	fs.event("TC", 1, 10, true, "")
	a := fs.adapter(footprintNow)
	fetchByID(t, a)
	second := fetchByID(t, a)
	if n := fs.requests("TC"); n != 1 {
		t.Errorf("asked %d times over two polls, want once for one episode", n)
	}
	if g := second["gdacs-TC-1"].Location.Geometry; g == nil || g.Type == "Point" {
		t.Errorf("the second poll lost the area: %+v", g)
	}

	fs.mu.Lock()
	fs.events = nil
	fs.mu.Unlock()
	fs.event("TC", 1, 11, true, "")
	fetchByID(t, a)
	if n := fs.requests("TC"); n != 2 {
		t.Errorf("asked %d times, want once more for the new episode", n)
	}
}

func TestAFailedFootprintKeepsThePointAndWaitsAnHour(t *testing.T) {
	t.Parallel()
	fs := newFootprintServer(t)
	fs.setFail("FL", true)
	fs.event("FL", 2, 1, true, "")
	a := fs.adapter(footprintNow)
	if g := fetchByID(t, a)["gdacs-FL-2"].Location.Geometry; g == nil || g.Type != "Point" {
		t.Errorf("geometry = %+v, want the point while the footprint fails", g)
	}
	fetchByID(t, a)
	if n := fs.requests("FL"); n != 1 {
		t.Errorf("asked %d times, want no retry within the hour", n)
	}
	a.now = func() time.Time { return footprintNow.Add(61 * time.Minute) }
	fs.setFail("FL", false)
	if g := fetchByID(t, a)["gdacs-FL-2"].Location.Geometry; g == nil || g.Type == "Point" {
		t.Errorf("geometry = %+v, want the area once the retry succeeds", g)
	}
}

func TestAPollFetchesAtMostItsShareOfFootprints(t *testing.T) {
	t.Parallel()
	fs := newFootprintServer(t)
	for i := range footprintsPerPoll + 5 {
		fs.event("FL", 100+i, 1, false, "")
	}
	a := fs.adapter(footprintNow)
	fetchByID(t, a)
	if n := fs.requests("FL"); n != footprintsPerPoll {
		t.Errorf("first poll asked %d, want %d", n, footprintsPerPoll)
	}
	fetchByID(t, a)
	if n := fs.requests("FL"); n != footprintsPerPoll+5 {
		t.Errorf("after the second poll %d asked, want all %d", n, footprintsPerPoll+5)
	}
}

func TestAFootprintOnAnotherHostIsNeverFollowed(t *testing.T) {
	t.Parallel()
	fs := newFootprintServer(t)
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("followed a footprint link to another host: %s", r.URL)
	}))
	t.Cleanup(other.Close)
	fs.event("DR", 3, 1, true, other.URL+"/fp/DR")
	if g := fetchByID(t, fs.adapter(footprintNow))["gdacs-DR-3"].Location.Geometry; g == nil || g.Type != "Point" {
		t.Errorf("geometry = %+v, want the point", g)
	}
}

func TestSimplifiedAreaFitsTheVertexLimit(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(filepath.Join("testdata", "gdacs", "footprint_DR.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fc struct{ Features []footprintFeature }
	if err := json.Unmarshal(data, &fc); err != nil {
		t.Fatal(err)
	}
	for _, f := range fc.Features {
		if f.Properties.Class != "Poly_area" {
			continue
		}
		// 5,527 points, still 1,185 at 1 km: the tolerance doubles until it fits.
		g := simplifiedArea(f.Geometry.polygons())
		if g == nil {
			t.Fatal("the drought's area simplified away")
		}
		if n := vertices(t, g); n > event.MaxVertices {
			t.Errorf("%d vertices, want at most %d", n, event.MaxVertices)
		}
		return
	}
	t.Fatal("no Poly_area in the drought fixture")
}
