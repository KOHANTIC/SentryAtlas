package adapters

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/KOHANTIC/SentryAtlas/backend/internal/event"
	"github.com/KOHANTIC/SentryAtlas/backend/internal/geo"
)

// footprintArea says, per GDACS event type, which feature of its footprint
// is the event's area. Other types keep their point: a fire's area is tiny,
// a quake's ShakeMap repeats USGS's, and a volcano's is only a circle.
var footprintArea = map[string]func(footprintProperties) bool{
	// The cumulative swath of winds of 60 km/h or more, along the track and
	// its forecast; GDACS also draws 90 and 120 km/h swaths inside it.
	"TC": func(p footprintProperties) bool { return p.Class == "Poly_Green" && p.Label == "60 km/h" },
	"FL": func(p footprintProperties) bool { return p.Class == "Poly_Affected" },
	"DR": func(p footprintProperties) bool { return p.Class == "Poly_area" },
}

const (
	// A poll shares its timeout with the event list, so each fetches at most
	// this many new footprints, this many at once, and starts none once
	// footprintReserve is all that is left. A full first fill takes a few polls.
	footprintsPerPoll = 25
	footprintWorkers  = 4
	footprintReserve  = 5 * time.Second
	// footprintRetry is how long a footprint that failed waits before the
	// next try, so a broken one does not cost a request every poll.
	footprintRetry = time.Hour
	// footprintTolerance (about 1 km, as the NWS zones use) is doubled up to
	// footprintMaxTolerance until a shape fits event.MaxVertices.
	footprintTolerance    = 0.01
	footprintMaxTolerance = 0.16
	// footprintDecimals keeps coordinates to about 100 m, plenty at 1 km.
	footprintDecimals = 3
)

// footprint is a cached footprint: the area, or nil when GDACS gave none
// usable, or the time a fetch failed.
type footprint struct {
	area     *event.Geometry
	failedAt time.Time
}

// footprintCache holds each footprint by type, event and episode, so a
// footprint is fetched once per GDACS episode.
type footprintCache struct {
	mu    sync.Mutex
	byKey map[string]footprint
}

type footprintFeature struct {
	Geometry   *geoJSONGeometry    `json:"geometry"`
	Properties footprintProperties `json:"properties"`
}

type footprintProperties struct {
	Class string `json:"Class"`
	Label string `json:"polygonlabel"`
}

// footprintKey is "<type>-<event id>-<episode>", and the GDACS type, for an
// event whose type has an area; false for one without.
func footprintKey(e event.Event) (key, code string, ok bool) {
	d := e.Details.GDACS
	if d == nil || d.FootprintURL == "" {
		return "", "", false
	}
	code, _, _ = strings.Cut(strings.TrimPrefix(e.ID, "gdacs-"), "-")
	if _, has := footprintArea[code]; !has {
		return "", "", false
	}
	return fmt.Sprintf("%s-%d", strings.TrimPrefix(e.ID, "gdacs-"), d.EpisodeID), code, true
}

// addFootprints draws each cyclone, flood and drought with its area: from
// the cache, or fetched within this poll's budget. One without an area yet
// keeps its point, and a footprint never fails the source.
func (a *GDACSAdapter) addFootprints(ctx context.Context, events []event.Event) {
	now := a.now()
	type job struct {
		i         int
		key, code string
	}
	var todo []job
	keep := map[string]bool{}
	a.footprints.mu.Lock()
	if a.footprints.byKey == nil {
		a.footprints.byKey = map[string]footprint{}
	}
	for i := range events {
		key, code, ok := footprintKey(events[i])
		// An event GDACS gives no place for has nowhere to draw an area, and a
		// link to any host but the event list's own is never followed.
		if !ok || events[i].Location == nil || events[i].Location.Center == nil || !sameHost(events[i].Details.GDACS.FootprintURL, a.baseURL) {
			continue
		}
		keep[key] = true
		fp, seen := a.footprints.byKey[key]
		switch {
		case seen && fp.failedAt.IsZero():
			setArea(&events[i], fp.area)
		case seen && now.Sub(fp.failedAt) < footprintRetry:
		default:
			todo = append(todo, job{i, key, code})
		}
	}
	// Footprints of events that have left the window go with them.
	for key := range a.footprints.byKey {
		if !keep[key] {
			delete(a.footprints.byKey, key)
		}
	}
	a.footprints.mu.Unlock()

	// Current events first, then the higher alerts.
	sort.SliceStable(todo, func(x, y int) bool {
		dx, dy := events[todo[x].i].Details.GDACS, events[todo[y].i].Details.GDACS
		if dx.Current != dy.Current {
			return dx.Current
		}
		return score(dx.AlertScore) > score(dy.AlertScore)
	})
	if len(todo) > footprintsPerPoll {
		todo = todo[:footprintsPerPoll]
	}
	if deadline, ok := ctx.Deadline(); ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, deadline.Add(-footprintReserve))
		defer cancel()
	}

	jobs := make(chan job)
	var wg sync.WaitGroup
	for range footprintWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				area, err := a.fetchFootprint(ctx, events[j.i].Details.GDACS.FootprintURL, j.code)
				// A fetch cut short by this poll's budget is tried again next poll.
				if ctx.Err() != nil && errors.Is(err, ctx.Err()) {
					continue
				}
				a.footprints.mu.Lock()
				if err != nil {
					slog.Warn("gdacs: footprint not read; the event keeps its point", "event", events[j.i].ID, "error", err.Error())
					a.footprints.byKey[j.key] = footprint{failedAt: now}
				} else {
					a.footprints.byKey[j.key] = footprint{area: area}
					setArea(&events[j.i], area)
				}
				a.footprints.mu.Unlock()
			}
		}()
	}
	for _, j := range todo {
		if ctx.Err() != nil {
			break
		}
		jobs <- j
	}
	close(jobs)
	wg.Wait()
}

// sameHost reports whether link is on the same scheme and host as base.
func sameHost(link, base string) bool {
	l, err1 := url.Parse(link)
	b, err2 := url.Parse(base)
	return err1 == nil && err2 == nil && l.Scheme == b.Scheme && l.Host == b.Host
}

func score(s *float64) float64 {
	if s == nil {
		return 0
	}
	return *s
}

// setArea draws the event as its area; its center stays where GDACS puts
// the event, so the mark does not move.
func setArea(e *event.Event, area *event.Geometry) {
	if area == nil || e.Location == nil || e.Location.Center == nil {
		return
	}
	e.Location.Geometry = area
}

// fetchFootprint reads a footprint and returns its area for the event type,
// simplified; nil with no error when the footprint has none usable.
func (a *GDACSAdapter) fetchFootprint(ctx context.Context, link, code string) (*event.Geometry, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
	if err != nil {
		return nil, fmt.Errorf("gdacs footprint: build request: %w", err)
	}
	features, err := fetchList[footprintFeature](a.client, req, "gdacs footprint", "features", a.maxBody)
	if err != nil {
		return nil, err
	}
	isArea := footprintArea[code]
	for _, f := range features {
		if f.Geometry == nil || !isArea(f.Properties) {
			continue
		}
		if area := simplifiedArea(f.Geometry.polygons()); area != nil {
			return area, nil
		}
	}
	return nil, nil
}

// simplifiedArea simplifies polygons until they fit event.MaxVertices,
// dropping rings that simplify away; nil if nothing is left.
func simplifiedArea(polygons [][][][]float64) *event.Geometry {
	for tol := footprintTolerance; tol <= footprintMaxTolerance; tol *= 2 {
		var out [][][][]float64
		n := 0
		for _, poly := range polygons {
			var rings [][][]float64
			for i, ring := range poly {
				r := simplifiedRing(ring, tol)
				if len(r) < 4 {
					if i == 0 {
						// The outer ring is gone, and its holes with it.
						rings = nil
						break
					}
					continue
				}
				rings = append(rings, r)
				n += len(r)
			}
			if len(rings) > 0 {
				out = append(out, rings)
			}
		}
		if n == 0 {
			return nil
		}
		if n <= event.MaxVertices {
			return roundedGeometry(out)
		}
	}
	return nil
}

// simplifiedRing simplifies a ring and rounds it to footprintDecimals,
// dropping points that rounding repeats.
func simplifiedRing(ring [][]float64, tol float64) [][]float64 {
	pts := make([][2]float64, 0, len(ring))
	for _, p := range ring {
		if len(p) >= 2 {
			pts = append(pts, [2]float64{p[0], p[1]})
		}
	}
	scale := math.Pow(10, footprintDecimals)
	out := make([][]float64, 0, len(pts))
	for _, p := range geo.Simplify(pts, tol) {
		q := []float64{math.Round(p[0]*scale) / scale, math.Round(p[1]*scale) / scale}
		if n := len(out); n > 0 && out[n-1][0] == q[0] && out[n-1][1] == q[1] {
			continue
		}
		out = append(out, q)
	}
	return out
}
