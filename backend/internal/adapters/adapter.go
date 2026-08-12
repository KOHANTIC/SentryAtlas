package adapters

import (
	"context"
	"time"

	"github.com/KOHANTIC/SentryAtlas/backend/internal/models"
)

type Adapter interface {
	FetchEvents(ctx context.Context, params FetchParams) ([]models.Event, error)
	Source() string
	SupportedTypes() []string
}

// FetchParams carries a request's parameters. Adapters only ever receive
// Types and Since: BBox and Limit are applied service-side after the merge,
// because passing them upstream would make every viewport a distinct cache
// key and defeat the per-source cache entirely.
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
