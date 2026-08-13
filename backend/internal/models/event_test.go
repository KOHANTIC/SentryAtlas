package models

import (
	"slices"
	"strings"
	"testing"
	"time"
)

var testTime = time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)

func fullEvent() Event {
	mag := 6.2
	return Event{
		ID:          "usgs-abc",
		Title:       "M 6.2 - Somewhere",
		Description: "A strong quake.",
		EventType:   "earthquake",
		Source:      "usgs",
		Geometry:    Geometry{Type: "Point", Coordinates: []float64{-151.9, 59.7}},
		Magnitude:   &mag,
		Severity:    "severe",
		StartedAt:   testTime,
		UpdatedAt:   testTime.Add(time.Hour),
		URL:         "https://example.org/abc",
		Metadata:    map[string]any{"place": "Somewhere"},
	}
}

func TestToGeoJSONFeatureFull(t *testing.T) {
	t.Parallel()
	f := fullEvent().ToGeoJSONFeature()

	if f.Type != "Feature" {
		t.Errorf("Type = %q, want Feature", f.Type)
	}
	if f.Geometry == nil {
		t.Fatal("Geometry = nil, want non-nil for located event")
	}
	if f.Geometry.Type != "Point" {
		t.Errorf("Geometry.Type = %q, want Point", f.Geometry.Type)
	}
	if !slices.Equal(f.Geometry.Coordinates, []float64{-151.9, 59.7}) {
		t.Errorf("Geometry.Coordinates = %v", f.Geometry.Coordinates)
	}

	want := map[string]any{
		"id":          "usgs-abc",
		"title":       "M 6.2 - Somewhere",
		"event_type":  "earthquake",
		"source":      "usgs",
		"started_at":  "2026-08-10T12:00:00Z",
		"updated_at":  "2026-08-10T13:00:00Z",
		"description": "A strong quake.",
		"magnitude":   6.2,
		"severity":    "severe",
		"url":         "https://example.org/abc",
	}
	for k, v := range want {
		if got := f.Properties[k]; got != v {
			t.Errorf("Properties[%q] = %v, want %v", k, got, v)
		}
	}
	meta, ok := f.Properties["metadata"].(map[string]any)
	if !ok || meta["place"] != "Somewhere" {
		t.Errorf("Properties[metadata] = %v, want map with place", f.Properties["metadata"])
	}
}

func TestToGeoJSONFeatureOmissions(t *testing.T) {
	t.Parallel()
	e := Event{
		ID:        "x-1",
		Title:     "Bare event",
		EventType: "other",
		Source:    "x",
		StartedAt: testTime,
		UpdatedAt: testTime,
	}
	f := e.ToGeoJSONFeature()

	if f.Geometry != nil {
		t.Errorf("Geometry = %v, want nil for event without coordinates", f.Geometry)
	}
	for _, k := range []string{"description", "magnitude", "severity", "url", "metadata"} {
		if _, present := f.Properties[k]; present {
			t.Errorf("Properties[%q] present, want omitted for empty value", k)
		}
	}
	for _, k := range []string{"id", "title", "event_type", "source", "started_at", "updated_at"} {
		if _, present := f.Properties[k]; !present {
			t.Errorf("Properties[%q] missing, want always present", k)
		}
	}
}

func TestToGeoJSONFeatureGeometryNullWithOneCoordinate(t *testing.T) {
	t.Parallel()
	e := fullEvent()
	e.Geometry.Coordinates = []float64{-151.9}
	if f := e.ToGeoJSONFeature(); f.Geometry != nil {
		t.Errorf("Geometry = %v, want nil for a single coordinate", f.Geometry)
	}
}

func TestToGeoJSONFeatureZeroMagnitudeKept(t *testing.T) {
	t.Parallel()
	e := fullEvent()
	zero := 0.0
	e.Magnitude = &zero
	f := e.ToGeoJSONFeature()
	got, present := f.Properties["magnitude"]
	if !present {
		t.Fatal("Properties[magnitude] missing, want present for explicit 0.0")
	}
	if got != 0.0 {
		t.Errorf("Properties[magnitude] = %v, want 0.0", got)
	}
}

func TestToFlatEvent(t *testing.T) {
	t.Parallel()

	t.Run("located", func(t *testing.T) {
		fe := fullEvent().ToFlatEvent()
		if fe.Coordinates == nil {
			t.Fatal("Coordinates = nil, want set")
		}
		if fe.Coordinates.Longitude != -151.9 || fe.Coordinates.Latitude != 59.7 {
			t.Errorf("Coordinates = %+v, want lon -151.9 lat 59.7", fe.Coordinates)
		}
		if fe.ID != "usgs-abc" || fe.EventType != "earthquake" || fe.Source != "usgs" {
			t.Errorf("fields not carried over: %+v", fe)
		}
		if fe.Magnitude == nil || *fe.Magnitude != 6.2 {
			t.Errorf("Magnitude = %v, want 6.2", fe.Magnitude)
		}
	})

	t.Run("unlocated never becomes null island", func(t *testing.T) {
		e := fullEvent()
		e.Geometry.Coordinates = nil
		if fe := e.ToFlatEvent(); fe.Coordinates != nil {
			t.Errorf("Coordinates = %+v, want nil for unlocated event", fe.Coordinates)
		}
	})

	t.Run("single coordinate treated as unlocated", func(t *testing.T) {
		e := fullEvent()
		e.Geometry.Coordinates = []float64{5}
		if fe := e.ToFlatEvent(); fe.Coordinates != nil {
			t.Errorf("Coordinates = %+v, want nil for one coordinate", fe.Coordinates)
		}
	})
}

func TestEventsToFeatureCollection(t *testing.T) {
	t.Parallel()
	statuses := []SourceStatus{
		{Source: "usgs", OK: true},
		{Source: "noaa", Error: "boom"},
	}
	fc := EventsToFeatureCollection([]Event{fullEvent(), fullEvent()}, statuses)

	if fc.Type != "FeatureCollection" {
		t.Errorf("Type = %q, want FeatureCollection", fc.Type)
	}
	if len(fc.Features) != 2 {
		t.Errorf("len(Features) = %d, want 2", len(fc.Features))
	}
	if len(fc.Sources) != 2 || fc.Sources[0].Source != "usgs" || fc.Sources[1].Error != "boom" {
		t.Errorf("Sources = %+v, want passthrough", fc.Sources)
	}
}

func TestEventsToJSON(t *testing.T) {
	t.Parallel()
	statuses := []SourceStatus{{Source: "usgs", OK: true}}
	resp := EventsToJSON([]Event{fullEvent()}, statuses)

	if resp.Total != 1 {
		t.Errorf("Total = %d, want 1", resp.Total)
	}
	if len(resp.Events) != 1 {
		t.Errorf("len(Events) = %d, want 1", len(resp.Events))
	}
	if len(resp.Sources) != 1 || resp.Sources[0].Source != "usgs" {
		t.Errorf("Sources = %+v", resp.Sources)
	}
}

func TestMarshalGeoJSONWireFormat(t *testing.T) {
	t.Parallel()

	t.Run("unlocated event marshals geometry null", func(t *testing.T) {
		e := fullEvent()
		e.Geometry.Coordinates = nil
		data, err := MarshalGeoJSON([]Event{e}, nil)
		if err != nil {
			t.Fatalf("MarshalGeoJSON: %v", err)
		}
		if !strings.Contains(string(data), `"geometry":null`) {
			t.Errorf("output lacks \"geometry\":null: %s", data)
		}
	})

	t.Run("empty events marshal as empty array", func(t *testing.T) {
		data, err := MarshalGeoJSON(nil, nil)
		if err != nil {
			t.Fatalf("MarshalGeoJSON: %v", err)
		}
		if !strings.Contains(string(data), `"features":[]`) {
			t.Errorf("output lacks \"features\":[]: %s", data)
		}
		if strings.Contains(string(data), `"sources"`) {
			t.Errorf("nil sources should be omitted: %s", data)
		}
	})
}

func TestMarshalEventsJSONWireFormat(t *testing.T) {
	t.Parallel()
	e := fullEvent()
	e.Geometry.Coordinates = nil
	data, err := MarshalEventsJSON([]Event{e}, nil)
	if err != nil {
		t.Fatalf("MarshalEventsJSON: %v", err)
	}
	if !strings.Contains(string(data), `"coordinates":null`) {
		t.Errorf("output lacks \"coordinates\":null for unlocated event: %s", data)
	}
	if !strings.Contains(string(data), `"total":1`) {
		t.Errorf("output lacks \"total\":1: %s", data)
	}
}
