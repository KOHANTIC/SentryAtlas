package service

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/KOHANTIC/SentryAtlas/backend/internal/adapters"
	"github.com/KOHANTIC/SentryAtlas/backend/internal/models"
	"github.com/KOHANTIC/SentryAtlas/backend/internal/store"
)

// ErrAllSourcesFailed reports that no relevant source has answered since the
// server started, so an empty result would be misleading rather than
// meaningful. Once one has, its last good copy is served instead.
var ErrAllSourcesFailed = errors.New("all upstream sources failed")

// StreamBatch is one source's part of a stream: its status, and its events
// when it has a copy to serve.
type StreamBatch struct {
	Status models.SourceStatus
	Events []models.Event
}

// EventsService answers v1 requests from the store the pollers keep; it
// never calls a source.
type EventsService struct {
	adapters []adapters.Adapter
	store    *store.Store
}

// NewEventsService serves the store's copies; the adapters say which types
// each source can have, so a request reads only the sources it can match.
func NewEventsService(adapterList []adapters.Adapter, st *store.Store) *EventsService {
	return &EventsService{adapters: adapterList, store: st}
}

// sourceCopy is one source's copy as v1 sees it.
type sourceCopy struct {
	status   models.SourceStatus
	events   []models.Event
	answered bool // the source has answered at least once since the start
}

// copies reads the relevant sources' copies from the store, in the
// adapters' order, projected onto v1.
func (s *EventsService) copies(relevant []adapters.Adapter) []sourceCopy {
	byName := map[string]store.Copy{}
	for _, c := range s.store.Read(time.Now()) {
		byName[c.Status.Source] = c
	}
	out := make([]sourceCopy, 0, len(relevant))
	for _, a := range relevant {
		c, ok := byName[a.Source()]
		if !ok {
			out = append(out, sourceCopy{status: models.SourceStatus{Source: a.Source(), Error: "not polled yet"}})
			continue
		}
		// shortcut: every copy is projected on each request (about 18,000
		// events, a few milliseconds); keep the projection per fetched_at if
		// request volume ever makes it show.
		events := make([]models.Event, len(c.Events))
		for i, e := range c.Events {
			events[i] = e.V1()
		}
		out = append(out, sourceCopy{
			status: models.SourceStatus{
				Source:    c.Status.Source,
				OK:        c.Status.OK,
				Error:     c.Status.Error,
				Stale:     c.Status.Stale,
				FetchedAt: c.Status.FetchedAt,
			},
			events:   events,
			answered: c.Status.FetchedAt != nil,
		})
	}
	return out
}

// GetEvents merges the relevant sources' copies, filtered, newest first and
// cut to the limit.
func (s *EventsService) GetEvents(_ context.Context, params adapters.FetchParams) ([]models.Event, []models.SourceStatus, error) {
	relevant := s.selectAdapters(params.Types)
	if len(relevant) == 0 {
		return []models.Event{}, nil, nil
	}

	var allEvents []models.Event
	statuses := make([]models.SourceStatus, 0, len(relevant))
	answered := false
	for _, c := range s.copies(relevant) {
		statuses = append(statuses, c.status)
		answered = answered || c.answered
		allEvents = append(allEvents, c.events...)
	}
	if !answered {
		return nil, statuses, ErrAllSourcesFailed
	}

	allEvents = filterEvents(allEvents, params)
	sortEventsByDate(allEvents)
	if params.Limit > 0 && len(allEvents) > params.Limit {
		allEvents = allEvents[:params.Limit]
	}
	return allEvents, statuses, nil
}

// StreamEvents sends one batch per relevant source, in order, each sorted
// newest first, and stops when ctx ends. Limit is enforced across the whole
// stream: once the cap is reached, later batches are trimmed or emptied.
func (s *EventsService) StreamEvents(ctx context.Context, params adapters.FetchParams, ch chan<- StreamBatch) {
	remaining := params.Limit // 0 means unlimited
	for _, c := range s.copies(s.selectAdapters(params.Types)) {
		filtered := filterEvents(c.events, params)
		sortEventsByDate(filtered)
		if params.Limit > 0 {
			if remaining <= 0 {
				filtered = nil
			} else if len(filtered) > remaining {
				filtered = filtered[:remaining]
			}
			remaining -= len(filtered)
		}
		// Sent even when empty, so the consumer can report the source.
		select {
		case ch <- StreamBatch{Status: c.status, Events: filtered}:
		case <-ctx.Done():
			return
		}
	}
}

func (s *EventsService) selectAdapters(types []string) []adapters.Adapter {
	var result []adapters.Adapter
	for _, a := range s.adapters {
		if adapters.SupportsAnyType(a, types) {
			result = append(result, a)
		}
	}
	return result
}

func filterEvents(events []models.Event, params adapters.FetchParams) []models.Event {
	filtered := make([]models.Event, 0, len(events))

	typeSet := make(map[string]struct{})
	for _, t := range params.Types {
		typeSet[t] = struct{}{}
	}

	for _, e := range events {
		if len(typeSet) > 0 {
			if _, ok := typeSet[e.EventType]; !ok {
				continue
			}
		}

		if !params.Since.IsZero() && e.StartedAt.Before(params.Since) {
			continue
		}

		if params.BBox != nil {
			// Events without coordinates cannot be inside any bounding box.
			// They used to bypass this filter, so every bbox query returned
			// all unlocated events worldwide.
			if len(e.Geometry.Coordinates) < 2 {
				continue
			}
			if !params.BBox.Contains(e.Geometry.Coordinates[0], e.Geometry.Coordinates[1]) {
				continue
			}
		}

		filtered = append(filtered, e)
	}

	return filtered
}

func sortEventsByDate(events []models.Event) {
	sort.Slice(events, func(i, j int) bool {
		return events[i].StartedAt.After(events[j].StartedAt)
	})
}
