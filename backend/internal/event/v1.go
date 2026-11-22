package event

import (
	"strings"

	"github.com/KOHANTIC/SentryAtlas/backend/internal/models"
)

// V1 projects a v2 event onto the v1 model, which /api/v1 keeps serving: its
// point is the center, its magnitude loses the unit, its severity is the
// level, and its metadata holds the keys v1 has always carried for its source.
func (e Event) V1() models.Event {
	v := models.Event{
		ID:          e.ID,
		Title:       e.Title,
		Description: e.Description,
		EventType:   e.Type,
		Source:      e.Source,
		StartedAt:   e.Time.Started,
		UpdatedAt:   e.Time.Updated,
		URL:         e.URL,
		Metadata:    e.v1Metadata(),
	}
	if e.Location != nil && e.Location.Center != nil {
		v.Geometry = models.Geometry{Type: "Point", Coordinates: []float64{e.Location.Center[0], e.Location.Center[1]}}
	}
	// v1's magnitude has always been an earthquake magnitude and has no unit,
	// so only USGS fills it; acres or knots there would read as a quake size.
	if e.Magnitude != nil && e.Source == SourceUSGS {
		m := e.Magnitude.Value
		v.Magnitude = &m
	}
	if e.Severity != nil {
		v.Severity = e.Severity.Level
	}
	return v
}

// v1Metadata is the v1 metadata of a source's details: the same keys v1
// carried before v2, so v1 keeps its shape.
func (e Event) v1Metadata() map[string]any {
	d := e.Details
	switch {
	case d.USGS != nil:
		m := map[string]any{"place": d.USGS.Place}
		if d.USGS.DepthKm != nil {
			m["depth"] = *d.USGS.DepthKm
		}
		return m
	case d.NOAA != nil:
		return map[string]any{
			"event":       d.NOAA.Event,
			"area_desc":   d.NOAA.Area,
			"urgency":     d.NOAA.Urgency,
			"certainty":   d.NOAA.Certainty,
			"sender_name": d.NOAA.Sender,
		}
	case d.GDACS != nil:
		m := map[string]any{
			"alert_level":   d.GDACS.AlertLevel,
			"country":       strings.Join(d.GDACS.Countries, ", "),
			"severity_unit": "",
		}
		// GDACS's severity is a physical quantity; v1 keeps it here, never as
		// the magnitude, which is a quake's size.
		if e.Magnitude != nil {
			m["severity_unit"] = e.Magnitude.Unit
			m["severity_value"] = e.Magnitude.Value
		}
		return m
	}
	return nil
}
