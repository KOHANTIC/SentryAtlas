package zones

import (
	"math"
	"os"
	"testing"
)

func TestCenter(t *testing.T) {
	t.Parallel()
	c, ok := Center([]string{"PZZ131", "PZZ132", "NOSUCH"})
	if !ok || c[0] < -124.5 || c[0] > -122.5 || c[1] < 47.8 || c[1] > 48.8 {
		t.Errorf("Center(PZZ131, PZZ132) = %v %v, want the Strait of Juan de Fuca", c, ok)
	}
	if _, ok := Center([]string{"NOSUCH"}); ok {
		t.Error("an unknown zone was placed")
	}
	if _, ok := Center(nil); ok {
		t.Error("no zones were placed")
	}
}

func TestMeanAcrossTheDateLine(t *testing.T) {
	t.Parallel()
	got := Mean([][2]float64{{179, 10}, {-179, 20}}, nil)
	if math.Abs(math.Abs(got[0])-180) > 1e-9 || got[1] != 15 {
		t.Errorf("Mean = %v, want [±180 15]", got)
	}
	got = Mean([][2]float64{{-179, 0}, {179, 0}, {179, 0}}, []float64{1, 1, 2})
	if math.Abs(got[0]-179.5) > 1e-9 {
		t.Errorf("weighted Mean = %v, want [179.5 0]", got)
	}
	if got := Mean([][2]float64{{1, 1}, {3, 3}}, []float64{0, 0}); got != [2]float64{2, 2} {
		t.Errorf("Mean with no weight = %v, want the plain mean", got)
	}
}

// The files are built by cmd/zones and committed; these bounds keep a
// refresh from quietly growing what the server embeds and the map loads.
func TestTheBuiltFilesFitTheBudget(t *testing.T) {
	t.Parallel()
	if n := len(centersJSON); n > 300<<10 || len(centers()) < 8000 {
		t.Errorf("centers.json: %d bytes, %d zones; want under 300 KB and the 8,000-odd zones", n, len(centers()))
	}
	info, err := os.Stat("../../../frontend/public/data/nws-zones.json")
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() > 6<<20 {
		t.Errorf("nws-zones.json: %d bytes, want under 6 MB", info.Size())
	}
}
