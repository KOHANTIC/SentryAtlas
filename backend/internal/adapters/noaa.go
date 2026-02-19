package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Kohantic/SentryAtlas/backend/internal/models"
)

const noaaBaseURL = "https://api.weather.gov/alerts/active"

// NWS event keywords -> our event type
var noaaEventTypeMap = map[string]string{
	"tornado":         "tornado",
	"flood":           "flood",
	"flash flood":     "flood",
	"hurricane":       "hurricane",
	"typhoon":         "hurricane",
	"tropical storm":  "storm",
	"severe storm":    "storm",
	"thunderstorm":    "storm",
	"winter storm":    "winter_storm",
	"blizzard":        "winter_storm",
	"ice storm":       "winter_storm",
	"tsunami":         "tsunami",
	"earthquake":      "earthquake",
	"volcano":         "volcano",
	"wildfire":        "wildfire",
	"fire":            "wildfire",
	"red flag":        "wildfire",
}

type NOAAAdapter struct {
	client    *http.Client
	userAgent string
}

func NewNOAAAdapter(client *http.Client, userAgent string) *NOAAAdapter {
	return &NOAAAdapter{client: client, userAgent: userAgent}
}

func (a *NOAAAdapter) Source() string {
	return "noaa"
}

func (a *NOAAAdapter) SupportedTypes() []string {
	return []string{"flood", "storm", "tornado", "hurricane", "winter_storm", "tsunami", "wildfire"}
}

func (a *NOAAAdapter) FetchEvents(ctx context.Context, params FetchParams) ([]models.Event, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, noaaBaseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("noaa: build request: %w", err)
	}

	req.Header.Set("User-Agent", a.userAgent)
	req.Header.Set("Accept", "application/geo+json")

	q := req.URL.Query()
	q.Set("status", "actual")
	q.Set("message_type", "alert")
	req.URL.RawQuery = q.Encode()

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("noaa: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("noaa: unexpected status %d", resp.StatusCode)
	}

	var result noaaResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("noaa: decode response: %w", err)
	}

	requestedTypes := make(map[string]struct{})
	for _, t := range params.Types {
		requestedTypes[t] = struct{}{}
	}

	events := make([]models.Event, 0, len(result.Features))
	for _, f := range result.Features {
		e := parseNOAAFeature(f)
		if e.EventType == "" {
			continue
		}
		if len(requestedTypes) > 0 {
			if _, ok := requestedTypes[e.EventType]; !ok {
				continue
			}
		}
		if !params.Since.IsZero() && e.StartedAt.Before(params.Since) {
			continue
		}
		events = append(events, e)
	}

	return events, nil
}

func parseNOAAFeature(f noaaFeature) models.Event {
	eventType := classifyNOAAEvent(f.Properties.Event)

	severity := strings.ToLower(f.Properties.Severity)
	switch severity {
	case "extreme":
		severity = "extreme"
	case "severe":
		severity = "severe"
	case "moderate":
		severity = "moderate"
	case "minor":
		severity = "minor"
	default:
		severity = ""
	}

	var coords []float64
	if f.Geometry != nil {
		coords = f.Geometry.centroid()
	}

	var startedAt, updatedAt time.Time
	if f.Properties.Onset != "" {
		if t, err := time.Parse(time.RFC3339, f.Properties.Onset); err == nil {
			startedAt = t
		}
	}
	if f.Properties.Sent != "" {
		if t, err := time.Parse(time.RFC3339, f.Properties.Sent); err == nil {
			updatedAt = t
		}
	}
	if startedAt.IsZero() {
		startedAt = updatedAt
	}

	return models.Event{
		ID:          fmt.Sprintf("noaa-%s", f.Properties.ID),
		Title:       f.Properties.Headline,
		Description: f.Properties.Description,
		EventType:   eventType,
		Source:      "noaa",
		Geometry: models.Geometry{
			Type:        "Point",
			Coordinates: coords,
		},
		Severity:  severity,
		StartedAt: startedAt,
		UpdatedAt: updatedAt,
		URL:       f.Properties.Web,
		Metadata: map[string]any{
			"event":       f.Properties.Event,
			"area_desc":   f.Properties.AreaDesc,
			"urgency":     f.Properties.Urgency,
			"certainty":   f.Properties.Certainty,
			"sender_name": f.Properties.SenderName,
		},
	}
}

func classifyNOAAEvent(event string) string {
	lower := strings.ToLower(event)
	for keyword, eventType := range noaaEventTypeMap {
		if strings.Contains(lower, keyword) {
			return eventType
		}
	}
	return "weather"
}

// NOAA GeoJSON-LD response types

type noaaResponse struct {
	Features []noaaFeature `json:"features"`
}

type noaaFeature struct {
	Geometry   *noaaGeometry  `json:"geometry"`
	Properties noaaProperties `json:"properties"`
}

type noaaGeometry struct {
	Type        string          `json:"type"`
	Coordinates json.RawMessage `json:"coordinates"`
}

// centroid returns a [lon, lat] pair from the raw GeoJSON coordinates,
// computing a simple centroid for Polygon rings.
func (g *noaaGeometry) centroid() []float64 {
	switch g.Type {
	case "Point":
		var pt []float64
		if err := json.Unmarshal(g.Coordinates, &pt); err == nil && len(pt) >= 2 {
			return pt[:2]
		}
	case "Polygon":
		var rings [][][]float64
		if err := json.Unmarshal(g.Coordinates, &rings); err == nil && len(rings) > 0 && len(rings[0]) > 0 {
			var sumLon, sumLat float64
			for _, pt := range rings[0] {
				sumLon += pt[0]
				sumLat += pt[1]
			}
			n := float64(len(rings[0]))
			return []float64{sumLon / n, sumLat / n}
		}
	case "MultiPolygon":
		var polys [][][][]float64
		if err := json.Unmarshal(g.Coordinates, &polys); err == nil && len(polys) > 0 && len(polys[0]) > 0 && len(polys[0][0]) > 0 {
			var sumLon, sumLat float64
			for _, pt := range polys[0][0] {
				sumLon += pt[0]
				sumLat += pt[1]
			}
			n := float64(len(polys[0][0]))
			return []float64{sumLon / n, sumLat / n}
		}
	}
	return nil
}

type noaaProperties struct {
	ID          string `json:"id"`
	Event       string `json:"event"`
	Headline    string `json:"headline"`
	Description string `json:"description"`
	Severity    string `json:"severity"`
	Urgency     string `json:"urgency"`
	Certainty   string `json:"certainty"`
	Onset       string `json:"onset"`
	Sent        string `json:"sent"`
	AreaDesc    string `json:"areaDesc"`
	SenderName  string `json:"senderName"`
	Web         string `json:"web"`
}
