// Package geo holds the plane geometry the adapters and the zone builder
// share, in degrees of longitude and latitude.
package geo

import "math"

// Simplify drops the points of a line that lie within tol of the line
// through the points kept around them (Douglas-Peucker); the ends stay.
func Simplify(pts [][2]float64, tol float64) [][2]float64 {
	if len(pts) < 3 {
		return pts
	}
	keep := make([]bool, len(pts))
	keep[0], keep[len(pts)-1] = true, true
	type span struct{ a, b int }
	stack := []span{{0, len(pts) - 1}}
	for len(stack) > 0 {
		s := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		far, dist := -1, tol
		for i := s.a + 1; i < s.b; i++ {
			if d := distance(pts[i], pts[s.a], pts[s.b]); d > dist {
				far, dist = i, d
			}
		}
		if far >= 0 {
			keep[far] = true
			stack = append(stack, span{s.a, far}, span{far, s.b})
		}
	}
	out := make([][2]float64, 0, len(pts))
	for i, k := range keep {
		if k {
			out = append(out, pts[i])
		}
	}
	return out
}

// distance from p to the segment a-b, in degrees.
func distance(p, a, b [2]float64) float64 {
	dx, dy := b[0]-a[0], b[1]-a[1]
	if dx == 0 && dy == 0 {
		return math.Hypot(p[0]-a[0], p[1]-a[1])
	}
	t := ((p[0]-a[0])*dx + (p[1]-a[1])*dy) / (dx*dx + dy*dy)
	t = math.Max(0, math.Min(1, t))
	return math.Hypot(p[0]-(a[0]+t*dx), p[1]-(a[1]+t*dy))
}
