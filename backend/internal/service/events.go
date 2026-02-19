package service

import (
	"context"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/Kohantic/SentryAtlas/backend/internal/adapters"
	"github.com/Kohantic/SentryAtlas/backend/internal/cache"
	"github.com/Kohantic/SentryAtlas/backend/internal/models"
)

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
func (s *EventsService) fetchAdapter(a adapters.Adapter, params adapters.FetchParams) ([]models.Event, error) {
	key := adapterCacheKey(a.Source(), params.Types, params.Since)

	if cached, ok := s.sourceCache.Get(key); ok {
		return cached, nil
	}

	upstreamParams := adapters.FetchParams{
		Types: params.Types,
		Since: params.Since,
	}

	result, err, _ := s.sfGroup.Do(key, func() (any, error) {
		ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
		defer cancel()
		events, err := a.FetchEvents(ctx, upstreamParams)
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

func (s *EventsService) GetEvents(ctx context.Context, params adapters.FetchParams) ([]models.Event, error) {
	relevant := s.selectAdapters(params.Types)
	if len(relevant) == 0 {
		return []models.Event{}, nil
	}

	var mu sync.Mutex
	var allEvents []models.Event
	var wg sync.WaitGroup

	for _, a := range relevant {
		a := a
		wg.Add(1)
		go func() {
			defer wg.Done()
			events, err := s.fetchAdapter(a, params)
			if err != nil {
				slog.Warn("adapter fetch failed",
					"source", a.Source(),
					"error", err,
				)
				return
			}
			mu.Lock()
			allEvents = append(allEvents, events...)
			mu.Unlock()
		}()
	}

	wg.Wait()

	allEvents = filterEvents(allEvents, params)
	sortEventsByDate(allEvents)

	if params.Limit > 0 && len(allEvents) > params.Limit {
		allEvents = allEvents[:params.Limit]
	}

	return allEvents, nil
}

func (s *EventsService) StreamEvents(ctx context.Context, params adapters.FetchParams, ch chan<- []models.Event) {
	relevant := s.selectAdapters(params.Types)
	if len(relevant) == 0 {
		return
	}

	var wg sync.WaitGroup

	for _, a := range relevant {
		a := a
		wg.Add(1)
		go func() {
			defer wg.Done()
			events, err := s.fetchAdapter(a, params)
			if err != nil {
				slog.Warn("adapter stream failed",
					"source", a.Source(),
					"error", err,
				)
				return
			}
			filtered := filterEvents(events, params)
			if len(filtered) > 0 {
				select {
				case ch <- filtered:
				case <-ctx.Done():
				}
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

		if params.BBox != nil && len(e.Geometry.Coordinates) >= 2 {
			lon, lat := e.Geometry.Coordinates[0], e.Geometry.Coordinates[1]
			if lon < params.BBox.MinLon || lon > params.BBox.MaxLon ||
				lat < params.BBox.MinLat || lat > params.BBox.MaxLat {
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

// adapterCacheKey builds a cache key from source identity and the parameters
// that actually change upstream results. BBox and Limit are excluded because
// adapters either don't support them or we want the full dataset for local filtering.
func adapterCacheKey(source string, types []string, since time.Time) string {
	sorted := make([]string, len(types))
	copy(sorted, types)
	sort.Strings(sorted)

	var b strings.Builder
	b.WriteString(source)
	b.WriteByte(':')
	b.WriteString(strings.Join(sorted, ","))
	if !since.IsZero() {
		b.WriteByte(':')
		b.WriteString(since.Format(time.RFC3339))
	}
	return b.String()
}
