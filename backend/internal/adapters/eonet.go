package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"time"

	"github.com/KOHANTIC/SentryAtlas/backend/internal/event"
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

// FetchEvents is FetchV2 projected onto the v1 model, which the service uses
// until it moves to v2 (#21).
func (a *EONETAdapter) FetchEvents(ctx context.Context) ([]models.Event, error) {
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

// FetchV2 fetches every open event in every category, the window, and maps
// each onto the v2 model.
func (a *EONETAdapter) FetchV2(ctx context.Context) ([]event.Event, error) {
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
	events := make([]event.Event, 0, len(list))
	for _, e := range list {
		events = append(events, eonetV2(e))
	}
	return events, nil
}

// eonetV2 maps one EONET event onto v2. Its observations, in date order,
// give the start (the first), the last update (the latest), the magnitude
// with its unit (the latest that has one), and the location: the latest
// polygon, or the track of its points ending at the latest position.
func eonetV2(e eonetEvent) event.Event {
	v := event.Event{
		ID:          "eonet-" + e.ID,
		Source:      event.SourceEONET,
		Type:        "other",
		Title:       e.Title,
		Description: e.Description,
		URL:         e.Link,
		Details:     event.Details{EONET: &event.EONET{}},
	}
	for i, c := range e.Categories {
		v.Details.EONET.Categories = append(v.Details.EONET.Categories, c.ID)
		if mapped, ok := eonetCategoryMap[c.ID]; ok && i == 0 {
			v.Type = mapped
		}
	}
	for _, s := range e.Sources {
		v.Details.EONET.Sources = append(v.Details.EONET.Sources, event.Link{ID: s.ID, URL: s.URL})
	}
	if e.Closed != nil {
		if t := eonetTime(*e.Closed); !t.IsZero() {
			v.Details.EONET.Closed = &t
		}
	}

	observations := append([]eonetGeometry(nil), e.Geometry...)
	sort.SliceStable(observations, func(i, j int) bool {
		return eonetTime(observations[i].Date).Before(eonetTime(observations[j].Date))
	})

	var track []event.Point
	var latestPolygon [][][]float64
	latestIsPolygon := false
	for _, g := range observations {
		if t := eonetTime(g.Date); !t.IsZero() {
			if v.Time.Started.IsZero() {
				v.Time.Started = t
			}
			v.Time.Updated = t
		}
		switch g.Type {
		case "Point":
			var p []float64
			if json.Unmarshal(g.Coordinates, &p) == nil && len(p) >= 2 {
				if pt, ok := lonLat(p[0], p[1]); ok {
					if len(track) == 0 || track[len(track)-1] != pt {
						track = append(track, pt)
					}
					latestIsPolygon = false
				}
			}
		case "Polygon":
			var rings [][][]float64
			if json.Unmarshal(g.Coordinates, &rings) == nil && len(rings) > 0 && wrapRings(rings) {
				latestPolygon, latestIsPolygon = rings, true
			}
		}
		if g.MagnitudeValue != nil && g.MagnitudeUnit != "" {
			v.Magnitude = &event.Magnitude{Value: *g.MagnitudeValue, Unit: g.MagnitudeUnit}
		}
	}
	// Older responses put the magnitude on the event itself.
	if v.Magnitude == nil && e.MagnitudeValue != nil && e.MagnitudeUnit != "" {
		v.Magnitude = &event.Magnitude{Value: *e.MagnitudeValue, Unit: e.MagnitudeUnit}
	}

	switch {
	case latestIsPolygon:
		polygons := [][][][]float64{latestPolygon}
		if c := areaCentroid(polygons); c != nil {
			center := event.Point{event.RoundCoordinate(c[0]), event.RoundCoordinate(c[1])}
			v.Location = &event.Location{Center: &center, Geometry: roundedGeometry(polygons)}
		}
	case len(track) == 1:
		coords, _ := json.Marshal(track[0])
		center := track[0]
		v.Location = &event.Location{Center: &center, Geometry: &event.Geometry{Type: "Point", Coordinates: coords}}
	case len(track) > 1:
		if len(track) > event.MaxVertices {
			track = track[len(track)-event.MaxVertices:]
		}
		coords, _ := json.Marshal(track)
		center := track[len(track)-1]
		v.Location = &event.Location{Center: &center, Geometry: &event.Geometry{Type: "LineString", Coordinates: coords}}
	}
	return v
}

// lonLat is a position as a rounded point, its longitude wrapped into
// -180..180; false when it cannot be one. EONET sends some longitudes past
// 180° ([189, 7] is -171) and some latitudes past 90° ([14, 200]).
func lonLat(lon, lat float64) (event.Point, bool) {
	if math.IsNaN(lon) || math.IsNaN(lat) || lat < -90 || lat > 90 || lon < -360 || lon > 360 {
		return event.Point{}, false
	}
	if lon > 180 {
		lon -= 360
	} else if lon < -180 {
		lon += 360
	}
	return event.Point{event.RoundCoordinate(lon), event.RoundCoordinate(lat)}, true
}

// wrapRings wraps every vertex of a polygon in place, as lonLat does; false,
// leaving the polygon unused, when a vertex cannot be a position.
func wrapRings(rings [][][]float64) bool {
	for _, ring := range rings {
		for _, v := range ring {
			if len(v) < 2 {
				continue
			}
			p, ok := lonLat(v[0], v[1])
			if !ok {
				return false
			}
			v[0], v[1] = p[0], p[1]
		}
	}
	return true
}

// eonetTime parses an EONET date into UTC; zero when absent or malformed.
func eonetTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t.UTC()
}

type eonetEvent struct {
	ID          string          `json:"id"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Link        string          `json:"link"`
	Closed      *string         `json:"closed"`
	Categories  []eonetCategory `json:"categories"`
	Sources     []eonetSource   `json:"sources"`
	Geometry    []eonetGeometry `json:"geometry"`
	// The magnitude belongs on each observation; older responses put it here.
	MagnitudeValue *float64 `json:"magnitudeValue"`
	MagnitudeUnit  string   `json:"magnitudeUnit"`
}

type eonetCategory struct {
	ID string `json:"id"`
}

type eonetSource struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

type eonetGeometry struct {
	Date string `json:"date"`
	Type string `json:"type"`
	// Raw, so a polygon observation is read rather than failing the event.
	Coordinates    json.RawMessage `json:"coordinates"`
	MagnitudeValue *float64        `json:"magnitudeValue"`
	MagnitudeUnit  string          `json:"magnitudeUnit"`
}
