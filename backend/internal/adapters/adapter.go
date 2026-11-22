package adapters

import (
	"context"
	"time"

	"github.com/KOHANTIC/SentryAtlas/backend/internal/models"
)

type Adapter interface {
	// FetchEvents fetches the source's fixed window, the same for every
	// request, so one cached copy per source can answer all of them.
	FetchEvents(ctx context.Context) ([]models.Event, error)
	Source() string
	SupportedTypes() []string
}

// FetchParams carries a request's parameters. Adapters never see them: the
// service applies all four to the cached window, because anything passed
// upstream would make the cache key depend on the request.
type FetchParams struct {
	Types []string
	BBox  *BBox
	Since time.Time
	Limit int
}

type BBox struct {
	MinLon float64
	MinLat float64
	MaxLon float64
	MaxLat float64
}

// SupportsAnyType returns true if the adapter supports at least one of the requested types.
// If requested is empty, all adapters are considered matching.
func SupportsAnyType(adapter Adapter, requested []string) bool {
	if len(requested) == 0 {
		return true
	}
	supported := make(map[string]struct{})
	for _, t := range adapter.SupportedTypes() {
		supported[t] = struct{}{}
	}
	for _, t := range requested {
		if _, ok := supported[t]; ok {
			return true
		}
	}
	return false
}
