package store

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/KOHANTIC/SentryAtlas/backend/internal/event"
)

var t0 = time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)

// events makes n events of a source, expiring at expires when it is set.
func events(source string, n int, expires *time.Time) []event.Event {
	out := make([]event.Event, n)
	for i := range out {
		out[i] = event.Event{ID: fmt.Sprintf("%s-%d", source, i), Source: source, Time: event.Time{Started: t0, Updated: t0, Expires: expires}}
	}
	return out
}

func only(t *testing.T, s *Store, now time.Time) Copy {
	t.Helper()
	copies := s.Read(now)
	if len(copies) != 1 {
		t.Fatalf("got %d copies, want 1", len(copies))
	}
	return copies[0]
}

func TestAPlausibleCopyReplacesTheLast(t *testing.T) {
	t.Parallel()
	s := New(Source{Name: "usgs"})
	if err := s.Put("usgs", events("usgs", 100, nil), t0); err != nil {
		t.Fatal(err)
	}
	if err := s.Put("usgs", events("usgs", 10, nil), t0.Add(time.Minute)); err != nil {
		t.Fatalf("a tenth of the last copy: %v, want it accepted", err)
	}
	c := only(t, s, t0.Add(time.Minute))
	if !c.Status.OK || c.Status.Stale || c.Status.Count != 10 || !c.Status.FetchedAt.Equal(t0.Add(time.Minute)) {
		t.Errorf("status = %+v, want ok and fresh with 10", c.Status)
	}
}

// An empty or much smaller answer is far likelier a broken one than the
// world going quiet, until it repeats.
func TestAnImplausibleCopyIsHeldUntilItRepeats(t *testing.T) {
	t.Parallel()
	for name, next := range map[string]int{"empty": 0, "under a tenth": 9} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			s := New(Source{Name: "usgs"})
			if err := s.Put("usgs", events("usgs", 100, nil), t0); err != nil {
				t.Fatal(err)
			}
			for i := 1; i < acceptAfter; i++ {
				if err := s.Put("usgs", events("usgs", next, nil), t0.Add(time.Duration(i)*time.Minute)); err == nil {
					t.Fatalf("copy %d of %d events accepted, want it held", i, next)
				}
				c := only(t, s, t0)
				if c.Status.OK || !c.Status.Stale || c.Status.Count != 100 || !c.Status.FetchedAt.Equal(t0) || !strings.Contains(c.Status.Error, "where the last copy had 100") {
					t.Fatalf("after copy %d: status = %+v, want the last copy served, stale", i, c.Status)
				}
			}
			if err := s.Put("usgs", events("usgs", next, nil), t0.Add(time.Hour)); err != nil {
				t.Fatalf("copy %d in a row: %v, want it accepted as real", acceptAfter, err)
			}
			if c := only(t, s, t0); c.Status.Count != next || !c.Status.OK {
				t.Errorf("status = %+v, want the new copy", c.Status)
			}
		})
	}
}

func TestASmallSourceMayShrink(t *testing.T) {
	t.Parallel()
	s := New(Source{Name: "noaa"})
	s.Put("noaa", events("noaa", 99, nil), t0)
	if err := s.Put("noaa", events("noaa", 1, nil), t0); err != nil {
		t.Errorf("99 to 1: %v, want it accepted: a copy under 100 is too small to judge", err)
	}
}

func TestAFailureKeepsTheLastCopy(t *testing.T) {
	t.Parallel()
	s := New(Source{Name: "eonet"})
	s.Put("eonet", events("eonet", 5, nil), t0)
	s.Fail("eonet", "timed out")
	c := only(t, s, t0.Add(time.Hour))
	if c.Status.OK || !c.Status.Stale || c.Status.Error != "timed out" || c.Status.Count != 5 || len(c.Events) != 5 {
		t.Errorf("status = %+v with %d events, want the last 5, stale, with the reason", c.Status, len(c.Events))
	}
	if err := s.Put("eonet", events("eonet", 6, nil), t0.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if c := only(t, s, t0.Add(2*time.Hour)); !c.Status.OK || c.Status.Stale || c.Status.Error != "" {
		t.Errorf("after a good poll: status = %+v, want ok", c.Status)
	}
}

func TestAFailureBeforeAnyCopyServesNothing(t *testing.T) {
	t.Parallel()
	s := New(Source{Name: "gdacs"})
	s.Fail("gdacs", "answered HTTP 503")
	c := only(t, s, t0)
	if c.Status.OK || c.Status.Stale || c.Status.FetchedAt != nil || c.Status.Error != "answered HTTP 503" || len(c.Events) != 0 {
		t.Errorf("status = %+v, want a failure with nothing to serve", c.Status)
	}
}

func TestExpiredEventsAreHidden(t *testing.T) {
	t.Parallel()
	s := New(Source{Name: "noaa"})
	soon, later := t0.Add(time.Hour), t0.Add(3*time.Hour)
	list := append(events("noaa", 2, &soon), events("noaa", 3, &later)...)
	list = append(list, events("noaa", 1, nil)...)
	s.Put("noaa", list, t0)

	if c := only(t, s, t0.Add(30*time.Minute)); len(c.Events) != 6 {
		t.Errorf("before any expiry: %d events, want 6", len(c.Events))
	}
	c := only(t, s, t0.Add(2*time.Hour))
	if len(c.Events) != 4 || c.Status.Count != 4 {
		t.Errorf("after the first expiry: %d events, count %d, want 4", len(c.Events), c.Status.Count)
	}
	for _, e := range c.Events {
		if e.Time.Expires != nil && e.Time.Expires.Equal(soon) {
			t.Errorf("%s expired at %v and is still served", e.ID, soon)
		}
	}
	if !c.Status.OK {
		t.Errorf("status = %+v; expiry is not a failure", c.Status)
	}
}

// An alert copy that has not been refreshed for an hour may hold alerts the
// source has since cancelled; it is withdrawn rather than served.
func TestACopyPastItsAgeIsWithdrawn(t *testing.T) {
	t.Parallel()
	s := New(Source{Name: "noaa", MaxAge: time.Hour}, Source{Name: "usgs"})
	s.Put("noaa", events("noaa", 3, nil), t0)
	s.Put("usgs", events("usgs", 3, nil), t0)
	s.Fail("noaa", "timed out")

	copies := s.Read(t0.Add(2 * time.Hour))
	if nws := copies[0]; len(nws.Events) != 0 || nws.Status.OK || nws.Status.Stale || nws.Status.Error != tooOld || nws.Status.FetchedAt == nil {
		t.Errorf("noaa = %+v with %d events, want withdrawn, saying why, with when it last answered", nws.Status, len(nws.Events))
	}
	if usgs := copies[1]; len(usgs.Events) != 3 {
		t.Errorf("usgs has %d events, want 3: it has no age limit", len(usgs.Events))
	}
}

func TestTheCapDropsTheRest(t *testing.T) {
	t.Parallel()
	s := New(Source{Name: "eonet"})
	if err := s.Put("eonet", events("eonet", MaxEventsPerSource+5, nil), t0); err != nil {
		t.Fatal(err)
	}
	if c := only(t, s, t0); len(c.Events) != MaxEventsPerSource {
		t.Errorf("%d events, want the cap, %d", len(c.Events), MaxEventsPerSource)
	}
}

func TestReadyOnceEverySourceIsTried(t *testing.T) {
	t.Parallel()
	s := New(Source{Name: "usgs"}, Source{Name: "noaa"})
	if s.Ready() {
		t.Fatal("ready before any poll")
	}
	if c := s.Read(t0)[0]; c.Status.OK || c.Status.Error != "not polled yet" {
		t.Errorf("before any poll: status = %+v", c.Status)
	}
	s.Put("usgs", events("usgs", 1, nil), t0)
	if s.Ready() {
		t.Fatal("ready with noaa untried")
	}
	s.Fail("noaa", "timed out")
	if !s.Ready() {
		t.Error("not ready after both were tried; a failed try counts")
	}
}
