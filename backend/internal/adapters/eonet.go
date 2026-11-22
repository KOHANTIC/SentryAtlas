package adapters

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/KOHANTIC/SentryAtlas/backend/internal/models"
)

const eonetBaseURL = "https://eonet.gsfc.nasa.gov/api/v3/events"

// EONET category ID -> our event type
var eonetCategoryMap = map[string]string{
	"wildfires":    "wildfire",
	"volcanoes":    "volcano",
	"severeStorms": "storm",
	"seaLakeIce":   "iceberg",
	"earthquakes":  "earthquake",
	"floods":       "flood",
	"landslides":   "landslide",
	"drought":      "drought",
}

type EONETAdapter struct {
	client  *http.Client
	baseURL string
	maxBody int64
}

func NewEONETAdapter(client *http.Client) *EONETAdapter {
	return &EONETAdapter{client: client, baseURL: eonetBaseURL, maxBody: maxResponseBytes}
}

func (a *EONETAdapter) Source() string {
	return "eonet"
}

func (a *EONETAdapter) SupportedTypes() []string {
	// "other" is emitted for EONET categories outside eonetCategoryMap.
	return []string{"wildfire", "volcano", "storm", "iceberg", "earthquake", "flood", "landslide", "drought", "other"}
}

// FetchEvents fetches every open event in every category: the window.
func (a *EONETAdapter) FetchEvents(ctx context.Context) ([]models.Event, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.baseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("eonet: build request: %w", err)
	}

	q := req.URL.Query()
	q.Set("status", "open")
	req.URL.RawQuery = q.Encode()

	list, err := fetchList[eonetEvent](a.client, req, "eonet", "events", a.maxBody)
	if err != nil {
		return nil, err
	}

	events := make([]models.Event, 0, len(list))
	for _, e := range list {
		events = append(events, parseEONETEvent(e))
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

type eonetEvent struct {
	ID             string          `json:"id"`
	Title          string          `json:"title"`
	Description    string          `json:"description"`
	Link           string          `json:"link"`
	Closed         *string         `json:"closed"`
	Categories     []eonetCategory `json:"categories"`
	Geometry       []eonetGeometry `json:"geometry"`
	MagnitudeValue *float64        `json:"magnitudeValue"`
	MagnitudeUnit  string          `json:"magnitudeUnit"`
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
