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

func newTestGDACS(t *testing.T, srv *httptest.Server) *GDACSAdapter {
	t.Helper()
	a := NewGDACSAdapter(srv.Client())
	a.baseURL = srv.URL
	return a
}

func TestGDACSSourceAndSupportedTypes(t *testing.T) {
	t.Parallel()
	a := NewGDACSAdapter(nil)
	if got := a.Source(); got != "gdacs" {
		t.Errorf("Source() = %q, want %q", got, "gdacs")
	}
	want := []string{"earthquake", "cyclone", "flood", "volcano", "drought", "wildfire", "other"}
	if got := a.SupportedTypes(); !slices.Equal(got, want) {
		t.Errorf("SupportedTypes() = %v, want %v", got, want)
	}
}

func TestGDACSFetchEventsParsing(t *testing.T) {
	t.Parallel()
	srv := serveFixture(t, "gdacs.json", nil)
	a := newTestGDACS(t, srv)

	events, err := a.FetchEvents(context.Background(), FetchParams{})
	if err != nil {
		t.Fatalf("FetchEvents: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("got %d events, want 3; IDs: %v", len(events), eventIDs(events))
	}

	t.Run("earthquake with object url and numeric eventid", func(t *testing.T) {
		e := eventByID(t, events, "gdacs-EQ-1479624")
		if e.EventType != "earthquake" {
			t.Errorf("EventType = %q, want earthquake", e.EventType)
		}
		if e.Source != "gdacs" {
			t.Errorf("Source = %q, want gdacs", e.Source)
		}
		if e.Title != "Earthquake in Philippines" {
			t.Errorf("Title = %q", e.Title)
		}
		if e.Severity != "minor" {
			t.Errorf("Severity = %q, want minor (alertlevel Green)", e.Severity)
		}
		if !slices.Equal(e.Geometry.Coordinates, []float64{125.5, 8.9}) {
			t.Errorf("Coordinates = %v, want [125.5 8.9]", e.Geometry.Coordinates)
		}
		if want := time.Date(2026, 8, 9, 4, 30, 0, 0, time.UTC); !e.StartedAt.Equal(want) {
			t.Errorf("StartedAt = %v, want %v", e.StartedAt, want)
		}
		if want := time.Date(2026, 8, 9, 6, 30, 0, 0, time.UTC); !e.UpdatedAt.Equal(want) {
			t.Errorf("UpdatedAt = %v, want %v", e.UpdatedAt, want)
		}
		if e.URL != "https://www.gdacs.org/report.aspx?eventtype=EQ&eventid=1479624" {
			t.Errorf("URL = %q, want the report URL from the object form", e.URL)
		}
		// The physical severity value belongs in metadata, never in Magnitude:
		// GDACS severities are unit-bearing quantities, not Richter magnitudes.
		if e.Magnitude != nil {
			t.Errorf("Magnitude = %v, want nil", *e.Magnitude)
		}
		if got := e.Metadata["severity_value"]; got != 6.5 {
			t.Errorf("Metadata[severity_value] = %v, want 6.5", got)
		}
		if got := e.Metadata["severity_unit"]; got != "M" {
			t.Errorf("Metadata[severity_unit] = %v, want M", got)
		}
		if got := e.Metadata["alert_level"]; got != "Green" {
			t.Errorf("Metadata[alert_level] = %v, want Green", got)
		}
		if got := e.Metadata["country"]; got != "Philippines" {
			t.Errorf("Metadata[country] = %v, want Philippines", got)
		}
	})

	t.Run("cyclone with string url and string eventid", func(t *testing.T) {
		e := eventByID(t, events, "gdacs-TC-1000999")
		if e.EventType != "cyclone" {
			t.Errorf("EventType = %q, want cyclone", e.EventType)
		}
		if e.Severity != "extreme" {
			t.Errorf("Severity = %q, want extreme (alertlevel Red)", e.Severity)
		}
		if e.URL != "https://www.gdacs.org/report.aspx?eventtype=TC&eventid=1000999" {
			t.Errorf("URL = %q, want the string-form url", e.URL)
		}
		// RFC3339 fromdate (with Z) must parse via the fallback layout.
		if want := time.Date(2026, 8, 8, 0, 0, 0, 0, time.UTC); !e.StartedAt.Equal(want) {
			t.Errorf("StartedAt = %v, want %v", e.StartedAt, want)
		}
		if got := e.Metadata["severity_value"]; got != 213.0 {
			t.Errorf("Metadata[severity_value] = %v, want 213", got)
		}
	})

	t.Run("unknown type with empty fields", func(t *testing.T) {
		e := eventByID(t, events, "gdacs-XX-555")
		if e.EventType != "other" {
			t.Errorf("EventType = %q, want other for unmapped code XX", e.EventType)
		}
		if e.Severity != "" {
			t.Errorf("Severity = %q, want empty for empty alertlevel", e.Severity)
		}
		if len(e.Geometry.Coordinates) != 0 {
			t.Errorf("Coordinates = %v, want none for null geometry", e.Geometry.Coordinates)
		}
		if !e.StartedAt.IsZero() {
			t.Errorf("StartedAt = %v, want zero for empty fromdate", e.StartedAt)
		}
		// severity 0 means unset: no severity_value key at all.
		if _, ok := e.Metadata["severity_value"]; ok {
			t.Error("Metadata[severity_value] present, want absent for severity 0")
		}
	})
}

func TestGDACSNoContent(t *testing.T) {
	t.Parallel()
	srv := serveRaw(t, 204, "")
	a := newTestGDACS(t, srv)

	events, err := a.FetchEvents(context.Background(), FetchParams{})
	if err != nil {
		t.Fatalf("FetchEvents on 204: %v, want nil error", err)
	}
	if events != nil {
		t.Errorf("events = %v, want nil on 204 No Content", events)
	}
}

func TestGDACSQueryParams(t *testing.T) {
	t.Parallel()
	var capture reqCapture
	srv := serveFixture(t, "gdacs.json", &capture)
	a := newTestGDACS(t, srv)

	since := time.Date(2026, 8, 1, 15, 30, 0, 0, time.UTC)
	// "storm" has no GDACS code and must be dropped from the eventlist.
	_, err := a.FetchEvents(context.Background(), FetchParams{
		Types: []string{"earthquake", "storm", "cyclone"},
		Since: since,
	})
	if err != nil {
		t.Fatalf("FetchEvents: %v", err)
	}

	q := capture.Query()
	if got := q.Get("eventlist"); got != "EQ;TC" {
		t.Errorf("eventlist = %q, want EQ;TC", got)
	}
	if got := q.Get("fromdate"); got != "2026-08-01" {
		t.Errorf("fromdate = %q, want 2026-08-01", got)
	}
	if got := q.Get("todate"); got == "" {
		t.Error("todate not set")
	}
}

func TestGDACSAlertToSeverity(t *testing.T) {
	t.Parallel()
	cases := []struct {
		level, want string
	}{
		{"red", "extreme"},
		{"Red", "extreme"},
		{"ORANGE", "severe"},
		{"orange", "severe"},
		{"green", "minor"},
		{"Green", "minor"},
		{"", ""},
		{"blue", ""},
	}
	for _, tc := range cases {
		if got := gdacsAlertToSeverity(tc.level); got != tc.want {
			t.Errorf("gdacsAlertToSeverity(%q) = %q, want %q", tc.level, got, tc.want)
		}
	}
}

func TestGDACSURLUnmarshal(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"string form", `"https://example.org/report"`, "https://example.org/report", false},
		{"object form", `{"report": "https://example.org/r2", "details": "https://example.org/d"}`, "https://example.org/r2", false},
		{"object without report", `{"details": "https://example.org/d"}`, "", false},
		{"invalid", `42`, "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var u gdacsURL
			err := json.Unmarshal([]byte(tc.input), &u)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if u.Report != tc.want {
				t.Errorf("Report = %q, want %q", u.Report, tc.want)
			}
		})
	}
}

func TestGDACSNon200(t *testing.T) {
	t.Parallel()
	srv := serveRaw(t, 502, "bad gateway")
	a := newTestGDACS(t, srv)

	_, err := a.FetchEvents(context.Background(), FetchParams{})
	if err == nil {
		t.Fatal("expected error for status 502")
	}
	if !strings.Contains(err.Error(), "unexpected status 502") {
		t.Errorf("error = %q, want mention of status 502", err)
	}
}

func TestGDACSMalformedJSON(t *testing.T) {
	t.Parallel()
	srv := serveRaw(t, 200, `{"features": [{"properties": {"eventid": true}}]}`)
	a := newTestGDACS(t, srv)

	_, err := a.FetchEvents(context.Background(), FetchParams{})
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
	if !strings.Contains(err.Error(), "decode response") {
		t.Errorf("error = %q, want decode error", err)
	}
}
