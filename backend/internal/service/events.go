package service

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/kohantic/disaster-watch/backend/internal/adapters"
	"github.com/kohantic/disaster-watch/backend/internal/cache"
	"github.com/kohantic/disaster-watch/backend/internal/models"
)

type EventsService struct {
	adapters []adapters.Adapter
	cache    *cache.Cache[[]models.Event]
	timeout  time.Duration
}

func NewEventsService(
	adapterList []adapters.Adapter,
	c *cache.Cache[[]models.Event],
	timeout time.Duration,
) *EventsService {
	return &EventsService{
		adapters: adapterList,
		cache:    c,
		timeout:  timeout,
	}
}

func (s *EventsService) GetEvents(ctx context.Context, params adapters.FetchParams) ([]models.Event, error) {
	cacheKey := buildCacheKey(params)
	if cached, ok := s.cache.Get(cacheKey); ok {
		return cached, nil
	}

	relevant := s.selectAdapters(params.Types)
	if len(relevant) == 0 {
		return []models.Event{}, nil
	}

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	var mu sync.Mutex
	var allEvents []models.Event

	g, gctx := errgroup.WithContext(ctx)

	for _, a := range relevant {
		a := a
		g.Go(func() error {
			events, err := a.FetchEvents(gctx, params)
			if err != nil {
				slog.Warn("adapter fetch failed",
					"source", a.Source(),
					"error", err,
				)
				// Partial failure: log and continue with other adapters
				return nil
			}
			mu.Lock()
			allEvents = append(allEvents, events...)
			mu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("events service: %w", err)
	}

	allEvents = filterEvents(allEvents, params)
	sortEventsByDate(allEvents)

	if params.Limit > 0 && len(allEvents) > params.Limit {
		allEvents = allEvents[:params.Limit]
	}

	s.cache.Set(cacheKey, allEvents)

	return allEvents, nil
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

func buildCacheKey(params adapters.FetchParams) string {
	parts := []string{}

	if len(params.Types) > 0 {
		sorted := make([]string, len(params.Types))
		copy(sorted, params.Types)
		sort.Strings(sorted)
		parts = append(parts, "types="+strings.Join(sorted, ","))
	}

	if params.BBox != nil {
		parts = append(parts, fmt.Sprintf("bbox=%.4f,%.4f,%.4f,%.4f",
			params.BBox.MinLon, params.BBox.MinLat,
			params.BBox.MaxLon, params.BBox.MaxLat))
	}

	if !params.Since.IsZero() {
		parts = append(parts, "since="+params.Since.Format(time.RFC3339))
	}

	if params.Limit > 0 {
		parts = append(parts, fmt.Sprintf("limit=%d", params.Limit))
	}

	key := strings.Join(parts, "&")
	hash := sha256.Sum256([]byte(key))
	return fmt.Sprintf("%x", hash[:8])
}
