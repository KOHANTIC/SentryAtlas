package adapters

import (
	"context"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/KOHANTIC/SentryAtlas/backend/internal/event"
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
	if got := a.SupportedTypes(); !slices.Equal(got, []string{"earthquake", "landslide", "other"}) {
		t.Errorf("SupportedTypes() = %v, want [earthquake landslide other]", got)
	}
}

func TestUSGSFetchEventsParsing(t *testing.T) {
	t.Parallel()
	srv := serveFixture(t, "usgs.json", nil)
	a := newTestUSGS(t, srv)

	events, err := a.FetchEvents(context.Background())
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
		// Two-element coordinates: the depth is unknown, not 0 km.
		if got, ok := e.Metadata["depth"]; ok {
			t.Errorf("Metadata[depth] = %v, want no depth when the feed gives none", got)
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

// The window is USGS's past-month feed, read as it is with no query.
func TestUSGSReadsTheMonthFeed(t *testing.T) {
	t.Parallel()
	if got := NewUSGSAdapter(nil).baseURL; !strings.HasSuffix(got, "/earthquakes/feed/v1.0/summary/all_month.geojson") {
		t.Errorf("baseURL = %q, want the past-month summary feed", got)
	}

	var capture reqCapture
	srv := serveFixture(t, "usgs.json", &capture)
	a := newTestUSGS(t, srv)
	if _, err := a.FetchEvents(context.Background()); err != nil {
		t.Fatalf("FetchEvents: %v", err)
	}
	if q := capture.Query(); len(q) != 0 {
		t.Errorf("query = %v, want none: the window does not depend on the request", q)
	}
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

// TestUSGSV2FromTheRealFeed maps the recorded feed onto v2: every event is
// valid, and each kind, status and flag lands where the model says.
func TestUSGSV2FromTheRealFeed(t *testing.T) {
	t.Parallel()
	srv := serveFixture(t, "live/usgs.json", nil)
	a := newTestUSGS(t, srv)

	events, err := a.FetchV2(context.Background())
	if err != nil {
		t.Fatalf("FetchV2: %v", err)
	}
	byKind := map[string]event.Event{}
	for _, e := range events {
		if err := e.Validate(); err != nil {
			t.Errorf("%s is not a valid v2 event: %v", e.ID, err)
		}
		if e.Time.Started.Location() != time.UTC {
			t.Errorf("%s started is not UTC", e.ID)
		}
		byKind[e.Details.USGS.Kind] = e
	}

	for kind, want := range map[string]string{
		"earthquake": "earthquake", "landslide": "landslide",
		"quarry blast": "other", "explosion": "other", "ice quake": "other", "other event": "other",
	} {
		e, ok := byKind[kind]
		if !ok {
			t.Errorf("no recorded %q", kind)
			continue
		}
		if e.Type != want {
			t.Errorf("a %q is type %q, want %q", kind, e.Type, want)
		}
	}

	var tsunami, alerted, automatic bool
	for _, e := range events {
		d := e.Details.USGS
		if d.Tsunami {
			tsunami = true
		}
		if d.Status == "automatic" {
			automatic = true
		}
		if e.Severity != nil {
			alerted = true
			if e.Severity.SourceLevel != d.Alert {
				t.Errorf("%s severity source_level %q, want the PAGER alert %q", e.ID, e.Severity.SourceLevel, d.Alert)
			}
		}
		if e.Magnitude != nil && e.Magnitude.Unit == "" {
			t.Errorf("%s magnitude has no unit", e.ID)
		}
	}
	if !tsunami || !alerted || !automatic {
		t.Errorf("tsunami %v, PAGER alert %v, automatic %v: want each in the recorded feed", tsunami, alerted, automatic)
	}
}

func TestUSGSNon200(t *testing.T) {
	t.Parallel()
	srv := serveRaw(t, 500, "internal error")
	a := newTestUSGS(t, srv)

	_, err := a.FetchEvents(context.Background())
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

	_, err := a.FetchEvents(context.Background())
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
	if !strings.Contains(err.Error(), "decode response") {
		t.Errorf("error = %q, want decode error", err)
	}
}
