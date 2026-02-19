package models

import (
	"encoding/json"
	"time"
)

type Event struct {
	ID          string         `json:"id"`
	Title       string         `json:"title"`
	Description string         `json:"description,omitempty"`
	EventType   string         `json:"event_type"`
	Source      string         `json:"source"`
	Geometry    Geometry       `json:"geometry"`
	Magnitude   *float64       `json:"magnitude,omitempty"`
	Severity    string         `json:"severity,omitempty"`
	StartedAt   time.Time      `json:"started_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	URL         string         `json:"url,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type Geometry struct {
	Type        string    `json:"type"`
	Coordinates []float64 `json:"coordinates"`
}

// GeoJSON types

type FeatureCollection struct {
	Type     string    `json:"type"`
	Features []Feature `json:"features"`
}

type Feature struct {
	Type       string         `json:"type"`
	Geometry   Geometry       `json:"geometry"`
	Properties map[string]any `json:"properties"`
}

// Flat JSON response type

type EventsResponse struct {
	Events  []FlatEvent `json:"events"`
	Total   int         `json:"total"`
	Sources []string    `json:"sources"`
}

type FlatEvent struct {
	ID          string         `json:"id"`
	Title       string         `json:"title"`
	Description string         `json:"description,omitempty"`
	EventType   string         `json:"event_type"`
	Source      string         `json:"source"`
	Coordinates Coordinates    `json:"coordinates"`
	Magnitude   *float64       `json:"magnitude,omitempty"`
	Severity    string         `json:"severity,omitempty"`
	StartedAt   time.Time      `json:"started_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	URL         string         `json:"url,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type Coordinates struct {
	Longitude float64 `json:"longitude"`
	Latitude  float64 `json:"latitude"`
}

func (e Event) ToGeoJSONFeature() Feature {
	props := map[string]any{
		"id":         e.ID,
		"title":      e.Title,
		"event_type": e.EventType,
		"source":     e.Source,
		"started_at": e.StartedAt.Format(time.RFC3339),
		"updated_at": e.UpdatedAt.Format(time.RFC3339),
	}
	if e.Description != "" {
		props["description"] = e.Description
	}
	if e.Magnitude != nil {
		props["magnitude"] = *e.Magnitude
	}
	if e.Severity != "" {
		props["severity"] = e.Severity
	}
	if e.URL != "" {
		props["url"] = e.URL
	}
	if len(e.Metadata) > 0 {
		props["metadata"] = e.Metadata
	}

	return Feature{
		Type:       "Feature",
		Geometry:   e.Geometry,
		Properties: props,
	}
}

func (e Event) ToFlatEvent() FlatEvent {
	var coords Coordinates
	if len(e.Geometry.Coordinates) >= 2 {
		coords = Coordinates{
			Longitude: e.Geometry.Coordinates[0],
			Latitude:  e.Geometry.Coordinates[1],
		}
	}

	return FlatEvent{
		ID:          e.ID,
		Title:       e.Title,
		Description: e.Description,
		EventType:   e.EventType,
		Source:      e.Source,
		Coordinates: coords,
		Magnitude:   e.Magnitude,
		Severity:    e.Severity,
		StartedAt:   e.StartedAt,
		UpdatedAt:   e.UpdatedAt,
		URL:         e.URL,
		Metadata:    e.Metadata,
	}
}

func EventsToFeatureCollection(events []Event) FeatureCollection {
	features := make([]Feature, 0, len(events))
	for _, e := range events {
		features = append(features, e.ToGeoJSONFeature())
	}
	return FeatureCollection{
		Type:     "FeatureCollection",
		Features: features,
	}
}

func EventsToJSON(events []Event) EventsResponse {
	sourceSet := make(map[string]struct{})
	flat := make([]FlatEvent, 0, len(events))

	for _, e := range events {
		flat = append(flat, e.ToFlatEvent())
		sourceSet[e.Source] = struct{}{}
	}

	sources := make([]string, 0, len(sourceSet))
	for s := range sourceSet {
		sources = append(sources, s)
	}

	return EventsResponse{
		Events:  flat,
		Total:   len(flat),
		Sources: sources,
	}
}

func MarshalGeoJSON(events []Event) ([]byte, error) {
	return json.Marshal(EventsToFeatureCollection(events))
}

func MarshalEventsJSON(events []Event) ([]byte, error) {
	return json.Marshal(EventsToJSON(events))
}
