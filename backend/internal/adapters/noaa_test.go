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

const testUserAgent = "SentryAtlasTest/0.0 (test@example.com)"

func newTestNOAA(t *testing.T, srv *httptest.Server) *NOAAAdapter {
	t.Helper()
	a := NewNOAAAdapter(srv.Client(), testUserAgent)
	a.baseURL = srv.URL
	return a
}

func TestNOAASourceAndSupportedTypes(t *testing.T) {
	t.Parallel()
	a := NewNOAAAdapter(nil, testUserAgent)
	if got := a.Source(); got != "noaa" {
		t.Errorf("Source() = %q, want %q", got, "noaa")
	}
	want := []string{"flood", "storm", "tornado", "hurricane", "winter_storm", "tsunami", "wildfire", "earthquake", "volcano", "weather"}
	if got := a.SupportedTypes(); !slices.Equal(got, want) {
		t.Errorf("SupportedTypes() = %v, want %v", got, want)
	}
}

func TestNOAAFetchEventsParsing(t *testing.T) {
	t.Parallel()
	srv := serveFixture(t, "noaa.json", nil)
	a := newTestNOAA(t, srv)

	events, err := a.FetchEvents(context.Background(), FetchParams{})
	if err != nil {
		t.Fatalf("FetchEvents: %v", err)
	}
	if len(events) != 4 {
		t.Fatalf("got %d events, want 4; IDs: %v", len(events), eventIDs(events))
	}

	t.Run("tornado with polygon centroid", func(t *testing.T) {
		e := eventByID(t, events, "noaa-urn:oid:2.49.0.1.840.0.tornado1")
		if e.EventType != "tornado" {
			t.Errorf("EventType = %q, want tornado", e.EventType)
		}
		if e.Severity != "extreme" {
			t.Errorf("Severity = %q, want extreme", e.Severity)
		}
		// Vertex average of the closed ring in the fixture.
		if !slices.Equal(e.Geometry.Coordinates, []float64{-96.6, 32.4}) {
			t.Errorf("Coordinates = %v, want centroid [-96.6 32.4]", e.Geometry.Coordinates)
		}
		if want := time.Date(2026, 8, 10, 14, 0, 0, 0, time.UTC); !e.StartedAt.Equal(want) {
			t.Errorf("StartedAt = %v, want onset %v", e.StartedAt, want)
		}
		if want := time.Date(2026, 8, 10, 14, 5, 0, 0, time.UTC); !e.UpdatedAt.Equal(want) {
			t.Errorf("UpdatedAt = %v, want sent %v", e.UpdatedAt, want)
		}
		if e.Title != "Tornado Warning issued August 10 at 2:00PM CDT by NWS Fort Worth TX" {
			t.Errorf("Title = %q", e.Title)
		}
		if got := e.Metadata["event"]; got != "Tornado Warning" {
			t.Errorf("Metadata[event] = %v", got)
		}
		if got := e.Metadata["area_desc"]; got != "Dallas County, TX" {
			t.Errorf("Metadata[area_desc] = %v", got)
		}
		if got := e.Metadata["urgency"]; got != "Immediate" {
			t.Errorf("Metadata[urgency] = %v", got)
		}
		if got := e.Metadata["certainty"]; got != "Observed" {
			t.Errorf("Metadata[certainty] = %v", got)
		}
		if got := e.Metadata["sender_name"]; got != "NWS Fort Worth TX" {
			t.Errorf("Metadata[sender_name] = %v", got)
		}
	})

	t.Run("null geometry yields no coordinates", func(t *testing.T) {
		e := eventByID(t, events, "noaa-urn:oid:2.49.0.1.840.0.flood1")
		if e.EventType != "flood" {
			t.Errorf("EventType = %q, want flood", e.EventType)
		}
		if e.Severity != "severe" {
			t.Errorf("Severity = %q, want severe", e.Severity)
		}
		if len(e.Geometry.Coordinates) != 0 {
			t.Errorf("Coordinates = %v, want none for null geometry", e.Geometry.Coordinates)
		}
	})

	t.Run("multipolygon centroid and weather fallback", func(t *testing.T) {
		e := eventByID(t, events, "noaa-urn:oid:2.49.0.1.840.0.weather1")
		if e.EventType != "weather" {
			t.Errorf("EventType = %q, want weather fallback", e.EventType)
		}
		if e.Severity != "moderate" {
			t.Errorf("Severity = %q, want moderate", e.Severity)
		}
		if !slices.Equal(e.Geometry.Coordinates, []float64{-119.6, 45.4}) {
			t.Errorf("Coordinates = %v, want centroid [-119.6 45.4]", e.Geometry.Coordinates)
		}
	})

	t.Run("unknown severity and empty onset", func(t *testing.T) {
		e := eventByID(t, events, "noaa-urn:oid:2.49.0.1.840.0.winter1")
		if e.EventType != "winter_storm" {
			t.Errorf("EventType = %q, want winter_storm", e.EventType)
		}
		if e.Severity != "" {
			t.Errorf("Severity = %q, want empty for Unknown", e.Severity)
		}
		// Onset is empty: StartedAt falls back to the sent time.
		want := time.Date(2026, 8, 7, 9, 0, 0, 0, time.UTC)
		if !e.StartedAt.Equal(want) {
			t.Errorf("StartedAt = %v, want sent time %v", e.StartedAt, want)
		}
	})
}

func TestNOAAUserAgentAndQuery(t *testing.T) {
	t.Parallel()
	var capture reqCapture
	srv := serveFixture(t, "noaa.json", &capture)
	a := newTestNOAA(t, srv)

	if _, err := a.FetchEvents(context.Background(), FetchParams{}); err != nil {
		t.Fatalf("FetchEvents: %v", err)
	}

	if got := capture.Header().Get("User-Agent"); got != testUserAgent {
		t.Errorf("User-Agent = %q, want %q", got, testUserAgent)
	}
	if got := capture.Header().Get("Accept"); got != "application/geo+json" {
		t.Errorf("Accept = %q, want application/geo+json", got)
	}
	q := capture.Query()
	if got := q.Get("status"); got != "actual" {
		t.Errorf("status = %q, want actual", got)
	}
	if got := q.Get("message_type"); got != "alert" {
		t.Errorf("message_type = %q, want alert", got)
	}
}

func TestNOAARequestedTypesFilter(t *testing.T) {
	t.Parallel()
	srv := serveFixture(t, "noaa.json", nil)
	a := newTestNOAA(t, srv)

	events, err := a.FetchEvents(context.Background(), FetchParams{Types: []string{"flood"}})
	if err != nil {
		t.Fatalf("FetchEvents: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1; IDs: %v", len(events), eventIDs(events))
	}
	if events[0].ID != "noaa-urn:oid:2.49.0.1.840.0.flood1" {
		t.Errorf("got %q, want the flood alert", events[0].ID)
	}
}

func TestNOAASinceFilter(t *testing.T) {
	t.Parallel()
	srv := serveFixture(t, "noaa.json", nil)
	a := newTestNOAA(t, srv)

	since := time.Date(2026, 8, 9, 0, 0, 0, 0, time.UTC)
	events, err := a.FetchEvents(context.Background(), FetchParams{Since: since})
	if err != nil {
		t.Fatalf("FetchEvents: %v", err)
	}
	// Only the tornado (08-10) and flood (08-09) alerts start on or after since.
	if len(events) != 2 {
		t.Fatalf("got %d events, want 2; IDs: %v", len(events), eventIDs(events))
	}
	eventByID(t, events, "noaa-urn:oid:2.49.0.1.840.0.tornado1")
	eventByID(t, events, "noaa-urn:oid:2.49.0.1.840.0.flood1")
}

func TestNOAACentroid(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		typ    string
		coords string
		want   []float64
	}{
		{"point", "Point", `[-100.5, 35.25]`, []float64{-100.5, 35.25}},
		{"point with elevation", "Point", `[-100.5, 35.25, 12.0]`, []float64{-100.5, 35.25}},
		{"short point", "Point", `[-100.5]`, nil},
		{"malformed point", "Point", `"oops"`, nil},
		{"polygon", "Polygon", `[[[0,0],[2,0],[2,2],[0,2]]]`, []float64{1, 1}},
		{"empty polygon", "Polygon", `[]`, nil},
		{"multipolygon uses first ring of first polygon", "MultiPolygon",
			`[[[[10,10],[14,10],[14,14],[10,14]]],[[[50,50],[52,50],[52,52]]]]`, []float64{12, 12}},
		{"unknown type", "LineString", `[[0,0],[1,1]]`, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := &noaaGeometry{Type: tc.typ, Coordinates: json.RawMessage(tc.coords)}
			got := g.centroid()
			if !slices.Equal(got, tc.want) {
				t.Errorf("centroid() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestClassifyNOAAEvent(t *testing.T) {
	t.Parallel()
	cases := []struct {
		event, want string
	}{
		{"Tornado Warning", "tornado"},
		{"Flash Flood Warning", "flood"},
		{"Flood Advisory", "flood"},
		{"Hurricane Warning", "hurricane"},
		{"Typhoon Local Statement", "hurricane"},
		{"Tropical Storm Warning", "storm"},
		{"Severe Thunderstorm Warning", "storm"},
		{"Winter Storm Watch", "winter_storm"},
		{"Blizzard Warning", "winter_storm"},
		{"Ice Storm Warning", "winter_storm"},
		{"Tsunami Advisory", "tsunami"},
		{"Earthquake Warning", "earthquake"},
		{"Volcano Warning", "volcano"},
		{"Red Flag Warning", "wildfire"},
		{"Fire Weather Watch", "wildfire"},
		{"Special Weather Statement", "weather"},
		{"Air Quality Alert", "weather"},
		{"", "weather"},
	}
	for _, tc := range cases {
		if got := classifyNOAAEvent(tc.event); got != tc.want {
			t.Errorf("classifyNOAAEvent(%q) = %q, want %q", tc.event, got, tc.want)
		}
	}
}

func TestNOAANon200(t *testing.T) {
	t.Parallel()
	srv := serveRaw(t, 429, "too many requests")
	a := newTestNOAA(t, srv)

	_, err := a.FetchEvents(context.Background(), FetchParams{})
	if err == nil {
		t.Fatal("expected error for status 429")
	}
	if !strings.Contains(err.Error(), "unexpected status 429") {
		t.Errorf("error = %q, want mention of status 429", err)
	}
}

func TestNOAAMalformedJSON(t *testing.T) {
	t.Parallel()
	srv := serveRaw(t, 200, `<html>gateway error</html>`)
	a := newTestNOAA(t, srv)

	_, err := a.FetchEvents(context.Background(), FetchParams{})
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
	if !strings.Contains(err.Error(), "decode response") {
		t.Errorf("error = %q, want decode error", err)
	}
}
