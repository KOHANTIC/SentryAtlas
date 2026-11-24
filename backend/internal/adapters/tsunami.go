package adapters

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/KOHANTIC/SentryAtlas/backend/internal/event"
)

// tsunamiFeeds are the two NWS tsunami warning centers' Atom feeds. Each
// holds the center's latest bulletin and links its CAP message.
var tsunamiFeeds = []string{
	"https://www.tsunami.gov/events/xml/PAAQAtom.xml", // NTWC, Palmer, Alaska
	"https://www.tsunami.gov/events/xml/PHEBAtom.xml", // PTWC, Honolulu
}

// tsunamiMaxBody caps one feed or CAP message; each is a few kilobytes.
const tsunamiMaxBody = 1 << 20

// tsunamiThreats are the products that warn of a tsunami. An information
// statement says none is expected, so it never becomes an event: a tsunami
// mark on the map always means a threat.
var tsunamiThreats = []string{"warning", "advisory", "watch", "threat"}

// TsunamiAdapter reads the bulletins of the NWS tsunami warning centers.
// It is a v2 source only: v1 keeps the four sources it always had.
type TsunamiAdapter struct {
	client    *http.Client
	userAgent string
	feeds     []string
	now       func() time.Time
}

func NewTsunamiAdapter(client *http.Client, userAgent string) *TsunamiAdapter {
	return &TsunamiAdapter{client: client, userAgent: userAgent, feeds: tsunamiFeeds, now: time.Now}
}

func (a *TsunamiAdapter) Source() string { return event.SourceTsunami }

// FetchV2 reads each center's feed and the CAP message of each bulletin in
// it, keeping the bulletins that warn of a tsunami. Either center failing
// fails the fetch, so the last good copy of both is kept rather than half.
func (a *TsunamiAdapter) FetchV2(ctx context.Context) ([]event.Event, error) {
	events := []event.Event{}
	for _, feed := range a.feeds {
		var f atomFeed
		if err := a.getXML(ctx, feed, &f); err != nil {
			return nil, err
		}
		for _, link := range f.capLinks() {
			var c capAlert
			if err := a.getXML(ctx, link, &c); err != nil {
				return nil, err
			}
			if e, ok := tsunamiEvent(c, a.now()); ok {
				events = append(events, e)
			}
		}
	}
	return events, nil
}

// getXML fetches url and decodes it into v, reading at most tsunamiMaxBody bytes.
func (a *TsunamiAdapter) getXML(ctx context.Context, url string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("tsunami: build request: %w", err)
	}
	req.Header.Set("User-Agent", a.userAgent)
	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("tsunami: request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("tsunami: %w", &StatusError{Code: resp.StatusCode})
	}
	if err := xml.NewDecoder(io.LimitReader(resp.Body, tsunamiMaxBody)).Decode(v); err != nil {
		return fmt.Errorf("tsunami: decode %s: %w: %w", url, ErrUnreadable, err)
	}
	return nil
}

type atomFeed struct {
	Entries []struct {
		Links []struct {
			Rel  string `xml:"rel,attr"`
			Type string `xml:"type,attr"`
			Href string `xml:"href,attr"`
		} `xml:"link"`
	} `xml:"entry"`
}

// capLinks are the CAP messages the feed's entries link.
func (f atomFeed) capLinks() []string {
	var links []string
	for _, e := range f.Entries {
		for _, l := range e.Links {
			if l.Type == "application/cap+xml" && l.Href != "" {
				links = append(links, l.Href)
			}
		}
	}
	return links
}

// capAlert is the part of a CAP 1.2 message a bulletin needs.
type capAlert struct {
	Identifier string    `xml:"identifier"`
	Sent       string    `xml:"sent"`
	Status     string    `xml:"status"`
	MsgType    string    `xml:"msgType"`
	Source     string    `xml:"source"`
	Incidents  string    `xml:"incidents"`
	Info       []capInfo `xml:"info"`
}

type capInfo struct {
	Event        string `xml:"event"`
	ResponseType string `xml:"responseType"`
	Urgency      string `xml:"urgency"`
	Severity     string `xml:"severity"`
	Certainty    string `xml:"certainty"`
	Onset        string `xml:"onset"`
	Expires      string `xml:"expires"`
	Description  string `xml:"description"`
	Instruction  string `xml:"instruction"`
	Web          string `xml:"web"`
	Parameters   []struct {
		Name  string `xml:"valueName"`
		Value string `xml:"value"`
	} `xml:"parameter"`
	Areas []struct {
		Desc string `xml:"areaDesc"`
	} `xml:"area"`
}

func (i capInfo) param(name string) string {
	for _, p := range i.Parameters {
		if p.Name == name {
			return strings.TrimSpace(p.Value)
		}
	}
	return ""
}

// tsunamiEvent maps a bulletin onto v2, or reports false for one that is
// not a live threat: a test or exercise, a cancellation, an information
// statement, or one past its expiry.
func tsunamiEvent(c capAlert, now time.Time) (event.Event, bool) {
	if !strings.EqualFold(c.Status, "Actual") || strings.EqualFold(c.MsgType, "Cancel") || len(c.Info) == 0 {
		return event.Event{}, false
	}
	info := c.Info[0]
	if !isTsunamiThreat(info.Event) {
		return event.Event{}, false
	}
	expires, hasExpiry := capTime(info.Expires)
	if hasExpiry && !expires.After(now) {
		return event.Event{}, false
	}
	sent, _ := capTime(c.Sent)
	origin, hasOrigin := capTime(info.param("EventOriginTime"))
	started := sent
	if onset, ok := capTime(info.Onset); ok {
		started = onset
	}
	if hasOrigin {
		started = origin
	}

	area := make([]string, 0, len(info.Areas))
	for _, a := range info.Areas {
		if d := strings.TrimSpace(a.Desc); d != "" {
			area = append(area, d)
		}
	}
	place := info.param("EventLocationName")
	title := strings.TrimSpace(info.Event)
	if place != "" {
		title += ": " + place
	}
	instruction := strings.TrimSpace(info.Instruction)
	// The centers write N/A where a bulletin has no instructions.
	if instruction == "N/A" {
		instruction = ""
	}

	details := &event.Tsunami{
		Center:      strings.TrimSpace(c.Source),
		Event:       strings.TrimSpace(info.Event),
		Urgency:     info.Urgency,
		Certainty:   info.Certainty,
		Response:    info.ResponseType,
		MessageType: c.MsgType,
		Area:        strings.Join(area, "; "),
		Instruction: instruction,
		Quake: event.Quake{
			Magnitude:     leadingNumber(info.param("EventPreliminaryMagnitude")),
			MagnitudeType: info.param("EventPreliminaryMagnitudeType"),
			DepthKm:       leadingNumber(info.param("EventDepth")),
		},
	}
	if hasOrigin {
		details.Quake.Origin = &origin
	}

	e := event.Event{
		ID:          tsunamiID(c),
		Source:      event.SourceTsunami,
		Type:        "tsunami",
		Title:       title,
		Description: strings.TrimSpace(info.Description),
		Time:        event.Time{Started: started, Updated: sent},
		URL:         strings.Replace(strings.TrimSpace(info.Web), "http://www.tsunami.gov/", "https://www.tsunami.gov/", 1),
		Details:     event.Details{Tsunami: details},
	}
	if hasExpiry {
		e.Time.Expires = &expires
	}
	if level := strings.ToLower(info.Severity); noaaSeverityRank[level] > 0 {
		e.Severity = &event.Severity{Level: level, SourceLevel: details.Event}
	}
	if lon, lat, ok := capLatLon(info.param("EventLatLon")); ok {
		center := event.Point{event.RoundCoordinate(lon), event.RoundCoordinate(lat)}
		coords, _ := json.Marshal(center)
		e.Location = &event.Location{Center: &center, Geometry: &event.Geometry{Type: "Point", Coordinates: coords}}
	}
	return e, true
}

func isTsunamiThreat(product string) bool {
	p := strings.ToLower(product)
	if !strings.Contains(p, "tsunami") || strings.Contains(p, "cancel") {
		return false
	}
	for _, t := range tsunamiThreats {
		if strings.Contains(p, t) {
			return true
		}
	}
	return false
}

// tsunamiID is "tsunami-<center code>-<incident>", such as
// tsunami-PHEB-26327000. The identifier is "<code>-<bulletin>-<incident>",
// so the bulletin number is left out and a center's later bulletins on the
// same quake update one event.
func tsunamiID(c capAlert) string {
	code, _, _ := strings.Cut(c.Identifier, "-")
	if incident := strings.TrimSpace(c.Incidents); code != "" && incident != "" {
		return "tsunami-" + code + "-" + incident
	}
	return "tsunami-" + c.Identifier
}

// capTime reads a CAP time such as 2026-11-23T02:49:16-00:00.
func capTime(s string) (time.Time, bool) {
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(s))
	if err != nil {
		return time.Time{}, false
	}
	return t.UTC(), true
}

// capLatLon reads EventLatLon, "-13.546,172.279 0.000": latitude,
// longitude, then a depth the centers leave at zero.
func capLatLon(s string) (lon, lat float64, ok bool) {
	pair, _, _ := strings.Cut(strings.TrimSpace(s), " ")
	la, lo, found := strings.Cut(pair, ",")
	if !found {
		return 0, 0, false
	}
	lat, err1 := strconv.ParseFloat(la, 64)
	lon, err2 := strconv.ParseFloat(lo, 64)
	if err1 != nil || err2 != nil || lat < -90 || lat > 90 || lon < -180 || lon > 180 {
		return 0, 0, false
	}
	return lon, lat, true
}

// leadingNumber reads the number a value starts with, as in "614 kilometers".
func leadingNumber(s string) *float64 {
	first, _, _ := strings.Cut(strings.TrimSpace(s), " ")
	v, err := strconv.ParseFloat(first, 64)
	if err != nil {
		return nil
	}
	return &v
}
