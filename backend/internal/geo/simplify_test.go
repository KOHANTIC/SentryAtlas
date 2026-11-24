package geo

import "testing"

func TestSimplifyKeepsShapeAndEnds(t *testing.T) {
	t.Parallel()
	line := [][2]float64{{0, 0}, {1, 0.001}, {2, -0.001}, {3, 0}, {3, 3}}
	got := Simplify(line, 0.01)
	want := [][2]float64{{0, 0}, {3, 0}, {3, 3}}
	if len(got) != len(want) {
		t.Fatalf("Simplify = %v, want %v: the near-straight points go, the corner stays", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("point %d = %v, want %v", i, got[i], want[i])
		}
	}
}
