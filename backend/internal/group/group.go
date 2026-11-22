// Package group finds the reports of one real event from different sources
// and gives them one group id and one primary member, so a map draws one
// marker, not a stack.
package group

import (
	"math"
	"net/url"
	"sort"

	"github.com/KOHANTIC/SentryAtlas/backend/internal/event"
)

// Two quakes are one when they are this close. GDACS repeats USGS's quakes:
// on 2026-11-22 its 519 quakes in the window lay a median 0.5 s, 0.8 km and
// 0.0 magnitude from their USGS twins, and these limits paired 516.
const (
	quakeSeconds = 30
	quakeKm      = 50
	quakeMag     = 0.5
)

// rank orders which member of a group is primary: the source closest to the
// event first.
var rank = map[string]int{event.SourceUSGS: 0, event.SourceGDACS: 1, event.SourceNOAA: 2, event.SourceEONET: 3}

// Assign sets GroupID on every event that has a twin in another source,
// the same for all of a group, and Primary on one member of each. Ids are
// worked out again on every call; an event alone has neither.
func Assign(events []event.Event) {
	byID := make(map[string]int, len(events))
	parent := make([]int, len(events))
	for i := range events {
		byID[events[i].ID] = i
		parent[i] = i
		events[i].GroupID, events[i].Primary = "", false
	}
	find := func(i int) int {
		for parent[i] != i {
			parent[i] = parent[parent[i]]
			i = parent[i]
		}
		return i
	}
	union := func(a, b int) { parent[find(a)] = find(b) }

	// Exact links: EONET lists the GDACS report an event came from.
	for i, e := range events {
		if e.Details.EONET == nil {
			continue
		}
		for _, s := range e.Details.EONET.Sources {
			if s.ID != "GDACS" {
				continue
			}
			if j, ok := byID[gdacsID(s.URL)]; ok {
				union(i, j)
			}
		}
	}
	for _, p := range quakePairs(events) {
		union(p[0], p[1])
	}

	members := map[int][]int{}
	for i := range events {
		r := find(i)
		members[r] = append(members[r], i)
	}
	for _, m := range members {
		if len(m) < 2 {
			continue
		}
		primary := m[0]
		for _, i := range m[1:] {
			a, b := events[i], events[primary]
			if rank[a.Source] < rank[b.Source] || (rank[a.Source] == rank[b.Source] && a.ID < b.ID) {
				primary = i
			}
		}
		id := "g-" + events[primary].ID
		for _, i := range m {
			events[i].GroupID = id
		}
		events[primary].Primary = true
	}
}

// gdacsID is the event id of a GDACS report link, as in
// gdacs-WF-1023712; "" if the link is not one.
func gdacsID(link string) string {
	u, err := url.Parse(link)
	if err != nil {
		return ""
	}
	q := u.Query()
	if q.Get("eventtype") == "" || q.Get("eventid") == "" {
		return ""
	}
	return "gdacs-" + q.Get("eventtype") + "-" + q.Get("eventid")
}

type quake struct {
	i    int
	t    float64 // Unix seconds
	pt   event.Point
	mag  float64
	used bool
}

// quakePairs matches each GDACS quake to the USGS quake it repeats: the
// closest within the limits, each quake used once, best pairs first.
func quakePairs(events []event.Event) [][2]int {
	var usgs, gdacs []*quake
	for i, e := range events {
		if e.Type != "earthquake" || e.Magnitude == nil || e.Location == nil || e.Location.Center == nil {
			continue
		}
		q := &quake{i: i, t: float64(e.Time.Started.UnixMilli()) / 1000, pt: *e.Location.Center, mag: e.Magnitude.Value}
		switch e.Source {
		case event.SourceUSGS:
			usgs = append(usgs, q)
		case event.SourceGDACS:
			gdacs = append(gdacs, q)
		}
	}
	// Ties broken by position, so the pairs, and the snapshot's tag, are the same every build.
	sort.Slice(usgs, func(a, b int) bool {
		if usgs[a].t != usgs[b].t {
			return usgs[a].t < usgs[b].t
		}
		return usgs[a].i < usgs[b].i
	})

	type candidate struct {
		g, u  *quake
		score float64
	}
	var cands []candidate
	for _, g := range gdacs {
		from := sort.Search(len(usgs), func(k int) bool { return usgs[k].t >= g.t-quakeSeconds })
		for _, u := range usgs[from:] {
			if u.t > g.t+quakeSeconds {
				break
			}
			km := distanceKm(g.pt, u.pt)
			dm := math.Abs(g.mag - u.mag)
			if km > quakeKm || dm > quakeMag {
				continue
			}
			cands = append(cands, candidate{g, u, math.Abs(g.t-u.t)/quakeSeconds + km/quakeKm + dm/quakeMag})
		}
	}
	sort.SliceStable(cands, func(a, b int) bool { return cands[a].score < cands[b].score })
	var pairs [][2]int
	for _, c := range cands {
		if c.g.used || c.u.used {
			continue
		}
		c.g.used, c.u.used = true, true
		pairs = append(pairs, [2]int{c.g.i, c.u.i})
	}
	return pairs
}

// distanceKm is the great-circle distance between two [lon, lat] points.
func distanceKm(a, b event.Point) float64 {
	const r = 6371.0
	la1, la2 := a[1]*math.Pi/180, b[1]*math.Pi/180
	dla := la2 - la1
	dlo := (b[0] - a[0]) * math.Pi / 180
	h := math.Sin(dla/2)*math.Sin(dla/2) + math.Cos(la1)*math.Cos(la2)*math.Sin(dlo/2)*math.Sin(dlo/2)
	return 2 * r * math.Asin(math.Min(1, math.Sqrt(h)))
}
