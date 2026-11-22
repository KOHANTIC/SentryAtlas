package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// gdacsRecord builds one GDACS event as the SEARCH endpoint lists it.
func gdacsRecord(id int, modified string) string {
	return fmt.Sprintf(`{"geometry":{"type":"Point","coordinates":[10,20]},"properties":{"eventtype":"WF","eventid":%d,"episodeid":1,"name":"Forest fire","alertlevel":"Green","alertscore":1,"iscurrent":"true","country":"Spain","fromdate":"2026-11-20T00:00:00","todate":"2026-11-21T00:00:00","datemodified":%q,"severitydata":{"severity":120,"severitytext":"120 ha","severityunit":"ha"},"url":{"report":"r","geometry":"g"}}}`, id, modified)
}

// servePages serves GDACS pages by their pagenumber, 204 past the last, and
// counts the pages asked for; a page listed in fail answers 503.
func servePages(t *testing.T, pages [][]string, fail map[int]bool) (*httptest.Server, func() []int) {
	t.Helper()
	var mu sync.Mutex
	var asked []int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n, _ := strconv.Atoi(r.URL.Query().Get("pagenumber"))
		mu.Lock()
		asked = append(asked, n)
		mu.Unlock()
		switch {
		case fail[n]:
			w.WriteHeader(http.StatusServiceUnavailable)
		case n < 1 || n > len(pages):
			w.WriteHeader(http.StatusNoContent)
		default:
			w.Write([]byte(`{"type":"FeatureCollection","features":[` + strings.Join(pages[n-1], ",") + `]}`))
		}
	}))
	t.Cleanup(srv.Close)
	return srv, func() []int {
		mu.Lock()
		defer mu.Unlock()
		out := slices.Clone(asked)
		slices.Sort(out)
		return out
	}
}

func TestGDACSReadsEveryPage(t *testing.T) {
	t.Parallel()
	// Six pages, more than one wave; event 2 is listed again on page 6 with
	// a later modification, as GDACS does.
	var pages [][]string
	for p := range 6 {
		pages = append(pages, []string{gdacsRecord(10*p+1, "2026-11-21T00:00:00"), gdacsRecord(10*p+2, "2026-11-21T00:00:00")})
	}
	pages[5] = append(pages[5], gdacsRecord(2, "2026-11-22T09:00:00"))
	srv, asked := servePages(t, pages, nil)
	a := newTestGDACS(t, srv)

	events, err := a.FetchV2(context.Background())
	if err != nil {
		t.Fatalf("FetchV2: %v", err)
	}
	if len(events) != 12 {
		t.Fatalf("got %d events, want 12: every page, the repeated event once", len(events))
	}
	for _, e := range events {
		if e.ID == "gdacs-WF-2" && e.Time.Updated.Format("15:04") != "09:00" {
			t.Errorf("repeated event updated %v, want its later record", e.Time.Updated)
		}
		if err := e.Validate(); err != nil {
			t.Errorf("%s: %v", e.ID, err)
		}
	}
	if got := asked(); !slices.Equal(got, []int{1, 2, 3, 4, 5, 6, 7, 8}) {
		t.Errorf("pages asked %v, want 1 to 8: two waves of four, ending at the first empty page", got)
	}
}

func TestGDACSOneFailedPageFailsTheWindow(t *testing.T) {
	t.Parallel()
	pages := [][]string{{gdacsRecord(1, "")}, {gdacsRecord(2, "")}, {gdacsRecord(3, "")}, {gdacsRecord(4, "")}, {gdacsRecord(5, "")}}
	srv, _ := servePages(t, pages, map[int]bool{5: true})
	a := newTestGDACS(t, srv)
	if events, err := a.FetchV2(context.Background()); err == nil {
		t.Fatalf("got %d events and no error, want the failed page to fail the window rather than serve it short", len(events))
	}
}

// A server that ignores the page number would otherwise be read 50 times.
func TestGDACSStopsWhenPagesRepeat(t *testing.T) {
	t.Parallel()
	var capture reqCapture
	srv := serveFixture(t, "gdacs.json", &capture)
	var mu sync.Mutex
	requests := 0
	counting := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests++
		mu.Unlock()
		srv.Config.Handler.ServeHTTP(w, r)
	}))
	t.Cleanup(counting.Close)
	a := newTestGDACS(t, counting)

	events, err := a.FetchV2(context.Background())
	if err != nil || len(events) != 3 {
		t.Fatalf("got %d events, err %v; want the fixture's 3", len(events), err)
	}
	if requests != gdacsParallelPages {
		t.Errorf("%d requests, want one wave of %d", requests, gdacsParallelPages)
	}
}

func TestGDACSV2Fields(t *testing.T) {
	t.Parallel()
	var f gdacsFeature
	body := `{"geometry":{"type":"Polygon","coordinates":[[[0,0],[4,0],[4,2],[0,2],[0,0]]]},"properties":{"eventtype":"FL","eventid":7,"episodeid":3,"name":"Flood in Spain","alertlevel":"Orange","alertscore":2,"iscurrent":"false","affectedcountries":[{"countryname":"Spain"},{"countryname":"France"}],"fromdate":"2026-11-01T00:00:00","todate":"2026-11-05T00:00:00","datemodified":"2026-11-06T12:00:00","severitydata":{"severity":0,"severitytext":"Magnitude 1.5","severityunit":""},"url":{"report":"r","geometry":"https://www.gdacs.org/gdacsapi/api/polygons/getgeometry?eventtype=FL&eventid=7"}}}`
	if err := json.Unmarshal([]byte(body), &f); err != nil {
		t.Fatal(err)
	}
	e := gdacsV2(f)
	if err := e.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	d := e.Details.GDACS
	if d.EpisodeID != 3 || d.AlertScore == nil || *d.AlertScore != 2 || d.Current || d.SeverityText != "Magnitude 1.5" || !slices.Equal(d.Countries, []string{"Spain", "France"}) || !strings.Contains(d.FootprintURL, "getgeometry") {
		t.Errorf("details = %+v", d)
	}
	if e.Magnitude != nil {
		t.Errorf("magnitude = %+v, want none: a flood has no severity unit", e.Magnitude)
	}
	if e.Time.Ends == nil || e.Time.Ends.Day() != 5 || e.Time.Updated.Day() != 6 {
		t.Errorf("time = %+v, want ends at todate and updated at datemodified", e.Time)
	}
	if e.Location == nil || e.Location.Geometry == nil || e.Location.Geometry.Type != "Polygon" || *e.Location.Center != [2]float64{2, 1} {
		t.Errorf("location = %+v, want the polygon centered on its area", e.Location)
	}
}

// TestGDACSV2FromTheRealEvents maps the recorded events onto v2: all valid,
// each severity with its unit, and every event's countries.
func TestGDACSV2FromTheRealEvents(t *testing.T) {
	t.Parallel()
	a := newTestGDACS(t, serveFixture(t, "live/gdacs.json", nil))
	events, err := a.FetchV2(context.Background())
	if err != nil {
		t.Fatalf("FetchV2: %v", err)
	}
	units := map[string]bool{}
	levels := map[string]bool{}
	for _, e := range events {
		if err := e.Validate(); err != nil {
			t.Errorf("%s: %v", e.ID, err)
		}
		if e.Magnitude != nil {
			units[e.Magnitude.Unit] = true
		}
		levels[e.Details.GDACS.AlertLevel] = true
		// Severity text is missing for volcanoes, so it is not required here.
		if e.Details.GDACS.EpisodeID == 0 || e.Details.GDACS.FootprintURL == "" || e.Time.Ends == nil {
			t.Errorf("%s: details %+v, ends %v; every recorded event has them", e.ID, e.Details.GDACS, e.Time.Ends)
		}
	}
	for _, u := range []string{"M", "ha", "km/h", "km2"} {
		if !units[u] {
			t.Errorf("no severity in %q; the recording holds one", u)
		}
	}
	if !levels["Green"] {
		t.Error("no Green alert; the window includes them")
	}
}
