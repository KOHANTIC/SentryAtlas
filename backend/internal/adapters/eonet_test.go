package adapters

import (
	"context"
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

	events, err := a.FetchEvents(context.Background(), FetchParams{})
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
		want := time.Date(2026, 8, 6, 10, 0, 0, 0, time.UTC)
		if !e.StartedAt.Equal(want) {
			t.Errorf("StartedAt = %v, want last geometry date %v", e.StartedAt, want)
		}
		if !e.UpdatedAt.Equal(e.StartedAt) {
			t.Errorf("UpdatedAt = %v, want same as StartedAt", e.UpdatedAt)
		}
		if e.Magnitude != nil {
			t.Errorf("Magnitude = %v, want nil", *e.Magnitude)
		}
		if e.URL != "https://eonet.gsfc.nasa.gov/api/v3/events/EONET_10001" {
			t.Errorf("URL = %q", e.URL)
		}
	})

	t.Run("magnitude from event-level magnitudeValue", func(t *testing.T) {
		e := eventByID(t, events, "eonet-EONET_10002")
		if e.EventType != "storm" {
			t.Errorf("EventType = %q, want storm", e.EventType)
		}
		if e.Magnitude == nil || *e.Magnitude != 65.0 {
			t.Errorf("Magnitude = %v, want 65.0", e.Magnitude)
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

func TestEONETSinceFiltering(t *testing.T) {
	t.Parallel()
	srv := serveFixture(t, "eonet.json", nil)
	a := newTestEONET(t, srv)

	since := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	events, err := a.FetchEvents(context.Background(), FetchParams{Since: since})
	if err != nil {
		t.Fatalf("FetchEvents: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("got %d events, want 2 (the July event filtered out); IDs: %v", len(events), eventIDs(events))
	}
	for _, e := range events {
		if e.ID == "eonet-EONET_10003" {
			t.Error("EONET_10003 (2026-07-01) should have been filtered by since=2026-07-15")
		}
	}
}

func TestEONETCategoryQuery(t *testing.T) {
	t.Parallel()
	var capture reqCapture
	srv := serveFixture(t, "eonet.json", &capture)
	a := newTestEONET(t, srv)

	// "weather" has no EONET category and must not be forwarded.
	_, err := a.FetchEvents(context.Background(), FetchParams{Types: []string{"wildfire", "weather", "storm"}})
	if err != nil {
		t.Fatalf("FetchEvents: %v", err)
	}

	q := capture.Query()
	if got := q.Get("status"); got != "open" {
		t.Errorf("status = %q, want open", got)
	}
	cats := q["category"]
	if !slices.Equal(cats, []string{"wildfires", "severeStorms"}) {
		t.Errorf("category = %v, want [wildfires severeStorms]", cats)
	}
}

func TestEONETNon200(t *testing.T) {
	t.Parallel()
	srv := serveRaw(t, 503, "unavailable")
	a := newTestEONET(t, srv)

	_, err := a.FetchEvents(context.Background(), FetchParams{})
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

	_, err := a.FetchEvents(context.Background(), FetchParams{})
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
	if !strings.Contains(err.Error(), "decode response") {
		t.Errorf("error = %q, want decode error", err)
	}
}
