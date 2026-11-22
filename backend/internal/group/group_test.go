package group

import (
	"testing"
	"time"

	"github.com/KOHANTIC/SentryAtlas/backend/internal/event"
)

func quakeAt(source, id, at string, lon, lat, mag float64) event.Event {
	t, err := time.Parse(time.RFC3339Nano, at)
	if err != nil {
		panic(err)
	}
	return event.Event{ID: id, Source: source, Type: "earthquake", Time: event.Time{Started: t, Updated: t},
		Magnitude: &event.Magnitude{Value: mag, Unit: "M"}, Location: &event.Location{Center: &event.Point{lon, lat}}}
}

func byID(events []event.Event) map[string]event.Event {
	m := map[string]event.Event{}
	for _, e := range events {
		m[e.ID] = e
	}
	return m
}

// Recorded pairs from 2026-10-23: two GDACS quakes and their USGS twins,
// and a GDACS quake with a second USGS quake 10 km away ten minutes before.
func TestQuakesFromBothSourcesAreOneEvent(t *testing.T) {
	t.Parallel()
	events := []event.Event{
		quakeAt("gdacs", "gdacs-EQ-1573456", "2026-10-23T20:37:41Z", 93.943, 32.4693, 4.8),
		quakeAt("usgs", "usgs-us6000u4gj", "2026-10-23T20:37:41.926Z", 93.8874, 32.4478, 4.8),
		quakeAt("gdacs", "gdacs-EQ-1573468", "2026-10-23T22:05:10Z", 165.6172, -13.8764, 5.1),
		quakeAt("usgs", "usgs-us6000u4h7", "2026-10-23T22:05:11.235Z", 165.5628, -13.9002, 5.1),
		quakeAt("gdacs", "gdacs-EQ-1573457", "2026-10-23T20:58:01Z", 121.9479, 23.7861, 4.9),
		quakeAt("usgs", "usgs-us6000u4gs", "2026-10-23T20:58:01.497Z", 121.9786, 23.7912, 5.0),
		quakeAt("usgs", "usgs-us6000u4gm", "2026-10-23T20:48:17.044Z", 121.9086, 23.8739, 4.8),
	}
	Assign(events)
	got := byID(events)
	for gdacs, usgs := range map[string]string{"gdacs-EQ-1573456": "usgs-us6000u4gj", "gdacs-EQ-1573468": "usgs-us6000u4h7", "gdacs-EQ-1573457": "usgs-us6000u4gs"} {
		g, u := got[gdacs], got[usgs]
		if g.GroupID == "" || g.GroupID != u.GroupID || g.GroupID != "g-"+usgs {
			t.Errorf("%s in %q and %s in %q, want both in g-%s", gdacs, g.GroupID, usgs, u.GroupID, usgs)
		}
		if !u.Primary || g.Primary {
			t.Errorf("%s primary %v, %s primary %v; want USGS as the primary", usgs, u.Primary, gdacs, g.Primary)
		}
	}
	if e := got["usgs-us6000u4gm"]; e.GroupID != "" || e.Primary {
		t.Errorf("the quake ten minutes earlier is grouped: %+v; it is another quake", e)
	}
}

func TestQuakesTooFarApartStayApart(t *testing.T) {
	t.Parallel()
	for name, other := range map[string]event.Event{
		"a minute later":     quakeAt("usgs", "usgs-b", "2026-10-23T20:38:42Z", 93.943, 32.4693, 4.8),
		"200 km away":        quakeAt("usgs", "usgs-b", "2026-10-23T20:37:41Z", 95.9, 32.4693, 4.8),
		"a magnitude larger": quakeAt("usgs", "usgs-b", "2026-10-23T20:37:41Z", 93.943, 32.4693, 5.8),
	} {
		events := []event.Event{quakeAt("gdacs", "gdacs-EQ-1", "2026-10-23T20:37:41Z", 93.943, 32.4693, 4.8), other}
		Assign(events)
		if events[0].GroupID != "" || events[1].GroupID != "" {
			t.Errorf("%s: grouped as %q", name, events[0].GroupID)
		}
	}
	// Two USGS quakes are never one event: only another source repeats one.
	events := []event.Event{quakeAt("usgs", "usgs-a", "2026-10-23T20:37:41Z", 93.943, 32.4693, 4.8), quakeAt("usgs", "usgs-b", "2026-10-23T20:37:42Z", 93.943, 32.4693, 4.8)}
	Assign(events)
	if events[0].GroupID != "" {
		t.Error("two USGS quakes were grouped")
	}
}

func TestEONETLinksToTheGDACSReportItCameFrom(t *testing.T) {
	t.Parallel()
	events := []event.Event{
		{ID: "eonet-EONET_1", Source: "eonet", Type: "wildfire", Details: event.Details{EONET: &event.EONET{
			Sources: []event.Link{{ID: "IRWIN", URL: "https://irwin.example/1"}, {ID: "GDACS", URL: "https://www.gdacs.org/report.aspx?eventtype=WF&eventid=1023712"}}}}},
		{ID: "gdacs-WF-1023712", Source: "gdacs", Type: "wildfire"},
		{ID: "gdacs-WF-9", Source: "gdacs", Type: "wildfire"},
	}
	Assign(events)
	got := byID(events)
	if got["eonet-EONET_1"].GroupID != "g-gdacs-WF-1023712" || !got["gdacs-WF-1023712"].Primary || got["gdacs-WF-9"].GroupID != "" {
		t.Errorf("groups = %+v", events)
	}
}

func TestGroupsAreWorkedOutAgainEachTime(t *testing.T) {
	t.Parallel()
	events := []event.Event{quakeAt("usgs", "usgs-a", "2026-10-23T20:37:41Z", 1, 1, 5)}
	events[0].GroupID, events[0].Primary = "g-old", true
	Assign(events)
	if events[0].GroupID != "" || events[0].Primary {
		t.Errorf("a lone event kept an old group: %+v", events[0])
	}
}

func TestGDACSID(t *testing.T) {
	t.Parallel()
	for link, want := range map[string]string{
		"https://www.gdacs.org/report.aspx?eventtype=WF&eventid=1023712":             "gdacs-WF-1023712",
		"https://www.gdacs.org/report.aspx?eventid=1001350&episodeid=3&eventtype=TC": "gdacs-TC-1001350",
		"https://www.gdacs.org/":   "",
		"%%":                       "",
		"https://volcano.si.edu/x": "",
	} {
		if got := gdacsID(link); got != want {
			t.Errorf("gdacsID(%q) = %q, want %q", link, got, want)
		}
	}
}
