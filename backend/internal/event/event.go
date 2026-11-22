// Package event is the v2 event model: what /api/v2 serves and what each
// source's adapter produces. The v1 model in package models becomes a
// projection of it. docs/api/v2/ describes it for API users, with a JSON
// Schema and an example snapshot that the tests hold to these types.
package event

import (
	"encoding/json"
	"time"
)

// Sources are the source ids, as they appear in ids and in Details.
const (
	SourceUSGS  = "usgs"
	SourceEONET = "eonet"
	SourceNOAA  = "noaa"
	SourceGDACS = "gdacs"
)

// Severity levels, most severe first. Each source's own level maps onto
// this one scale; Severity.SourceLevel keeps the source's word.
const (
	SeverityExtreme  = "extreme"
	SeveritySevere   = "severe"
	SeverityModerate = "moderate"
	SeverityMinor    = "minor"
)

// Event is one event as v2 describes it.
type Event struct {
	// ID is "<source>-<upstream id>" and stays the same when the source
	// updates the event, so a link or an alert can follow it.
	ID string `json:"id"`
	// GroupID is shared by the reports of one real event from different
	// sources. It is worked out again on every fetch, so it is never a key.
	GroupID string `json:"group_id,omitempty"`
	// Primary marks the one member of a group to draw; the others are the
	// same event reported by another source.
	Primary bool   `json:"primary,omitempty"`
	Source  string `json:"source"`
	// Type is one of the event types in models.EventTypes.
	Type  string `json:"type"`
	Title string `json:"title"`
	// Description is the source's own text. Long, so the snapshot leaves it
	// out and only the event-detail endpoint carries it.
	Description string     `json:"description,omitempty"`
	Severity    *Severity  `json:"severity,omitempty"`
	Magnitude   *Magnitude `json:"magnitude,omitempty"`
	Time        Time       `json:"time"`
	// Location is nil when the source gives nothing to place the event by.
	Location *Location `json:"location,omitempty"`
	// URL is the source's own page for the event.
	URL     string  `json:"url,omitempty"`
	Details Details `json:"details"`
}

// Severity is a level on the shared scale, with the source's own word.
type Severity struct {
	Level       string `json:"level"`
	SourceLevel string `json:"source_level"`
}

// Magnitude is a value in the unit the source gives it: a magnitude type
// such as "mww" or "ml" for earthquakes, "kts" for wind, "acres" for a fire.
type Magnitude struct {
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
}

// Time holds an event's times, each with one meaning whatever the source,
// all in UTC.
type Time struct {
	// Started is when the event began: an earthquake's origin, a fire's
	// first observation, an alert's onset.
	Started time.Time `json:"started"`
	// Updated is when the source last changed its record.
	Updated time.Time `json:"updated"`
	// Ends is when the source expects the event to be over.
	Ends *time.Time `json:"ends,omitempty"`
	// Expires is when the record stops being valid, as an alert does.
	Expires *time.Time `json:"expires,omitempty"`
}

// Location places an event: a center point, the full shape when the source
// gives one, and the zones of an alert issued for named zones.
type Location struct {
	// Center is set for every event that can be placed, including one
	// placed only by its zones.
	Center   *Point    `json:"center,omitempty"`
	Geometry *Geometry `json:"geometry,omitempty"`
	// Zones are NWS zone ids, such as "LMZ844".
	Zones []string `json:"zones,omitempty"`
}

// Point is [longitude, latitude].
type Point [2]float64

// Geometry is a GeoJSON geometry: Point, LineString, Polygon or MultiPolygon.
type Geometry struct {
	Type        string          `json:"type"`
	Coordinates json.RawMessage `json:"coordinates"`
}

// Details holds what only one source says about an event. Exactly the field
// named by Event.Source is set.
type Details struct {
	USGS  *USGS  `json:"usgs,omitempty"`
	EONET *EONET `json:"eonet,omitempty"`
	NOAA  *NWS   `json:"noaa,omitempty"`
	GDACS *GDACS `json:"gdacs,omitempty"`
}

// USGS is what the USGS feed says beyond the shared fields.
type USGS struct {
	// Kind is USGS's event type: "earthquake", "quarry blast", "explosion",
	// "ice quake" and others. Only earthquakes have Type "earthquake".
	Kind string `json:"kind"`
	// Status is "automatic" or "reviewed".
	Status string `json:"status"`
	// Place is USGS's description of where, such as "28 km NNE of Chenega, Alaska".
	Place   string   `json:"place,omitempty"`
	DepthKm *float64 `json:"depth_km,omitempty"`
	Tsunami bool     `json:"tsunami"`
	// Felt is the number of "Did You Feel It?" reports.
	Felt *int `json:"felt,omitempty"`
	// CDI and MMI are the reported and the instrumental intensity.
	CDI          *float64 `json:"cdi,omitempty"`
	MMI          *float64 `json:"mmi,omitempty"`
	Significance int      `json:"significance"`
	// Alert is the PAGER alert level, when USGS issued one.
	Alert string `json:"alert,omitempty"`
	// DetailURL is USGS's detail feed, which lists ShakeMap, PAGER and the
	// other products for the event.
	DetailURL string `json:"detail_url,omitempty"`
}

// EONET is what NASA EONET says beyond the shared fields.
type EONET struct {
	Categories []string   `json:"categories"`
	Closed     *time.Time `json:"closed,omitempty"`
	// Sources are the agencies EONET took the event from, with their pages.
	Sources []Link `json:"sources,omitempty"`
}

// NWS is what a National Weather Service alert says beyond the shared fields.
type NWS struct {
	// Event is the product, such as "Small Craft Advisory".
	Event       string `json:"event"`
	Urgency     string `json:"urgency"`
	Certainty   string `json:"certainty"`
	Response    string `json:"response,omitempty"`
	MessageType string `json:"message_type"`
	Sender      string `json:"sender"`
	// Area names the places the alert covers.
	Area string `json:"area"`
	// Instruction is the alert's safety text, word for word. Long, so only
	// the event-detail endpoint carries it.
	Instruction string `json:"instruction,omitempty"`
}

// GDACS is what GDACS says beyond the shared fields.
type GDACS struct {
	AlertLevel   string   `json:"alert_level"`
	AlertScore   *float64 `json:"alert_score,omitempty"`
	EpisodeID    int      `json:"episode_id"`
	Countries    []string `json:"countries,omitempty"`
	Current      bool     `json:"current"`
	SeverityText string   `json:"severity_text,omitempty"`
	// FootprintURL is GDACS's polygon endpoint for the event's footprint.
	FootprintURL string `json:"footprint_url,omitempty"`
}

// Link is a source's id for an event and its page.
type Link struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

// Snapshot is the whole set of events for one window, as one response.
type Snapshot struct {
	Generated time.Time `json:"generated"`
	// Window is "7d" or "30d".
	Window  string         `json:"window"`
	Sources []SourceStatus `json:"sources"`
	Events  []Event        `json:"events"`
}

// SourceStatus is how fresh one source's part of a snapshot is.
type SourceStatus struct {
	Source string `json:"source"`
	OK     bool   `json:"ok"`
	// FetchedAt is when the source last answered; nil if it never has.
	FetchedAt *time.Time `json:"fetched_at,omitempty"`
	// Stale is true when the last fetch failed and an older copy is served.
	Stale bool `json:"stale"`
	// Error is a short reason, set when the last fetch failed.
	Error string `json:"error,omitempty"`
	Count int    `json:"count"`
}
