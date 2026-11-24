package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"

	"github.com/KOHANTIC/SentryAtlas/backend/internal/geo"
	"github.com/KOHANTIC/SentryAtlas/backend/internal/zones"
)

const (
	// tolerance is how far, in degrees (about 1 km), a simplified outline
	// may stray from the drawn one.
	tolerance = 0.01
	// decimals keeps coordinates to about 100 m, plenty at that tolerance.
	decimals = 3
)

// zone gathers one id's records: a zone or county split into several.
type zone struct {
	centers [][2]float64
	areas   []float64
	rings   [][][2]float64 // simplified outer rings
	largest [][2]float64   // the largest outer ring as drawn, if every ring simplifies away
	bigArea float64
}

// zoneID names a row by the fields its file has: public zones by state and
// zone number, counties by state and county FIPS, marine and offshore zones
// by their own id. "" for a row without one, such as a high-seas area.
func zoneID(row map[string]string) string {
	switch {
	case row["STATE"] != "" && row["ZONE"] != "":
		return row["STATE"] + "Z" + row["ZONE"]
	case row["STATE"] != "" && len(row["FIPS"]) == 5:
		return row["STATE"] + "C" + row["FIPS"][2:]
	}
	return row["ID"]
}

// add takes one file's rows and shapes, which correspond record by record.
func add(all map[string]*zone, rows []map[string]string, shapes [][][][2]float64) error {
	if len(rows) != len(shapes) {
		return fmt.Errorf("%d rows but %d shapes", len(rows), len(shapes))
	}
	for i, row := range rows {
		id := zoneID(row)
		if id == "" {
			continue
		}
		z := all[id]
		if z == nil {
			z = &zone{}
			all[id] = z
		}
		for _, ring := range shapes[i] {
			// Shapefiles draw outer rings clockwise and holes the other way.
			a := signedArea(ring)
			if a >= 0 {
				continue
			}
			if -a > z.bigArea {
				z.bigArea, z.largest = -a, ring
			}
			if s := rounded(geo.Simplify(ring, tolerance)); len(s) >= 4 {
				z.rings = append(z.rings, s)
			}
			// The files' own LON and LAT are not used: across 180° they are
			// a naive average (Aleutians West at -107.9°). A ring never
			// crosses 180°, so its own center is sound.
			z.centers = append(z.centers, ringCenter(ring, a))
			z.areas = append(z.areas, -a)
		}
	}
	return nil
}

// ringCenter is the area center of a ring of signed area a; the mean of its
// points when it has no area.
func ringCenter(ring [][2]float64, a float64) [2]float64 {
	if a == 0 {
		return zones.Mean(ring, nil)
	}
	var cx, cy float64
	for i := range ring {
		p, q := ring[i], ring[(i+1)%len(ring)]
		cross := p[0]*q[1] - q[0]*p[1]
		cx += (p[0] + q[0]) * cross
		cy += (p[1] + q[1]) * cross
	}
	return [2]float64{cx / (6 * a), cy / (6 * a)}
}

// center is the zone's center: its outer rings' centers weighted by their
// area, across 180°.
func (z *zone) center() ([2]float64, bool) {
	if len(z.centers) == 0 {
		return [2]float64{}, false
	}
	return zones.Mean(z.centers, z.areas), true
}

// outline is the zone's rings, or its largest ring as drawn when all of
// them were too small to survive simplification.
func (z *zone) outline() [][][2]float64 {
	if len(z.rings) == 0 && z.largest != nil {
		return [][][2]float64{rounded(z.largest)}
	}
	return z.rings
}

// signedArea is the shoelace area: negative for a clockwise ring.
func signedArea(ring [][2]float64) float64 {
	a := 0.0
	for i := range ring {
		p, q := ring[i], ring[(i+1)%len(ring)]
		a += p[0]*q[1] - q[0]*p[1]
	}
	return a / 2
}

// rounded rounds a ring to decimals, drops repeated points and keeps it
// closed.
func rounded(ring [][2]float64) [][2]float64 {
	out := make([][2]float64, 0, len(ring))
	for _, p := range ring {
		q := [2]float64{round(p[0]), round(p[1])}
		if len(out) == 0 || out[len(out)-1] != q {
			out = append(out, q)
		}
	}
	if len(out) > 1 && out[0] != out[len(out)-1] {
		out = append(out, out[0])
	}
	return out
}

func round(v float64) float64 {
	p := math.Pow(10, decimals)
	return math.Round(v*p) / p
}

func sortedIDs(all map[string]*zone) []string {
	ids := make([]string, 0, len(all))
	for id := range all {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// centersFile is the id-to-center table, one zone a line so a refresh
// reads as a diff.
func centersFile(all map[string]*zone) []byte {
	var b bytes.Buffer
	b.WriteString("{\n")
	first := true
	for _, id := range sortedIDs(all) {
		c, ok := all[id].center()
		if !ok {
			continue
		}
		if !first {
			b.WriteString(",\n")
		}
		first = false
		fmt.Fprintf(&b, "%q:[%s,%s]", id, num(round(c[0])), num(round(c[1])))
	}
	b.WriteString("\n}\n")
	return b.Bytes()
}

// shapesFile is a GeoJSON FeatureCollection of every zone's outline as a
// MultiPolygon, its id the zone id, one zone a line.
func shapesFile(all map[string]*zone) ([]byte, error) {
	var b bytes.Buffer
	b.WriteString(`{"type":"FeatureCollection","features":[` + "\n")
	first := true
	for _, id := range sortedIDs(all) {
		outline := all[id].outline()
		if len(outline) == 0 {
			continue
		}
		polys := make([][][][2]float64, len(outline))
		for i, ring := range outline {
			polys[i] = [][][2]float64{ring}
		}
		coords, err := json.Marshal(polys)
		if err != nil {
			return nil, err
		}
		if !first {
			b.WriteString(",\n")
		}
		first = false
		fmt.Fprintf(&b, `{"type":"Feature","id":%q,"properties":null,"geometry":{"type":"MultiPolygon","coordinates":%s}}`, id, coords)
	}
	b.WriteString("\n]}\n")
	return b.Bytes(), nil
}

func num(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }
