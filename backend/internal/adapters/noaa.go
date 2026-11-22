package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/KOHANTIC/SentryAtlas/backend/internal/event"
	"github.com/KOHANTIC/SentryAtlas/backend/internal/models"
)

const noaaBaseURL = "https://api.weather.gov/alerts/active"

// noaaTypeRules classify an NWS product by the first keyword its name
// contains, in this order, so the more specific name always wins; a name
// with none is "weather". Fire-weather products describe conditions, not a
// fire, and a hurricane-force wind warning is a marine product, not a
// hurricane.
var noaaTypeRules = []struct{ keyword, eventType string }{
	{"tsunami", "tsunami"},
	{"tornado", "tornado"},
	{"hurricane force wind", "weather"},
	{"hurricane", "hurricane"},
	{"typhoon", "hurricane"},
	{"tropical storm", "storm"},
	{"severe thunderstorm", "storm"},
	{"thunderstorm", "storm"},
	{"severe storm", "storm"},
	{"blizzard", "winter_storm"},
	{"ice storm", "winter_storm"},
	{"winter storm", "winter_storm"},
	{"flash flood", "flood"},
	{"flood", "flood"},
	{"red flag", "weather"},
	{"fire weather", "weather"},
	{"fire", "wildfire"},
	{"earthquake", "earthquake"},
	{"volcano", "volcano"},
}

type NOAAAdapter struct {
	client    *http.Client
	userAgent string
	baseURL   string
	maxBody   int64
}

func NewNOAAAdapter(client *http.Client, userAgent string) *NOAAAdapter {
	return &NOAAAdapter{client: client, userAgent: userAgent, baseURL: noaaBaseURL, maxBody: maxResponseBytes}
}

func (a *NOAAAdapter) Source() string {
	return "noaa"
}

func (a *NOAAAdapter) SupportedTypes() []string {
	// Everything classifyNOAAEvent can emit, including the "weather"
	// fallback for alerts with no more specific class.
	return []string{"flood", "storm", "tornado", "hurricane", "winter_storm", "tsunami", "wildfire", "earthquake", "volcano", "weather"}
}

// FetchEvents is FetchV2 projected onto the v1 model, which the service uses
// until it moves to v2 (#21).
func (a *NOAAAdapter) FetchEvents(ctx context.Context) ([]models.Event, error) {
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

// FetchV2 fetches every active alert and update, the window, and maps each
// alert onto v2: one event however many messages carry it. An update
// replaces the message it updates, so asking for alerts alone used to miss
// every alert that had been updated.
func (a *NOAAAdapter) FetchV2(ctx context.Context) ([]event.Event, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.baseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("noaa: build request: %w", err)
	}
	req.Header.Set("User-Agent", a.userAgent)
	req.Header.Set("Accept", "application/geo+json")
	q := req.URL.Query()
	q.Set("status", "actual")
	req.URL.RawQuery = q.Encode()

	features, err := fetchList[noaaFeature](a.client, req, "noaa", "features", a.maxBody)
	if err != nil {
		return nil, err
	}

	var order []string
	groups := map[string][]noaaFeature{}
	for _, f := range features {
		key := noaaGroupKey(f.Properties)
		if _, seen := groups[key]; !seen {
			order = append(order, key)
		}
		groups[key] = append(groups[key], f)
	}

	events := make([]event.Event, 0, len(order))
	for _, key := range order {
		if e, ok := noaaEvent(groups[key]); ok {
			events = append(events, e)
		}
	}
	return events, nil
}

// noaaGroupKey names the alert a message belongs to. Messages that carry one
// alert share its VTEC event (office, phenomenon, significance and event
// number): its updates, and the parallel messages that cover different zones
// of it. A message without VTEC is its own first message's alert.
func noaaGroupKey(p noaaProperties) string {
	for _, v := range p.Parameters.VTEC {
		if k := vtecEventKey(v); k != "" {
			return "vtec:" + k
		}
	}
	return "root:" + noaaRoot(p)
}

// vtecEventKey reads office, phenomenon, significance and event number from
// a P-VTEC string such as /O.CON.KDLH.SC.Y.0140.261122T2200Z-261123T0400Z/,
// giving KDLH.SC.Y.0140; "" if it is not one.
func vtecEventKey(vtec string) string {
	parts := strings.Split(strings.Trim(vtec, "/"), ".")
	if len(parts) < 7 {
		return ""
	}
	return strings.Join(parts[2:6], ".")
}

// noaaRoot is the identifier of the first message in a message's chain: its
// earliest reference, or itself when it references none.
func noaaRoot(p noaaProperties) string {
	root := p.ID
	var first time.Time
	for _, r := range p.References {
		// Compared as times: the strings carry different UTC offsets.
		if t := noaaTime(r.Sent); first.IsZero() || t.Before(first) {
			root, first = r.Identifier, t
		}
	}
	return root
}

var noaaSeverityRank = map[string]int{
	event.SeverityExtreme: 4, event.SeveritySevere: 3, event.SeverityModerate: 2, event.SeverityMinor: 1,
}

// noaaEvent merges the messages of one alert into one event, false when the
// alert has been cancelled. Its id is the first message of the chain, so it
// does not change when the alert is updated.
func noaaEvent(messages []noaaFeature) (event.Event, bool) {
	sort.SliceStable(messages, func(i, j int) bool {
		return noaaTime(messages[i].Properties.Sent).Before(noaaTime(messages[j].Properties.Sent))
	})
	latest := messages[len(messages)-1].Properties
	if latest.MessageType == "Cancel" {
		return event.Event{}, false
	}

	id := ""
	var started, updated time.Time
	var ends, expires *time.Time
	var severity *event.Severity
	zones := map[string]bool{}
	var areas []string
	var polygons [][][][]float64
	for _, m := range messages {
		p := m.Properties
		if p.MessageType == "Cancel" {
			continue
		}
		if r := noaaRoot(p); id == "" || r < id {
			id = r
		}
		start := noaaTime(p.Onset)
		if start.IsZero() {
			start = noaaTime(p.Effective)
		}
		if start.IsZero() {
			start = noaaTime(p.Sent)
		}
		if !start.IsZero() && (started.IsZero() || start.Before(started)) {
			started = start
		}
		if sent := noaaTime(p.Sent); sent.After(updated) {
			updated = sent
		}
		ends = laterTime(ends, noaaTime(p.Ends))
		expires = laterTime(expires, noaaTime(p.Expires))
		level := strings.ToLower(p.Severity)
		if noaaSeverityRank[level] > 0 && (severity == nil || noaaSeverityRank[level] > noaaSeverityRank[severity.Level]) {
			severity = &event.Severity{Level: level, SourceLevel: p.Severity}
		}
		for _, z := range p.AffectedZones {
			zones[z[strings.LastIndex(z, "/")+1:]] = true
		}
		if p.AreaDesc != "" && !containsString(areas, p.AreaDesc) {
			areas = append(areas, p.AreaDesc)
		}
		if m.Geometry != nil {
			polygons = append(polygons, m.Geometry.polygons()...)
		}
	}
	if updated.IsZero() {
		updated = started
	}

	title := latest.Headline
	if title == "" {
		title = latest.Event
	}
	e := event.Event{
		ID:          "noaa-" + id,
		Source:      event.SourceNOAA,
		Type:        classifyNOAAEvent(latest.Event),
		Title:       title,
		Description: latest.Description,
		Severity:    severity,
		Time:        event.Time{Started: started, Updated: updated, Ends: ends, Expires: expires},
		URL:         latest.Web,
		Details: event.Details{NOAA: &event.NWS{
			Event:       latest.Event,
			Urgency:     latest.Urgency,
			Certainty:   latest.Certainty,
			Response:    latest.Response,
			MessageType: latest.MessageType,
			Sender:      latest.SenderName,
			Area:        strings.Join(areas, "; "),
			Instruction: latest.Instruction,
		}},
	}
	if loc := noaaLocation(polygons, zones); loc != nil {
		e.Location = loc
	}
	return e, true
}

// noaaLocation places an alert by its polygons, when it has any, and its
// zones. A shape over event.MaxVertices points is left out; its center stays.
func noaaLocation(polygons [][][][]float64, zones map[string]bool) *event.Location {
	loc := &event.Location{}
	for z := range zones {
		loc.Zones = append(loc.Zones, z)
	}
	sort.Strings(loc.Zones)

	if center := areaCentroid(polygons); center != nil {
		c := event.Point{event.RoundCoordinate(center[0]), event.RoundCoordinate(center[1])}
		loc.Center = &c
		if g := roundedGeometry(polygons); g != nil {
			loc.Geometry = g
		}
	}
	if loc.Center == nil && len(loc.Zones) == 0 {
		return nil
	}
	return loc
}

// roundedGeometry is the polygons as one Polygon or MultiPolygon, rounded
// to event.CoordinateDecimals; nil over event.MaxVertices points.
func roundedGeometry(polygons [][][][]float64) *event.Geometry {
	points := 0
	rounded := make([][][]event.Point, 0, len(polygons))
	for _, poly := range polygons {
		var rings [][]event.Point
		for _, ring := range poly {
			var r []event.Point
			for _, pt := range ring {
				if len(pt) < 2 {
					continue
				}
				r = append(r, event.Point{event.RoundCoordinate(pt[0]), event.RoundCoordinate(pt[1])})
			}
			points += len(r)
			rings = append(rings, r)
		}
		rounded = append(rounded, rings)
	}
	if points == 0 || points > event.MaxVertices {
		return nil
	}
	if len(rounded) == 1 {
		coords, _ := json.Marshal(rounded[0])
		return &event.Geometry{Type: "Polygon", Coordinates: coords}
	}
	coords, _ := json.Marshal(rounded)
	return &event.Geometry{Type: "MultiPolygon", Coordinates: coords}
}

// areaCentroid is the area-weighted center of the polygons' outer rings,
// the point a shape balances on. A ring with no area falls back to the mean
// of its distinct points. Points without both numbers are skipped; nil when
// no point is whole.
func areaCentroid(polygons [][][][]float64) []float64 {
	var sumA, sumX, sumY float64
	var mean []float64
	for _, poly := range polygons {
		if len(poly) == 0 {
			continue
		}
		var ring [][]float64
		for _, pt := range poly[0] {
			if len(pt) >= 2 {
				ring = append(ring, pt)
			}
		}
		if len(ring) == 0 {
			continue
		}
		if mean == nil {
			mean = averagePoint(distinctPoints(ring))
		}
		var a, cx, cy float64
		for i := range ring {
			x0, y0 := ring[i][0], ring[i][1]
			x1, y1 := ring[(i+1)%len(ring)][0], ring[(i+1)%len(ring)][1]
			cross := x0*y1 - x1*y0
			a += cross
			cx += (x0 + x1) * cross
			cy += (y0 + y1) * cross
		}
		if math.Abs(a) < 1e-12 {
			continue
		}
		a /= 2
		sumA += math.Abs(a)
		sumX += math.Abs(a) * cx / (6 * a)
		sumY += math.Abs(a) * cy / (6 * a)
	}
	if sumA > 0 {
		return []float64{sumX / sumA, sumY / sumA}
	}
	return mean
}

// distinctPoints drops a closing point that repeats the first.
func distinctPoints(ring [][]float64) [][]float64 {
	if n := len(ring); n > 1 && ring[0][0] == ring[n-1][0] && ring[0][1] == ring[n-1][1] {
		return ring[:n-1]
	}
	return ring
}

// averagePoint is the mean of the ring's points, skipping any point without
// both a longitude and a latitude, which would otherwise index out of range.
// It is nil when no point is whole.
func averagePoint(ring [][]float64) []float64 {
	var sumLon, sumLat float64
	n := 0
	for _, pt := range ring {
		if len(pt) < 2 {
			continue
		}
		sumLon += pt[0]
		sumLat += pt[1]
		n++
	}
	if n == 0 {
		return nil
	}
	return []float64{sumLon / float64(n), sumLat / float64(n)}
}

// noaaTime parses an NWS time into UTC; zero when absent or malformed.
func noaaTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t.UTC()
}

// laterTime is the later of cur and t, ignoring a zero t.
func laterTime(cur *time.Time, t time.Time) *time.Time {
	if t.IsZero() || (cur != nil && !t.After(*cur)) {
		return cur
	}
	return &t
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func classifyNOAAEvent(name string) string {
	lower := strings.ToLower(name)
	for _, r := range noaaTypeRules {
		if strings.Contains(lower, r.keyword) {
			return r.eventType
		}
	}
	return "weather"
}

// NOAA GeoJSON-LD response types

type noaaFeature struct {
	Geometry   *noaaGeometry  `json:"geometry"`
	Properties noaaProperties `json:"properties"`
}

type noaaGeometry struct {
	Type        string          `json:"type"`
	Coordinates json.RawMessage `json:"coordinates"`
}

// polygons reads the geometry as a list of polygons: one for a Polygon,
// several for a MultiPolygon, none for anything else.
func (g *noaaGeometry) polygons() [][][][]float64 {
	switch g.Type {
	case "Polygon":
		var rings [][][]float64
		if err := json.Unmarshal(g.Coordinates, &rings); err == nil && len(rings) > 0 {
			return [][][][]float64{rings}
		}
	case "MultiPolygon":
		var polys [][][][]float64
		if err := json.Unmarshal(g.Coordinates, &polys); err == nil {
			return polys
		}
	}
	return nil
}

// centroid is the point that places the geometry: a Point as it is, a
// Polygon or MultiPolygon at its area-weighted center.
func (g *noaaGeometry) centroid() []float64 {
	if g.Type == "Point" {
		var pt []float64
		if err := json.Unmarshal(g.Coordinates, &pt); err == nil && len(pt) >= 2 {
			return pt[:2]
		}
		return nil
	}
	return areaCentroid(g.polygons())
}

type noaaProperties struct {
	ID            string          `json:"id"`
	Event         string          `json:"event"`
	Headline      string          `json:"headline"`
	Description   string          `json:"description"`
	Instruction   string          `json:"instruction"`
	Severity      string          `json:"severity"`
	Urgency       string          `json:"urgency"`
	Certainty     string          `json:"certainty"`
	Response      string          `json:"response"`
	MessageType   string          `json:"messageType"`
	Onset         string          `json:"onset"`
	Effective     string          `json:"effective"`
	Sent          string          `json:"sent"`
	Ends          string          `json:"ends"`
	Expires       string          `json:"expires"`
	AreaDesc      string          `json:"areaDesc"`
	SenderName    string          `json:"senderName"`
	Web           string          `json:"web"`
	AffectedZones []string        `json:"affectedZones"`
	References    []noaaReference `json:"references"`
	Parameters    noaaParameters  `json:"parameters"`
}

type noaaReference struct {
	Identifier string `json:"identifier"`
	Sent       string `json:"sent"`
}

type noaaParameters struct {
	VTEC []string `json:"VTEC"`
}
