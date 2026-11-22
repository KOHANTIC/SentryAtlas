// Package store holds the latest good copy of each source's events, kept
// fresh by one poller per source, so a request never calls a source or waits
// on one.
package store

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/KOHANTIC/SentryAtlas/backend/internal/event"
)

const (
	// MaxEventsPerSource caps one source's copy; the largest, USGS's month,
	// held 9,444 on 2026-11-22.
	MaxEventsPerSource = 25_000
	// acceptAfter implausible copies in a row are taken as a real change.
	acceptAfter = 3
	// tooOld is the status a source shows once its copy is past its age.
	tooOld = "has not answered for too long; its last copy is no longer served"
)

// Source names a source the store keeps, and how old its copy may get
// before it is withdrawn; zero means it is served however old.
type Source struct {
	Name   string
	MaxAge time.Duration
}

// Store is safe for concurrent use. The events it returns are shared and
// must not be modified.
type Store struct {
	mu      sync.RWMutex
	order   []string
	entries map[string]*entry
	version uint64 // rises with every Put and Fail
}

type entry struct {
	maxAge    time.Duration
	events    []event.Event
	fetchedAt time.Time // when the copy in use arrived; zero if none has
	tried     bool
	reason    string // why the last poll's answer is not in use; "" if it is
	held      int    // implausible copies in a row
}

// New makes a store for the sources, in the order it reports them.
func New(sources ...Source) *Store {
	s := &Store{entries: map[string]*entry{}}
	for _, src := range sources {
		s.order = append(s.order, src.Name)
		s.entries[src.Name] = &entry{maxAge: src.MaxAge}
	}
	return s
}

// Put offers a source's new copy, fetched at at. It replaces the copy in use
// when it is plausible next to it, or when an implausible copy has come
// acceptAfter times in a row; otherwise the old copy stays, marked stale,
// and the returned error says why.
func (s *Store) Put(source string, events []event.Event, at time.Time) error {
	if len(events) > MaxEventsPerSource {
		slog.Warn("source copy over the cap; the rest is dropped", "source", source, "events", len(events), "cap", MaxEventsPerSource)
		events = events[:MaxEventsPerSource]
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.entry(source)
	e.tried = true
	s.version++
	if !plausible(len(e.events), len(events)) && e.held+1 < acceptAfter {
		e.held++
		e.reason = fmt.Sprintf("sent %d events where the last copy had %d", len(events), len(e.events))
		return fmt.Errorf("%s: %s", source, e.reason)
	}
	e.events, e.fetchedAt, e.reason, e.held = events, at, "", 0
	return nil
}

// Fail records a poll that got no copy, with the short reason clients see.
func (s *Store) Fail(source, reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.entry(source)
	e.tried = true
	e.held = 0
	e.reason = reason
	s.version++
}

// Version rises whenever a poll is recorded, so a reader can tell whether
// anything may have changed since it last looked.
func (s *Store) Version() uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.version
}

// plausible says whether a copy of n events can follow one of last: not
// empty after a full one, and not under a tenth of a copy of 100 or more.
func plausible(last, n int) bool {
	switch {
	case last == 0:
		return true
	case n == 0:
		return false
	case last >= 100:
		return n*10 >= last
	}
	return true
}

func (s *Store) entry(source string) *entry {
	e, ok := s.entries[source]
	if !ok {
		panic("store: unknown source " + source)
	}
	return e
}

// Copy is what one source contributes to a read.
type Copy struct {
	Status event.SourceStatus
	Events []event.Event
}

// Read returns each source's copy as of now, in the store's order. Events
// past their expiry are left out, and a copy past its source's age is
// withdrawn.
func (s *Store) Read(now time.Time) []Copy {
	s.mu.RLock()
	defer s.mu.RUnlock()
	copies := make([]Copy, 0, len(s.order))
	for _, name := range s.order {
		e := s.entries[name]
		st := event.SourceStatus{Source: name, Error: e.reason}
		if !e.tried {
			st.Error = "not polled yet"
		}
		var events []event.Event
		if !e.fetchedAt.IsZero() {
			at := e.fetchedAt
			st.FetchedAt = &at
			if e.maxAge > 0 && now.Sub(at) > e.maxAge {
				st.Error = tooOld
			} else {
				events = unexpired(e.events, now)
				st.Stale = e.reason != ""
			}
		}
		st.OK = st.Error == ""
		st.Count = len(events)
		copies = append(copies, Copy{Status: st, Events: events})
	}
	return copies
}

// unexpired is events without those whose record expired before now; the
// same slice when none has.
func unexpired(events []event.Event, now time.Time) []event.Event {
	for i, e := range events {
		if e.Time.Expires != nil && e.Time.Expires.Before(now) {
			kept := append(make([]event.Event, 0, len(events)), events[:i]...)
			for _, e := range events[i+1:] {
				if e.Time.Expires == nil || !e.Time.Expires.Before(now) {
					kept = append(kept, e)
				}
			}
			return kept
		}
	}
	return events
}

// Ready says whether every source has been polled at least once, answered
// or not.
func (s *Store) Ready() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, e := range s.entries {
		if !e.tried {
			return false
		}
	}
	return true
}
