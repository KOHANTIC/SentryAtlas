package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sort"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/KOHANTIC/SentryAtlas/backend/internal/adapters"
	"github.com/KOHANTIC/SentryAtlas/backend/internal/cache"
	"github.com/KOHANTIC/SentryAtlas/backend/internal/models"
)

// ErrAllSourcesFailed reports that every relevant upstream source failed,
// so an empty result would be misleading rather than meaningful.
var ErrAllSourcesFailed = errors.New("all upstream sources failed")

// StreamBatch is one per-source delivery on the streaming path. Either
// Events or Err is set, never both.
type StreamBatch struct {
	Source string
	Events []models.Event
	Err    error
}

type EventsService struct {
	adapters    []adapters.Adapter
	sourceCache *cache.Cache[[]models.Event]
	sfGroup     singleflight.Group
	timeout     time.Duration
}

func NewEventsService(
	adapterList []adapters.Adapter,
	c *cache.Cache[[]models.Event],
	timeout time.Duration,
) *EventsService {
	return &EventsService{
		adapters:    adapterList,
		sourceCache: c,
		timeout:     timeout,
	}
}

// fetchAdapter returns cached events for an adapter or fetches from upstream.
// Uses singleflight to deduplicate concurrent requests for the same data.
// The upstream fetch uses a detached context so that cancellation of one
// request doesn't kill a shared in-flight call that other requests need.
func (s *EventsService) fetchAdapter(a adapters.Adapter) ([]models.Event, error) {
	// One entry per source: every adapter fetches the same window whatever
	// the request, so every request within the TTL shares one fetch.
	key := a.Source()

	if cached, ok := s.sourceCache.Get(key); ok {
		return cached, nil
	}

	result, err, _ := s.sfGroup.Do(key, func() (_ any, err error) {
		// Adapter work runs outside the HTTP recoverer, so a panic here would
		// end the process. Recovered inside the singleflight call, it becomes
		// this source's failure for every request waiting on it.
		defer func() {
			if r := recover(); r != nil {
				slog.Error("adapter panicked", "source", key, "panic", fmt.Sprint(r), "stack", string(debug.Stack()))
				err = fmt.Errorf("%s: %w: %v", key, errInternal, r)
			}
		}()

		ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
		defer cancel()
		events, err := a.FetchEvents(ctx)
		if err != nil {
			return nil, err
		}
		s.sourceCache.Set(key, events)
		return events, nil
	})
	if err != nil {
		return nil, err
	}

	return result.([]models.Event), nil
}

func (s *EventsService) GetEvents(ctx context.Context, params adapters.FetchParams) ([]models.Event, []models.SourceStatus, error) {
	relevant := s.selectAdapters(params.Types)
	if len(relevant) == 0 {
		return []models.Event{}, nil, nil
	}

	var mu sync.Mutex
	var allEvents []models.Event
	statuses := make([]models.SourceStatus, 0, len(relevant))
	var wg sync.WaitGroup

	for _, a := range relevant {
		wg.Add(1)
		go func() {
			defer wg.Done()
			events, err := s.fetchAdapter(a)

			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				slog.Warn("adapter fetch failed",
					"source", a.Source(),
					"error", err,
				)
				statuses = append(statuses, models.SourceStatus{
					Source: a.Source(),
					Error:  Reason(err),
				})
				return
			}
			statuses = append(statuses, models.SourceStatus{
				Source: a.Source(),
				OK:     true,
			})
			allEvents = append(allEvents, events...)
		}()
	}

	// Wait for the fan-out, but stop blocking the caller if it goes away.
	// Abandoned goroutines still complete their detached fetches and warm
	// the cache — only this request's wait is cancelled.
	waitDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitDone)
	}()
	select {
	case <-waitDone:
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	}

	failed := 0
	for _, st := range statuses {
		if !st.OK {
			failed++
		}
	}
	if failed == len(relevant) {
		return nil, statuses, ErrAllSourcesFailed
	}

	allEvents = filterEvents(allEvents, params)
	sortEventsByDate(allEvents)

	if params.Limit > 0 && len(allEvents) > params.Limit {
		allEvents = allEvents[:params.Limit]
	}

	return allEvents, statuses, nil
}

// StreamEvents delivers one batch per source as each upstream fetch
// completes, preserving the progressive-loading UX that SSE exists for.
// Batches are date-sorted internally; a global sort across sources would
// require waiting for every adapter, defeating the streaming. Limit is
// enforced across the whole stream: once the cap is reached, later batches
// are trimmed or dropped.
func (s *EventsService) StreamEvents(ctx context.Context, params adapters.FetchParams, ch chan<- StreamBatch) {
	relevant := s.selectAdapters(params.Types)
	if len(relevant) == 0 {
		return
	}

	var wg sync.WaitGroup
	var limitMu sync.Mutex
	remaining := params.Limit // 0 means unlimited

	for _, a := range relevant {
		wg.Add(1)
		go func() {
			defer wg.Done()
			events, err := s.fetchAdapter(a)
			if err != nil {
				slog.Warn("adapter stream failed",
					"source", a.Source(),
					"error", err,
				)
				select {
				case ch <- StreamBatch{Source: a.Source(), Err: err}:
				case <-ctx.Done():
				}
				return
			}
			filtered := filterEvents(events, params)
			sortEventsByDate(filtered)

			if params.Limit > 0 {
				limitMu.Lock()
				if remaining <= 0 {
					filtered = nil
				} else if len(filtered) > remaining {
					filtered = filtered[:remaining]
				}
				remaining -= len(filtered)
				limitMu.Unlock()
			}

			// Sent even when empty so the consumer can report the source
			// as reachable.
			select {
			case ch <- StreamBatch{Source: a.Source(), Events: filtered}:
			case <-ctx.Done():
			}
		}()
	}

	wg.Wait()
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
