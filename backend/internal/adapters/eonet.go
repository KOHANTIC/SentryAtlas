package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/kohantic/disaster-watch/backend/internal/models"
)

const eonetBaseURL = "https://eonet.gsfc.nasa.gov/api/v3/events"

// EONET category ID -> our event type
var eonetCategoryMap = map[string]string{
	"wildfires":     "wildfire",
	"volcanoes":     "volcano",
	"severeStorms":  "storm",
	"seaLakeIce":    "iceberg",
	"earthquakes":   "earthquake",
	"floods":        "flood",
	"landslides":    "landslide",
	"drought":       "drought",
}

// Reverse map: our event type -> EONET category ID
var eventTypeToEONET = map[string]string{
	"wildfire":   "wildfires",
	"volcano":    "volcanoes",
	"storm":      "severeStorms",
	"iceberg":    "seaLakeIce",
	"earthquake": "earthquakes",
	"flood":      "floods",
	"landslide":  "landslides",
	"drought":    "drought",
}

type EONETAdapter struct {
	client *http.Client
}

func NewEONETAdapter(client *http.Client) *EONETAdapter {
	return &EONETAdapter{client: client}
}

func (a *EONETAdapter) Source() string {
	return "eonet"
}

func (a *EONETAdapter) SupportedTypes() []string {
	return []string{"wildfire", "volcano", "storm", "iceberg", "earthquake", "flood", "landslide", "drought"}
}

func (a *EONETAdapter) FetchEvents(ctx context.Context, params FetchParams) ([]models.Event, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, eonetBaseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("eonet: build request: %w", err)
	}

	q := req.URL.Query()
	q.Set("status", "open")

	if params.Limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", params.Limit))
	}

	if len(params.Types) > 0 {
		for _, t := range params.Types {
			if cat, ok := eventTypeToEONET[t]; ok {
				q.Add("category", cat)
			}
		}
	}

	req.URL.RawQuery = q.Encode()

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("eonet: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("eonet: unexpected status %d", resp.StatusCode)
	}

	var result eonetResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("eonet: decode response: %w", err)
	}

	events := make([]models.Event, 0, len(result.Events))
	for _, e := range result.Events {
		parsed := parseEONETEvent(e)
		if !params.Since.IsZero() && parsed.StartedAt.Before(params.Since) {
			continue
		}
		events = append(events, parsed)
	}

	return events, nil
}

func parseEONETEvent(e eonetEvent) models.Event {
	eventType := "other"
	if len(e.Categories) > 0 {
		if mapped, ok := eonetCategoryMap[e.Categories[0].ID]; ok {
			eventType = mapped
		}
	}

	var coords []float64
	var startedAt time.Time
	if len(e.Geometry) > 0 {
		last := e.Geometry[len(e.Geometry)-1]
		if len(last.Coordinates) >= 2 {
			coords = last.Coordinates[:2]
		}
		if t, err := time.Parse(time.RFC3339, last.Date); err == nil {
			startedAt = t
		}
	}

	var mag *float64
	if e.MagnitudeValue != nil {
		mag = e.MagnitudeValue
	}

	return models.Event{
		ID:          fmt.Sprintf("eonet-%s", e.ID),
		Title:       e.Title,
		Description: e.Description,
		EventType:   eventType,
		Source:      "eonet",
		Geometry: models.Geometry{
			Type:        "Point",
			Coordinates: coords,
		},
		Magnitude: mag,
		StartedAt: startedAt,
		UpdatedAt: startedAt,
		URL:       e.Link,
	}
}

// EONET response types

type eonetResponse struct {
	Events []eonetEvent `json:"events"`
}

type eonetEvent struct {
	ID                   string           `json:"id"`
	Title                string           `json:"title"`
	Description          string           `json:"description"`
	Link                 string           `json:"link"`
	Closed               *string          `json:"closed"`
	Categories           []eonetCategory  `json:"categories"`
	Geometry             []eonetGeometry  `json:"geometry"`
	MagnitudeValue       *float64         `json:"magnitudeValue"`
	MagnitudeUnit        string           `json:"magnitudeUnit"`
}

type eonetCategory struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type eonetGeometry struct {
	Date        string    `json:"date"`
	Type        string    `json:"type"`
	Coordinates []float64 `json:"coordinates"`
}
