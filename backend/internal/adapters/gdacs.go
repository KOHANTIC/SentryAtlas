package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/KOHANTIC/SentryAtlas/backend/internal/event"
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

const (
	// gdacsWindowDays is how far back the window reaches.
	gdacsWindowDays = 30
	// gdacsParallelPages pages are read at once. GDACS serves 100 events a
	// page and ignores any page size; the 30-day window held 17 pages on
	// 2026-11-22, about 20 s one after another against a 30 s budget.
	gdacsParallelPages = 4
	// gdacsMaxPages bounds the window at 5,000 events.
	gdacsMaxPages = 50
)

type GDACSAdapter struct {
	client  *http.Client
	baseURL string
	maxBody int64
}

func NewGDACSAdapter(client *http.Client) *GDACSAdapter {
	return &GDACSAdapter{client: client, baseURL: gdacsBaseURL, maxBody: maxResponseBytes}
}

func (a *GDACSAdapter) Source() string {
	return "gdacs"
}

func (a *GDACSAdapter) SupportedTypes() []string {
	// "other" is emitted for GDACS codes outside gdacsEventTypeMap.
	return []string{"earthquake", "cyclone", "flood", "volcano", "drought", "wildfire", "other"}
}

// FetchEvents is FetchV2 projected onto the v1 model, which the service uses
// until it moves to v2 (#21).
func (a *GDACSAdapter) FetchEvents(ctx context.Context) ([]models.Event, error) {
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

// FetchV2 fetches every event of every alert level and every type GDACS has
// a code for over the last gdacsWindowDays, the window, page by page, and
// maps each onto v2. An event listed on two pages is kept once, as its
// latest record. One page that fails fails the window, so it is never
// served short.
func (a *GDACSAdapter) FetchV2(ctx context.Context) ([]event.Event, error) {
	codes := make([]string, 0, len(gdacsEventTypeMap))
	for code := range gdacsEventTypeMap {
		codes = append(codes, code)
	}
	sort.Strings(codes)

	now := time.Now().UTC()
	q := url.Values{}
	q.Set("eventlist", strings.Join(codes, ";"))
	q.Set("fromdate", now.AddDate(0, 0, -gdacsWindowDays).Format("2006-01-02"))
	q.Set("todate", now.Format("2006-01-02"))
	// Without it GDACS returns Orange and Red alerts only.
	q.Set("alertlevel", "Green;Orange;Red")

	var order []string
	latest := map[string]gdacsFeature{}
	for first := 1; first <= gdacsMaxPages; first += gdacsParallelPages {
		n := min(gdacsParallelPages, gdacsMaxPages-first+1)
		pages := make([][]gdacsFeature, n)
		errs := make([]error, n)
		var wg sync.WaitGroup
		for i := range n {
			wg.Add(1)
			go func() {
				defer wg.Done()
				pages[i], errs[i] = a.fetchPage(ctx, q, first+i)
			}()
		}
		wg.Wait()
		for _, err := range errs {
			if err != nil {
				return nil, err
			}
		}

		for _, page := range pages {
			fresh := 0
			for _, f := range page {
				key := gdacsKey(f.Properties)
				prev, seen := latest[key]
				if !seen {
					order = append(order, key)
					fresh++
				}
				if !seen || gdacsTime(f.Properties.DateModified).After(gdacsTime(prev.Properties.DateModified)) {
					latest[key] = f
				}
			}
			// An empty page is past the end (GDACS answers 204); a page with
			// nothing new means the page number was not honored.
			if fresh == 0 {
				return gdacsEvents(order, latest), nil
			}
		}
	}
	slog.Warn("gdacs: the window is longer than the page limit; later pages not read", "pages", gdacsMaxPages)
	return gdacsEvents(order, latest), nil
}

func (a *GDACSAdapter) fetchPage(ctx context.Context, q url.Values, page int) ([]gdacsFeature, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.baseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("gdacs: build request: %w", err)
	}
	pq := url.Values{}
	for k, v := range q {
		pq[k] = v
	}
	pq.Set("pagenumber", strconv.Itoa(page))
	req.URL.RawQuery = pq.Encode()
	// GDACS answers 204 No Content past the last page, or when the window is
	// empty; fetchList reads that as an empty list.
	return fetchList[gdacsFeature](a.client, req, "gdacs", "features", a.maxBody)
}

func gdacsEvents(order []string, latest map[string]gdacsFeature) []event.Event {
	events := make([]event.Event, 0, len(order))
	for _, key := range order {
		events = append(events, gdacsV2(latest[key]))
	}
	return events
}

// gdacsKey is an event's type and id, as in gdacs-EQ-1479624.
func gdacsKey(p gdacsProperties) string {
	id := p.EventID.String()
	if n, err := p.EventID.Int64(); err == nil {
		id = strconv.FormatInt(n, 10)
	}
	return p.EventType + "-" + id
}

// gdacsV2 maps one GDACS event onto v2.
func gdacsV2(f gdacsFeature) event.Event {
	p := f.Properties
	v := event.Event{
		ID:          "gdacs-" + gdacsKey(p),
		Source:      event.SourceGDACS,
		Type:        "other",
		Title:       p.Name,
		Description: p.Description,
		URL:         p.URL.Report,
		Time:        event.Time{Started: gdacsTime(p.FromDate), Updated: gdacsTime(p.DateModified)},
		Details: event.Details{GDACS: &event.GDACS{
			AlertLevel:   p.AlertLevel,
			AlertScore:   p.AlertScore,
			EpisodeID:    p.EpisodeID,
			Countries:    gdacsCountries(p),
			Current:      p.IsCurrent == "true",
			SeverityText: p.SeverityData.Text,
			FootprintURL: p.URL.Geometry,
		}},
	}
	if mapped, ok := gdacsEventTypeMap[p.EventType]; ok {
		v.Type = mapped
	}
	if v.Time.Updated.IsZero() {
		v.Time.Updated = v.Time.Started
	}
	if t := gdacsTime(p.ToDate); !t.IsZero() {
		v.Time.Ends = &t
	}
	if level := gdacsAlertToSeverity(p.AlertLevel); level != "" {
		v.Severity = &event.Severity{Level: level, SourceLevel: p.AlertLevel}
	}
	// A physical quantity in its unit (km/h, ha, km2, M); floods and
	// volcanoes come with neither.
	if s := p.SeverityData; s.Unit != "" && s.Value != 0 {
		v.Magnitude = &event.Magnitude{Value: s.Value, Unit: s.Unit}
	}
	if f.Geometry != nil {
		if c := f.Geometry.centroid(); c != nil {
			center := event.Point{event.RoundCoordinate(c[0]), event.RoundCoordinate(c[1])}
			loc := &event.Location{Center: &center}
			if f.Geometry.Type == "Point" {
				coords, _ := json.Marshal(center)
				loc.Geometry = &event.Geometry{Type: "Point", Coordinates: coords}
			} else {
				loc.Geometry = roundedGeometry(f.Geometry.polygons())
			}
			v.Location = loc
		}
	}
	return v
}

// gdacsCountries names the affected countries, from the list GDACS gives or,
// for the records without one, its comma-separated country field.
func gdacsCountries(p gdacsProperties) []string {
	var names []string
	for _, c := range p.AffectedCountries {
		if c.Name != "" {
			names = append(names, c.Name)
		}
	}
	if len(names) > 0 {
		return names
	}
	for _, c := range strings.Split(p.Country, ",") {
		if c = strings.TrimSpace(c); c != "" {
			names = append(names, c)
		}
	}
	return names
}

// gdacsTime parses a GDACS date, UTC without a zone or RFC 3339, into UTC;
// zero when absent or malformed.
func gdacsTime(s string) time.Time {
	if t, err := time.Parse("2006-01-02T15:04:05", s); err == nil {
		return t
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC()
	}
	return time.Time{}
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

type gdacsFeature struct {
	Geometry   *geoJSONGeometry `json:"geometry"`
	Properties gdacsProperties  `json:"properties"`
}

type gdacsProperties struct {
	EventType         string         `json:"eventtype"`
	EventID           json.Number    `json:"eventid"`
	EpisodeID         int            `json:"episodeid"`
	Name              string         `json:"name"`
	Description       string         `json:"description"`
	AlertLevel        string         `json:"alertlevel"`
	AlertScore        *float64       `json:"alertscore"`
	IsCurrent         string         `json:"iscurrent"`
	Country           string         `json:"country"`
	AffectedCountries []gdacsCountry `json:"affectedcountries"`
	FromDate          string         `json:"fromdate"`
	ToDate            string         `json:"todate"`
	DateModified      string         `json:"datemodified"`
	SeverityData      gdacsSeverity  `json:"severitydata"`
	URL               gdacsURL       `json:"url"`
}

type gdacsCountry struct {
	Name string `json:"countryname"`
}

type gdacsSeverity struct {
	Value float64 `json:"severity"`
	Text  string  `json:"severitytext"`
	Unit  string  `json:"severityunit"`
}

type gdacsURL struct {
	Report   string `json:"report"`
	Geometry string `json:"geometry"`
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
