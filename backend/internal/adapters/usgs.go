package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/KOHANTIC/SentryAtlas/backend/internal/event"
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

// SupportedTypes covers what the feed holds besides earthquakes: landslides,
// and quarry blasts, explosions and ice quakes, which are "other".
func (a *USGSAdapter) SupportedTypes() []string {
	return []string{"earthquake", "landslide", "other"}
}

// FetchEvents is FetchV2 projected onto the v1 model, which the service uses
// until it moves to v2 (#21).
func (a *USGSAdapter) FetchEvents(ctx context.Context) ([]models.Event, error) {
	v2, err := a.FetchV2(ctx)
	if err != nil {
		return nil, err
	}
	events := make([]models.Event, 0, len(v2))
	for _, e := range v2 {
		events = append(events, e.V1())
	}
	return events, nil
}

// FetchV2 fetches the window and maps each record onto the v2 model.
func (a *USGSAdapter) FetchV2(ctx context.Context) ([]event.Event, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.baseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("usgs: build request: %w", err)
	}

	features, err := fetchList[usgsFeature](a.client, req, "usgs", "features", a.maxBody)
	if err != nil {
		return nil, err
	}

	events := make([]event.Event, 0, len(features))
	for _, f := range features {
		events = append(events, usgsEvent(f))
	}
	return events, nil
}

// usgsEvent maps one feed record onto the v2 model.
func usgsEvent(f usgsFeature) event.Event {
	p := f.Properties
	kind := p.Type
	if kind == "" {
		kind = "earthquake"
	}
	e := event.Event{
		ID:     "usgs-" + f.ID,
		Source: event.SourceUSGS,
		Type:   usgsType(kind),
		Title:  p.Title,
		URL:    p.URL,
		Time: event.Time{
			Started: time.UnixMilli(int64(p.Time)).UTC(),
			Updated: time.UnixMilli(int64(p.Updated)).UTC(),
		},
		Details: event.Details{USGS: &event.USGS{
			Kind:         kind,
			Status:       p.Status,
			Place:        p.Place,
			Tsunami:      p.Tsunami == 1,
			Felt:         p.Felt,
			CDI:          p.CDI,
			MMI:          p.MMI,
			Significance: p.Sig,
			Alert:        p.Alert,
			DetailURL:    p.Detail,
		}},
	}

	// Presence-based, not zero-based: a magnitude-0.0 quake is real data,
	// distinct from USGS omitting the field.
	if p.Mag != nil {
		unit := p.MagType
		if unit == "" {
			unit = "M"
		}
		e.Magnitude = &event.Magnitude{Value: *p.Mag, Unit: unit}
	}
	if level := usgsAlertToSeverity(p.Alert); level != "" {
		e.Severity = &event.Severity{Level: level, SourceLevel: p.Alert}
	}

	c := f.Geometry.Coordinates
	if len(c) >= 2 && c[0] != nil && c[1] != nil {
		center := event.Point{event.RoundCoordinate(*c[0]), event.RoundCoordinate(*c[1])}
		coords, _ := json.Marshal(center)
		e.Location = &event.Location{Center: &center, Geometry: &event.Geometry{Type: "Point", Coordinates: coords}}
	}
	// A missing or null depth stays unknown; it used to read as 0 km.
	if len(c) >= 3 && c[2] != nil {
		depth := *c[2]
		e.Details.USGS.DepthKm = &depth
	}
	return e
}

// usgsType is the event type of a USGS kind: only earthquakes are
// earthquakes; quarry blasts, explosions and ice quakes are "other".
func usgsType(kind string) string {
	switch kind {
	case "earthquake":
		return "earthquake"
	case "landslide":
		return "landslide"
	default:
		return "other"
	}
}

func usgsAlertToSeverity(alert string) string {
	switch alert {
	case "red":
		return event.SeverityExtreme
	case "orange":
		return event.SeveritySevere
	case "yellow":
		return event.SeverityModerate
	case "green":
		return event.SeverityMinor
	default:
		return ""
	}
}

type usgsFeature struct {
	ID         string         `json:"id"`
	Geometry   usgsGeometry   `json:"geometry"`
	Properties usgsProperties `json:"properties"`
}

type usgsGeometry struct {
	// Pointers, so a null depth is told apart from 0 km.
	Coordinates []*float64 `json:"coordinates"`
}

type usgsProperties struct {
	Mag     *float64 `json:"mag"`
	MagType string   `json:"magType"`
	Place   string   `json:"place"`
	Time    float64  `json:"time"`
	Updated float64  `json:"updated"`
	URL     string   `json:"url"`
	Detail  string   `json:"detail"`
	Title   string   `json:"title"`
	Alert   string   `json:"alert"`
	Type    string   `json:"type"`
	Status  string   `json:"status"`
	Tsunami int      `json:"tsunami"`
	Felt    *int     `json:"felt"`
	CDI     *float64 `json:"cdi"`
	MMI     *float64 `json:"mmi"`
	Sig     int      `json:"sig"`
}
