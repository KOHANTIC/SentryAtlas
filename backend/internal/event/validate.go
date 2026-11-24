package event

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/KOHANTIC/SentryAtlas/backend/internal/models"
)

// CoordinateDecimals is how many decimals a coordinate keeps: about a meter.
const CoordinateDecimals = 5

// MaxVertices caps the points in one geometry. An adapter simplifies a
// larger shape, or leaves it out and keeps the center.
const MaxVertices = 1000

// RoundCoordinate rounds a coordinate to CoordinateDecimals.
func RoundCoordinate(v float64) float64 {
	const scale = 1e5
	return math.Round(v*scale) / scale
}

var sources = map[string]bool{SourceUSGS: true, SourceEONET: true, SourceNOAA: true, SourceGDACS: true, SourceTsunami: true}

var severityLevels = map[string]bool{SeverityExtreme: true, SeveritySevere: true, SeverityModerate: true, SeverityMinor: true}

// Validate reports the first way e breaks the v2 model's rules, or nil.
func (e *Event) Validate() error {
	if !sources[e.Source] {
		return fmt.Errorf("source %q is not one of usgs, eonet, noaa, gdacs, tsunami", e.Source)
	}
	if !strings.HasPrefix(e.ID, e.Source+"-") || len(e.ID) == len(e.Source)+1 {
		return fmt.Errorf("id %q is not %q followed by the upstream id", e.ID, e.Source+"-")
	}
	if !models.IsValidEventType(e.Type) {
		return fmt.Errorf("type %q is not a known event type", e.Type)
	}
	if e.Title == "" {
		return fmt.Errorf("title is empty")
	}
	if e.Severity != nil && !severityLevels[e.Severity.Level] {
		return fmt.Errorf("severity level %q is not on the scale", e.Severity.Level)
	}
	if e.Magnitude != nil && e.Magnitude.Unit == "" {
		return fmt.Errorf("magnitude %v has no unit", e.Magnitude.Value)
	}
	if err := e.Time.validate(); err != nil {
		return err
	}
	if e.Location != nil {
		if err := e.Location.validate(); err != nil {
			return err
		}
	}
	return e.Details.validate(e.Source)
}

func (t Time) validate() error {
	if t.Started.IsZero() {
		return fmt.Errorf("time.started is not set")
	}
	for name, v := range map[string]*time.Time{"started": &t.Started, "updated": &t.Updated, "ends": t.Ends, "expires": t.Expires} {
		if v != nil && !v.IsZero() && v.Location() != time.UTC {
			return fmt.Errorf("time.%s %s is not in UTC", name, v.Format(time.RFC3339))
		}
	}
	return nil
}

func (l *Location) validate() error {
	if l.Center == nil {
		if l.Geometry != nil {
			return fmt.Errorf("location has a geometry but no center")
		}
		if len(l.Zones) == 0 {
			return fmt.Errorf("location is empty: leave it out instead")
		}
		return nil
	}
	if err := checkPoint(*l.Center); err != nil {
		return fmt.Errorf("location.center: %w", err)
	}
	if l.Geometry != nil {
		if err := l.Geometry.validate(); err != nil {
			return fmt.Errorf("location.geometry: %w", err)
		}
	}
	return nil
}

// checkPoint requires a point on the globe, rounded to CoordinateDecimals.
func checkPoint(p Point) error {
	lon, lat := p[0], p[1]
	if math.IsNaN(lon) || math.IsNaN(lat) || lon < -180 || lon > 180 || lat < -90 || lat > 90 {
		return fmt.Errorf("[%v, %v] is not a longitude and latitude", lon, lat)
	}
	if RoundCoordinate(lon) != lon || RoundCoordinate(lat) != lat {
		return fmt.Errorf("[%v, %v] has more than %d decimals", lon, lat, CoordinateDecimals)
	}
	return nil
}

func (g *Geometry) validate() error {
	var points []Point
	var err error
	switch g.Type {
	case "Point":
		var p Point
		err = json.Unmarshal(g.Coordinates, &p)
		points = []Point{p}
	case "LineString":
		err = json.Unmarshal(g.Coordinates, &points)
	case "Polygon":
		var rings [][]Point
		err = json.Unmarshal(g.Coordinates, &rings)
		for _, r := range rings {
			points = append(points, r...)
		}
	case "MultiPolygon":
		var polys [][][]Point
		err = json.Unmarshal(g.Coordinates, &polys)
		for _, poly := range polys {
			for _, r := range poly {
				points = append(points, r...)
			}
		}
	default:
		return fmt.Errorf("type %q is not Point, LineString, Polygon or MultiPolygon", g.Type)
	}
	if err != nil {
		return fmt.Errorf("%s coordinates do not parse: %w", g.Type, err)
	}
	if len(points) == 0 {
		return fmt.Errorf("%s has no points", g.Type)
	}
	if len(points) > MaxVertices {
		return fmt.Errorf("%s has %d points, more than %d", g.Type, len(points), MaxVertices)
	}
	for _, p := range points {
		if err := checkPoint(p); err != nil {
			return err
		}
	}
	return nil
}

func (d Details) validate(source string) error {
	set := map[string]bool{
		SourceUSGS:    d.USGS != nil,
		SourceEONET:   d.EONET != nil,
		SourceNOAA:    d.NOAA != nil,
		SourceGDACS:   d.GDACS != nil,
		SourceTsunami: d.Tsunami != nil,
	}
	for name, present := range set {
		if present != (name == source) {
			return fmt.Errorf("details must hold exactly the %q part", source)
		}
	}
	return nil
}
