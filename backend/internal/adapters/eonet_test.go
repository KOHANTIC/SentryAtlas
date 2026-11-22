package adapters

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"
)

func newTestEONET(t *testing.T, srv *httptest.Server) *EONETAdapter {
	t.Helper()
	a := NewEONETAdapter(srv.Client())
	a.baseURL = srv.URL
	return a
}

func TestEONETSourceAndSupportedTypes(t *testing.T) {
	t.Parallel()
	a := NewEONETAdapter(nil)
	if got := a.Source(); got != "eonet" {
		t.Errorf("Source() = %q, want %q", got, "eonet")
	}
	want := []string{"wildfire", "volcano", "storm", "iceberg", "earthquake", "flood", "landslide", "drought", "other"}
	if got := a.SupportedTypes(); !slices.Equal(got, want) {
		t.Errorf("SupportedTypes() = %v, want %v", got, want)
	}
}

func TestEONETFetchEventsParsing(t *testing.T) {
	t.Parallel()
	srv := serveFixture(t, "eonet.json", nil)
	a := newTestEONET(t, srv)

	events, err := a.FetchEvents(context.Background())
	if err != nil {
		t.Fatalf("FetchEvents: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("got %d events, want 3", len(events))
	}

	t.Run("category mapping and last geometry wins", func(t *testing.T) {
		e := eventByID(t, events, "eonet-EONET_10001")
		if e.EventType != "wildfire" {
			t.Errorf("EventType = %q, want wildfire", e.EventType)
		}
		if e.Source != "eonet" {
			t.Errorf("Source = %q, want eonet", e.Source)
		}
		if e.Title != "Creek Fire, Fresno County, California" {
			t.Errorf("Title = %q", e.Title)
		}
		// The fixture has two geometry entries; the last one must win.
		if !slices.Equal(e.Geometry.Coordinates, []float64{-119.31, 37.22}) {
			t.Errorf("Coordinates = %v, want last geometry [-119.31 37.22]", e.Geometry.Coordinates)
		}
		// Started is the first observation, updated the latest (#19); both
		// used to be the latest.
		if want := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC); !e.StartedAt.Equal(want) {
			t.Errorf("StartedAt = %v, want the first observation %v", e.StartedAt, want)
		}
		if want := time.Date(2026, 8, 6, 10, 0, 0, 0, time.UTC); !e.UpdatedAt.Equal(want) {
			t.Errorf("UpdatedAt = %v, want the latest observation %v", e.UpdatedAt, want)
		}
		if e.Magnitude != nil {
			t.Errorf("Magnitude = %v, want nil", *e.Magnitude)
		}
		if e.URL != "https://eonet.gsfc.nasa.gov/api/v3/events/EONET_10001" {
			t.Errorf("URL = %q", e.URL)
		}
	})

	t.Run("v1 carries no magnitude for EONET", func(t *testing.T) {
		e := eventByID(t, events, "eonet-EONET_10002")
		if e.EventType != "storm" {
			t.Errorf("EventType = %q, want storm", e.EventType)
		}
		// v1's magnitude is an earthquake magnitude; knots there would read
		// as a quake's size. v2 carries it with its unit.
		if e.Magnitude != nil {
			t.Errorf("Magnitude = %v, want none in v1", *e.Magnitude)
		}
	})

	t.Run("unmapped category becomes other", func(t *testing.T) {
		e := eventByID(t, events, "eonet-EONET_10003")
		if e.EventType != "other" {
			t.Errorf("EventType = %q, want other for dustHaze", e.EventType)
		}
		if e.Description != "A large dust plume moving west off the coast." {
			t.Errorf("Description = %q", e.Description)
		}
	})
}

// The window is every open event, old or new and in any category; the
// service cuts each request from it.
func TestEONETReturnsTheWholeWindow(t *testing.T) {
	t.Parallel()
	srv := serveFixture(t, "eonet.json", nil)
	a := newTestEONET(t, srv)

	events, err := a.FetchEvents(context.Background())
	if err != nil {
		t.Fatalf("FetchEvents: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("got %d events, want all 3; IDs: %v", len(events), eventIDs(events))
	}
	eventByID(t, events, "eonet-EONET_10003") // the oldest, and in an unmapped category
}

func TestEONETQuery(t *testing.T) {
	t.Parallel()
	var capture reqCapture
	srv := serveFixture(t, "eonet.json", &capture)
	a := newTestEONET(t, srv)

	if _, err := a.FetchEvents(context.Background()); err != nil {
		t.Fatalf("FetchEvents: %v", err)
	}

	q := capture.Query()
	if got := q.Get("status"); got != "open" {
		t.Errorf("status = %q, want open", got)
	}
	if len(q) != 1 {
		t.Errorf("query = %v, want only status=open: the window does not depend on the request", q)
	}
}

func TestEONETNon200(t *testing.T) {
	t.Parallel()
	srv := serveRaw(t, 503, "unavailable")
	a := newTestEONET(t, srv)

	_, err := a.FetchEvents(context.Background())
	if err == nil {
		t.Fatal("expected error for status 503")
	}
	if !strings.Contains(err.Error(), "unexpected status 503") {
		t.Errorf("error = %q, want mention of status 503", err)
	}
}

func TestEONETMalformedJSON(t *testing.T) {
	t.Parallel()
	srv := serveRaw(t, 200, `{"events": "oops"}`)
	a := newTestEONET(t, srv)

	_, err := a.FetchEvents(context.Background())
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
	if !strings.Contains(err.Error(), "decode response") {
		t.Errorf("error = %q, want decode error", err)
	}
}

// TestEONETV2FromTheRealEvents maps the recorded events onto v2: every event
// is valid, a storm's track is a line ending at its latest position, and
// each magnitude keeps its unit.
func TestEONETV2FromTheRealEvents(t *testing.T) {
	t.Parallel()
	a := newTestEONET(t, serveFixture(t, "live/eonet.json", nil))
	events, err := a.FetchV2(context.Background())
	if err != nil {
		t.Fatalf("FetchV2: %v", err)
	}
	units := map[string]bool{}
	var track bool
	for _, e := range events {
		if err := e.Validate(); err != nil {
			t.Errorf("%s: %v", e.ID, err)
		}
		if len(e.Details.EONET.Categories) == 0 || len(e.Details.EONET.Sources) == 0 {
			t.Errorf("%s: details %+v, want its categories and sources", e.ID, e.Details.EONET)
		}
		if e.Magnitude != nil {
			units[e.Magnitude.Unit] = true
		}
		if e.Location != nil && e.Location.Geometry != nil && e.Location.Geometry.Type == "LineString" {
			track = true
			var points [][2]float64
			if err := json.Unmarshal(e.Location.Geometry.Coordinates, &points); err != nil {
				t.Fatal(err)
			}
			if last := points[len(points)-1]; last != [2]float64(*e.Location.Center) {
				t.Errorf("%s: center %v, want the track's latest point %v", e.ID, *e.Location.Center, last)
			}
			if !e.Time.Started.Before(e.Time.Updated) {
				t.Errorf("%s: started %v, updated %v; a track starts before its latest point", e.ID, e.Time.Started, e.Time.Updated)
			}
		}
	}
	if !track {
		t.Error("no track: the recording holds a storm with 13 positions")
	}
	for _, u := range []string{"acres", "kts", "NM^2"} {
		if !units[u] {
			t.Errorf("no magnitude in %q; the recording holds one", u)
		}
	}
}
