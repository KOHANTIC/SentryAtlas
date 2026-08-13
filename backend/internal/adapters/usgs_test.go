package adapters

import (
	"context"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"
)

func newTestUSGS(t *testing.T, srv *httptest.Server) *USGSAdapter {
	t.Helper()
	a := NewUSGSAdapter(srv.Client())
	a.baseURL = srv.URL
	return a
}

func TestUSGSSourceAndSupportedTypes(t *testing.T) {
	t.Parallel()
	a := NewUSGSAdapter(nil)
	if got := a.Source(); got != "usgs" {
		t.Errorf("Source() = %q, want %q", got, "usgs")
	}
	if got := a.SupportedTypes(); !slices.Equal(got, []string{"earthquake"}) {
		t.Errorf("SupportedTypes() = %v, want [earthquake]", got)
	}
}

func TestUSGSFetchEventsParsing(t *testing.T) {
	t.Parallel()
	srv := serveFixture(t, "usgs.json", nil)
	a := newTestUSGS(t, srv)

	events, err := a.FetchEvents(context.Background(), FetchParams{})
	if err != nil {
		t.Fatalf("FetchEvents: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("got %d events, want 3", len(events))
	}

	t.Run("happy path with alert and depth", func(t *testing.T) {
		e := eventByID(t, events, "usgs-us7000red1")
		if e.Title != "M 6.2 - 35 km W of Anchor Point, Alaska" {
			t.Errorf("Title = %q", e.Title)
		}
		if e.EventType != "earthquake" {
			t.Errorf("EventType = %q, want earthquake", e.EventType)
		}
		if e.Source != "usgs" {
			t.Errorf("Source = %q, want usgs", e.Source)
		}
		if !slices.Equal(e.Geometry.Coordinates, []float64{-151.9234, 59.7568}) {
			t.Errorf("Coordinates = %v, want [-151.9234 59.7568]", e.Geometry.Coordinates)
		}
		if e.Geometry.Type != "Point" {
			t.Errorf("Geometry.Type = %q, want Point", e.Geometry.Type)
		}
		if e.Magnitude == nil || *e.Magnitude != 6.2 {
			t.Errorf("Magnitude = %v, want 6.2", e.Magnitude)
		}
		if e.Severity != "extreme" {
			t.Errorf("Severity = %q, want extreme (alert red)", e.Severity)
		}
		if want := time.UnixMilli(1786320000000); !e.StartedAt.Equal(want) {
			t.Errorf("StartedAt = %v, want %v", e.StartedAt, want)
		}
		if want := time.UnixMilli(1786323600000); !e.UpdatedAt.Equal(want) {
			t.Errorf("UpdatedAt = %v, want %v", e.UpdatedAt, want)
		}
		if e.URL != "https://earthquake.usgs.gov/earthquakes/eventpage/us7000red1" {
			t.Errorf("URL = %q", e.URL)
		}
		if got := e.Metadata["place"]; got != "35 km W of Anchor Point, Alaska" {
			t.Errorf("Metadata[place] = %v", got)
		}
		if got := e.Metadata["depth"]; got != 42.5 {
			t.Errorf("Metadata[depth] = %v, want 42.5 (third coordinate)", got)
		}
	})

	t.Run("null magnitude stays nil", func(t *testing.T) {
		e := eventByID(t, events, "usgs-nc75001234")
		if e.Magnitude != nil {
			t.Errorf("Magnitude = %v, want nil for JSON null", *e.Magnitude)
		}
		if e.Severity != "" {
			t.Errorf("Severity = %q, want empty for null alert", e.Severity)
		}
		// Two-element coordinates: no depth available.
		if got := e.Metadata["depth"]; got != 0.0 {
			t.Errorf("Metadata[depth] = %v, want 0 when absent", got)
		}
	})

	t.Run("zero magnitude is kept, not dropped", func(t *testing.T) {
		e := eventByID(t, events, "usgs-ak0269zero")
		if e.Magnitude == nil {
			t.Fatal("Magnitude = nil, want non-nil pointer to 0.0")
		}
		if *e.Magnitude != 0.0 {
			t.Errorf("Magnitude = %v, want 0.0", *e.Magnitude)
		}
		if e.Severity != "moderate" {
			t.Errorf("Severity = %q, want moderate (alert yellow)", e.Severity)
		}
	})
}

func TestUSGSQueryParams(t *testing.T) {
	t.Parallel()

	t.Run("default starttime is set", func(t *testing.T) {
		var capture reqCapture
		srv := serveFixture(t, "usgs.json", &capture)
		a := newTestUSGS(t, srv)

		if _, err := a.FetchEvents(context.Background(), FetchParams{}); err != nil {
			t.Fatalf("FetchEvents: %v", err)
		}
		q := capture.Query()
		if got := q.Get("format"); got != "geojson" {
			t.Errorf("format = %q, want geojson", got)
		}
		start := q.Get("starttime")
		if start == "" {
			t.Fatal("starttime not set")
		}
		if _, err := time.Parse(time.RFC3339, start); err != nil {
			t.Errorf("starttime %q is not RFC3339: %v", start, err)
		}
	})

	t.Run("since becomes starttime", func(t *testing.T) {
		var capture reqCapture
		srv := serveFixture(t, "usgs.json", &capture)
		a := newTestUSGS(t, srv)

		since := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
		if _, err := a.FetchEvents(context.Background(), FetchParams{Since: since}); err != nil {
			t.Fatalf("FetchEvents: %v", err)
		}
		if got, want := capture.Query().Get("starttime"), since.Format(time.RFC3339); got != want {
			t.Errorf("starttime = %q, want %q", got, want)
		}
	})
}

func TestUSGSAlertToSeverity(t *testing.T) {
	t.Parallel()
	cases := []struct {
		alert, want string
	}{
		{"red", "extreme"},
		{"orange", "severe"},
		{"yellow", "moderate"},
		{"green", "minor"},
		{"", ""},
		{"purple", ""},
	}
	for _, tc := range cases {
		if got := usgsAlertToSeverity(tc.alert); got != tc.want {
			t.Errorf("usgsAlertToSeverity(%q) = %q, want %q", tc.alert, got, tc.want)
		}
	}
}

func TestUSGSDepthFromCoords(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		coords []float64
		want   float64
	}{
		{"three coords", []float64{1, 2, 42.5}, 42.5},
		{"two coords", []float64{1, 2}, 0},
		{"nil", nil, 0},
	}
	for _, tc := range cases {
		if got := depthFromCoords(tc.coords); got != tc.want {
			t.Errorf("%s: depthFromCoords(%v) = %v, want %v", tc.name, tc.coords, got, tc.want)
		}
	}
}

func TestUSGSNon200(t *testing.T) {
	t.Parallel()
	srv := serveRaw(t, 500, "internal error")
	a := newTestUSGS(t, srv)

	_, err := a.FetchEvents(context.Background(), FetchParams{})
	if err == nil {
		t.Fatal("expected error for status 500")
	}
	if !strings.Contains(err.Error(), "unexpected status 500") {
		t.Errorf("error = %q, want mention of status 500", err)
	}
}

func TestUSGSMalformedJSON(t *testing.T) {
	t.Parallel()
	srv := serveRaw(t, 200, "{not json")
	a := newTestUSGS(t, srv)

	_, err := a.FetchEvents(context.Background(), FetchParams{})
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
	if !strings.Contains(err.Error(), "decode response") {
		t.Errorf("error = %q, want decode error", err)
	}
}
