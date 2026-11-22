// Package zones places NWS alerts that name zones but carry no shape: it
// holds the center of every public, marine and offshore zone and county,
// built from NWS's shapefiles by cmd/zones (docs/data/nws-zones.md).
package zones

import (
	_ "embed"
	"encoding/json"
	"math"
	"sync"
)

//go:embed centers.json
var centersJSON []byte

var centers = sync.OnceValue(func() map[string][2]float64 {
	m := map[string][2]float64{}
	if err := json.Unmarshal(centersJSON, &m); err != nil {
		// The file is built and tested with the code; a bad one is a build bug.
		panic("zones: centers.json: " + err.Error())
	}
	return m
})

// Center is the mean of the known zones' centers, [lon, lat], and false if
// none of the ids is known.
func Center(ids []string) ([2]float64, bool) {
	var pts [][2]float64
	for _, id := range ids {
		if c, ok := centers()[id]; ok {
			pts = append(pts, c)
		}
	}
	if len(pts) == 0 {
		return [2]float64{}, false
	}
	return Mean(pts, nil), true
}

// Mean is the weighted mean of points, [lon, lat], weights nil meaning
// equal. Longitudes are taken on the side of the first point, so points
// either side of 180° average near it rather than near 0°.
func Mean(pts [][2]float64, weights []float64) [2]float64 {
	var lon, lat, total float64
	ref := pts[0][0]
	for i, p := range pts {
		w := 1.0
		if weights != nil {
			w = weights[i]
		}
		x := p[0]
		for x-ref > 180 {
			x -= 360
		}
		for x-ref < -180 {
			x += 360
		}
		lon += w * x
		lat += w * p[1]
		total += w
	}
	if total == 0 {
		return Mean(pts, nil)
	}
	lon /= total
	// Back into [-180, 180).
	lon = math.Mod(lon+540, 360) - 180
	return [2]float64{lon, lat / total}
}
