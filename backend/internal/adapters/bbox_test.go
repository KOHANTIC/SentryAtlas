package adapters

import (
	"math"
	"testing"
)

func TestNewBBoxPutsTheBoxOnTheGlobe(t *testing.T) {
	t.Parallel()
	type point struct{ lon, lat float64 }
	cases := []struct {
		name            string
		minLon, maxLon  float64
		inside, outside []point
	}{
		{"an ordinary box", 0, 20, []point{{10, 0}}, []point{{30, 0}, {-10, 0}}},
		{"over the Pacific as a map sends it", 170, 190, []point{{175, 0}, {-175, 0}, {180, 0}}, []point{{0, 0}, {160, 0}, {-160, 0}}},
		{"over the Pacific as GeoJSON writes it", 170, -170, []point{{175, 0}, {-175, 0}}, []point{{0, 0}, {160, 0}, {-160, 0}}},
		{"over the Pacific unwrapped the other way", -190, -170, []point{{175, 0}, {-175, 0}}, []point{{0, 0}, {160, 0}, {-160, 0}}},
		{"wholly past 180", 190, 200, []point{{-165, 0}}, []point{{165, 0}, {0, 0}}},
		{"wider than the world", -400, 400, []point{{0, 0}, {179, 0}, {-179, 0}}, nil},
		{"exactly the world", -180, 180, []point{{0, 0}, {180, 0}, {-180, 0}}, nil},
		{"up to 180", 0, 180, []point{{90, 0}, {180, 0}}, []point{{-90, 0}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			b, err := NewBBox(c.minLon, -10, c.maxLon, 10)
			if err != nil {
				t.Fatalf("NewBBox: %v", err)
			}
			for _, p := range c.inside {
				if !b.Contains(p.lon, p.lat) {
					t.Errorf("box %+v does not contain (%v, %v)", *b, p.lon, p.lat)
				}
			}
			for _, p := range c.outside {
				if b.Contains(p.lon, p.lat) {
					t.Errorf("box %+v contains (%v, %v)", *b, p.lon, p.lat)
				}
			}
			if b.Contains(c.inside[0].lon, 20) {
				t.Errorf("box %+v contains a point north of it", *b)
			}
		})
	}
}

func TestNewBBoxRejectsWhatIsNotABox(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name                           string
		minLon, minLat, maxLon, maxLat float64
	}{
		{"NaN", 0, math.NaN(), 1, 1},
		{"infinite longitude", 0, 0, math.Inf(1), 1},
		{"negative infinity", math.Inf(-1), 0, 1, 1},
		{"latitude above 90", 0, 0, 1, 95},
		{"latitude below -90", 0, -91, 1, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if b, err := NewBBox(c.minLon, c.minLat, c.maxLon, c.maxLat); err == nil {
				t.Errorf("NewBBox = %+v, want an error", *b)
			}
		})
	}
}
