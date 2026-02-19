package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/KOHANTIC/SentryAtlas/backend/internal/models"
)

const usgsBaseURL = "https://earthquake.usgs.gov/fdsnws/event/1/query"

type USGSAdapter struct {
	client *http.Client
}

func NewUSGSAdapter(client *http.Client) *USGSAdapter {
	return &USGSAdapter{client: client}
}

func (a *USGSAdapter) Source() string {
	return "usgs"
}

func (a *USGSAdapter) SupportedTypes() []string {
	return []string{"earthquake"}
}

func (a *USGSAdapter) FetchEvents(ctx context.Context, params FetchParams) ([]models.Event, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, usgsBaseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("usgs: build request: %w", err)
	}

	q := req.URL.Query()
	q.Set("format", "geojson")

	if !params.Since.IsZero() {
		q.Set("starttime", params.Since.Format(time.RFC3339))
	} else {
		q.Set("starttime", time.Now().AddDate(0, 0, -7).Format(time.RFC3339))
	}

	if params.BBox != nil {
		q.Set("minlongitude", fmt.Sprintf("%f", params.BBox.MinLon))
		q.Set("minlatitude", fmt.Sprintf("%f", params.BBox.MinLat))
		q.Set("maxlongitude", fmt.Sprintf("%f", params.BBox.MaxLon))
		q.Set("maxlatitude", fmt.Sprintf("%f", params.BBox.MaxLat))
	}

	if params.Limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", params.Limit))
	}

	req.URL.RawQuery = q.Encode()

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("usgs: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("usgs: unexpected status %d", resp.StatusCode)
	}

	var result usgsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("usgs: decode response: %w", err)
	}

	events := make([]models.Event, 0, len(result.Features))
	for _, f := range result.Features {
		e, err := parseUSGSFeature(f)
		if err != nil {
			continue
		}
		events = append(events, e)
	}

	return events, nil
}

func parseUSGSFeature(f usgsFeature) (models.Event, error) {
	var coords []float64
	if len(f.Geometry.Coordinates) >= 2 {
		coords = f.Geometry.Coordinates[:2]
	}

	startedAt := time.UnixMilli(int64(f.Properties.Time))
	updatedAt := time.UnixMilli(int64(f.Properties.Updated))

	var mag *float64
	if f.Properties.Mag != 0 {
		m := f.Properties.Mag
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
	}, nil
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

type usgsResponse struct {
	Features []usgsFeature `json:"features"`
}

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
	Mag     float64 `json:"mag"`
	Place   string  `json:"place"`
	Time    float64 `json:"time"`
	Updated float64 `json:"updated"`
	URL     string  `json:"url"`
	Title   string  `json:"title"`
	Alert   string  `json:"alert"`
}
