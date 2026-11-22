package event

import (
	"encoding/json"
	"testing"
	"time"
)

func TestV1ProjectsAUSGSEvent(t *testing.T) {
	t.Parallel()
	started := time.Date(2026, 11, 21, 4, 29, 14, 0, time.UTC)
	center := Point{-147.79, 60.301}
	depth := 3.6
	e := Event{
		ID: "usgs-ak1", Source: SourceUSGS, Type: "earthquake", Title: "M 4.1",
		Magnitude: &Magnitude{Value: 4.1, Unit: "ml"},
		Severity:  &Severity{Level: SeverityMinor, SourceLevel: "green"},
		Time:      Time{Started: started, Updated: started.Add(time.Hour)},
		Location:  &Location{Center: &center, Geometry: &Geometry{Type: "Point", Coordinates: json.RawMessage(`[-147.79,60.301]`)}},
		URL:       "https://example.test/ak1",
		Details:   Details{USGS: &USGS{Kind: "earthquake", Place: "28 km NNE of Chenega, Alaska", DepthKm: &depth}},
	}

	v := e.V1()
	if v.ID != e.ID || v.EventType != "earthquake" || v.Source != "usgs" || v.Title != "M 4.1" || v.URL != e.URL {
		t.Errorf("identity fields = %+v", v)
	}
	if v.Magnitude == nil || *v.Magnitude != 4.1 {
		t.Errorf("Magnitude = %v, want 4.1 without its unit", v.Magnitude)
	}
	if v.Severity != SeverityMinor {
		t.Errorf("Severity = %q, want the level", v.Severity)
	}
	if !v.StartedAt.Equal(started) || !v.UpdatedAt.Equal(started.Add(time.Hour)) {
		t.Errorf("times = %v, %v", v.StartedAt, v.UpdatedAt)
	}
	if v.Geometry.Type != "Point" || len(v.Geometry.Coordinates) != 2 || v.Geometry.Coordinates[0] != -147.79 || v.Geometry.Coordinates[1] != 60.301 {
		t.Errorf("Geometry = %+v, want the center as a point", v.Geometry)
	}
	if v.Metadata["place"] != "28 km NNE of Chenega, Alaska" || v.Metadata["depth"] != 3.6 {
		t.Errorf("Metadata = %v, want v1's place and depth", v.Metadata)
	}

	e.Details.USGS.DepthKm = nil
	if _, ok := e.V1().Metadata["depth"]; ok {
		t.Error("an unknown depth reached v1 metadata")
	}
	e.Location = nil
	if g := e.V1().Geometry; len(g.Coordinates) != 0 {
		t.Errorf("an unlocated event has v1 geometry %+v", g)
	}
}
