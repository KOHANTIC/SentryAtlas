package adapters

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/KOHANTIC/SentryAtlas/backend/internal/models"
)

// The past month of earthquakes of every magnitude: USGS serves it from its
// CDN and updates it every minute, so it is the window, not a query.
const usgsBaseURL = "https://earthquake.usgs.gov/earthquakes/feed/v1.0/summary/all_month.geojson"

type USGSAdapter struct {
	client  *http.Client
	baseURL string
	maxBody int64
}

func NewUSGSAdapter(client *http.Client) *USGSAdapter {
	return &USGSAdapter{client: client, baseURL: usgsBaseURL, maxBody: maxResponseBytes}
}

func (a *USGSAdapter) Source() string {
	return "usgs"
}

func (a *USGSAdapter) SupportedTypes() []string {
	return []string{"earthquake"}
}

func (a *USGSAdapter) FetchEvents(ctx context.Context) ([]models.Event, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.baseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("usgs: build request: %w", err)
	}

	features, err := fetchList[usgsFeature](a.client, req, "usgs", "features", a.maxBody)
	if err != nil {
		return nil, err
	}

	events := make([]models.Event, 0, len(features))
	for _, f := range features {
		events = append(events, parseUSGSFeature(f))
	}

	return events, nil
}

func parseUSGSFeature(f usgsFeature) models.Event {
	var coords []float64
	if len(f.Geometry.Coordinates) >= 2 {
		coords = f.Geometry.Coordinates[:2]
	}

	startedAt := time.UnixMilli(int64(f.Properties.Time))
	updatedAt := time.UnixMilli(int64(f.Properties.Updated))

	// Presence-based, not zero-based: a magnitude-0.0 quake is real data,
	// distinct from USGS omitting the field.
	var mag *float64
	if f.Properties.Mag != nil {
		m := *f.Properties.Mag
		mag = &m
	}

	severity := usgsAlertToSeverity(f.Properties.Alert)

	return models.Event{
		ID:        fmt.Sprintf("usgs-%s", f.ID),
		Title:     f.Properties.Title,
		EventType: "earthquake",
		Source:    "usgs",
		Geometry: models.Geometry{
			Type:        f.Geometry.Type,
			Coordinates: coords,
		},
		Magnitude: mag,
		Severity:  severity,
		StartedAt: startedAt,
		UpdatedAt: updatedAt,
		URL:       f.Properties.URL,
		Metadata: map[string]any{
			"place": f.Properties.Place,
			"depth": depthFromCoords(f.Geometry.Coordinates),
		},
	}
}

func usgsAlertToSeverity(alert string) string {
	switch alert {
	case "red":
		return "extreme"
	case "orange":
		return "severe"
	case "yellow":
		return "moderate"
	case "green":
		return "minor"
	default:
		return ""
	}
}

func depthFromCoords(coords []float64) float64 {
	if len(coords) >= 3 {
		return coords[2]
	}
	return 0
}

// USGS GeoJSON response types

type usgsFeature struct {
	ID         string         `json:"id"`
	Geometry   usgsGeometry   `json:"geometry"`
	Properties usgsProperties `json:"properties"`
}

type usgsGeometry struct {
	Type        string    `json:"type"`
	Coordinates []float64 `json:"coordinates"`
}

type usgsProperties struct {
	Mag     *float64 `json:"mag"`
	Place   string   `json:"place"`
	Time    float64  `json:"time"`
	Updated float64  `json:"updated"`
	URL     string   `json:"url"`
	Title   string   `json:"title"`
	Alert   string   `json:"alert"`
}
