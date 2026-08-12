package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/KOHANTIC/SentryAtlas/backend/internal/models"
)

const gdacsBaseURL = "https://www.gdacs.org/gdacsapi/api/events/geteventlist/SEARCH"

// GDACS event type codes -> our event type
var gdacsEventTypeMap = map[string]string{
	"EQ": "earthquake",
	"TC": "cyclone",
	"FL": "flood",
	"VO": "volcano",
	"DR": "drought",
	"WF": "wildfire",
}

// Reverse map: our event type -> GDACS code
var eventTypeToGDACS = map[string]string{
	"earthquake": "EQ",
	"cyclone":    "TC",
	"flood":      "FL",
	"volcano":    "VO",
	"drought":    "DR",
	"wildfire":   "WF",
}

type GDACSAdapter struct {
	client  *http.Client
	baseURL string
}

func NewGDACSAdapter(client *http.Client) *GDACSAdapter {
	return &GDACSAdapter{client: client, baseURL: gdacsBaseURL}
}

func (a *GDACSAdapter) Source() string {
	return "gdacs"
}

func (a *GDACSAdapter) SupportedTypes() []string {
	// "other" is emitted for GDACS codes outside gdacsEventTypeMap;
	// requesting it fetches the full window and filters server-side.
	return []string{"earthquake", "cyclone", "flood", "volcano", "drought", "wildfire", "other"}
}

func (a *GDACSAdapter) FetchEvents(ctx context.Context, params FetchParams) ([]models.Event, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.baseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("gdacs: build request: %w", err)
	}

	q := req.URL.Query()

	if len(params.Types) > 0 {
		var codes []string
		for _, t := range params.Types {
			if code, ok := eventTypeToGDACS[t]; ok {
				codes = append(codes, code)
			}
		}
		if len(codes) > 0 {
			q.Set("eventlist", strings.Join(codes, ";"))
		}
	}

	if !params.Since.IsZero() {
		q.Set("fromdate", params.Since.Format("2006-01-02"))
	} else {
		q.Set("fromdate", time.Now().AddDate(0, 0, -30).Format("2006-01-02"))
	}
	q.Set("todate", time.Now().Format("2006-01-02"))

	req.URL.RawQuery = q.Encode()

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gdacs: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gdacs: unexpected status %d", resp.StatusCode)
	}

	var result gdacsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("gdacs: decode response: %w", err)
	}

	events := make([]models.Event, 0, len(result.Features))
	for _, f := range result.Features {
		e := parseGDACSFeature(f)
		events = append(events, e)
	}

	return events, nil
}

func parseGDACSFeature(f gdacsFeature) models.Event {
	eventType := "other"
	if mapped, ok := gdacsEventTypeMap[f.Properties.EventType]; ok {
		eventType = mapped
	}

	severity := gdacsAlertToSeverity(f.Properties.AlertLevel)

	var coords []float64
	if f.Geometry.Type == "Point" && len(f.Geometry.Coordinates) >= 2 {
		coords = f.Geometry.Coordinates[:2]
	}

	var startedAt time.Time
	if f.Properties.FromDate != "" {
		if t, err := time.Parse("2006-01-02T15:04:05", f.Properties.FromDate); err == nil {
			startedAt = t
		} else if t, err := time.Parse(time.RFC3339, f.Properties.FromDate); err == nil {
			startedAt = t
		}
	}

	var updatedAt time.Time
	if f.Properties.ToDate != "" {
		if t, err := time.Parse("2006-01-02T15:04:05", f.Properties.ToDate); err == nil {
			updatedAt = t
		} else if t, err := time.Parse(time.RFC3339, f.Properties.ToDate); err == nil {
			updatedAt = t
		}
	}
	if updatedAt.IsZero() {
		updatedAt = startedAt
	}

	metadata := map[string]any{
		"alert_level":   f.Properties.AlertLevel,
		"country":       f.Properties.Country,
		"severity_unit": f.Properties.SeverityUnit,
	}
	// GDACS "severity" is a physical quantity in severity_unit (e.g. wind
	// km/h) — it must not masquerade as the magnitude field, which would
	// mix units with Richter magnitudes from USGS.
	if f.Properties.Severity != 0 {
		metadata["severity_value"] = f.Properties.Severity
	}

	eventID := f.Properties.EventID.String()
	if n, err := f.Properties.EventID.Int64(); err == nil {
		eventID = strconv.FormatInt(n, 10)
	}

	return models.Event{
		ID:          fmt.Sprintf("gdacs-%s-%s", f.Properties.EventType, eventID),
		Title:       f.Properties.Name,
		Description: f.Properties.Description,
		EventType:   eventType,
		Source:      "gdacs",
		Geometry: models.Geometry{
			Type:        "Point",
			Coordinates: coords,
		},
		Severity:  severity,
		StartedAt: startedAt,
		UpdatedAt: updatedAt,
		URL:       f.Properties.URL.Report,
		Metadata:  metadata,
	}
}

func gdacsAlertToSeverity(level string) string {
	switch strings.ToLower(level) {
	case "red":
		return "extreme"
	case "orange":
		return "severe"
	case "green":
		return "minor"
	default:
		return ""
	}
}

// GDACS GeoJSON response types

type gdacsResponse struct {
	Features []gdacsFeature `json:"features"`
}

type gdacsFeature struct {
	Geometry   gdacsGeometry   `json:"geometry"`
	Properties gdacsProperties `json:"properties"`
}

type gdacsGeometry struct {
	Type        string    `json:"type"`
	Coordinates []float64 `json:"coordinates"`
}

type gdacsProperties struct {
	EventType    string      `json:"eventtype"`
	EventID      json.Number `json:"eventid"`
	Name         string      `json:"name"`
	Description  string      `json:"description"`
	AlertLevel   string      `json:"alertlevel"`
	Severity     float64     `json:"severity"`
	SeverityUnit string      `json:"severityunit"`
	Country      string      `json:"country"`
	FromDate     string      `json:"fromdate"`
	ToDate       string      `json:"todate"`
	URL          gdacsURL    `json:"url"`
}

type gdacsURL struct {
	Report string `json:"report"`
}

func (u *gdacsURL) UnmarshalJSON(data []byte) error {
	// Handle both string and object forms
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		u.Report = s
		return nil
	}
	type alias gdacsURL
	var obj alias
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	*u = gdacsURL(obj)
	return nil
}
