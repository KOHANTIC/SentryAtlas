package adapters

import (
	"context"
	"errors"
	"math"
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

// BBox is a longitude and latitude box. When MinLon is greater than MaxLon
// the box crosses the date line.
type BBox struct {
	MinLon float64
	MinLat float64
	MaxLon float64
	MaxLat float64
}

// NewBBox checks a box and puts its longitudes on the globe. A box 360° wide
// or more is the whole world. Otherwise a longitude outside -180..180 is
// wrapped back into it, so a map's view over the Pacific (170 to 190) becomes
// the date-line-crossing box 170 to -170, as GeoJSON writes it.
func NewBBox(minLon, minLat, maxLon, maxLat float64) (*BBox, error) {
	for _, v := range []float64{minLon, minLat, maxLon, maxLat} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return nil, errors.New("values must be finite numbers")
		}
	}
	if minLat < -90 || minLat > 90 || maxLat < -90 || maxLat > 90 {
		return nil, errors.New("latitudes must be between -90 and 90")
	}
	if maxLon-minLon >= 360 {
		return &BBox{MinLon: -180, MinLat: minLat, MaxLon: 180, MaxLat: maxLat}, nil
	}
	return &BBox{MinLon: wrapLon(minLon), MinLat: minLat, MaxLon: wrapLon(maxLon), MaxLat: maxLat}, nil
}

// wrapLon brings a longitude outside -180..180 back onto the globe.
func wrapLon(lon float64) float64 {
	if lon >= -180 && lon <= 180 {
		return lon
	}
	return math.Mod(math.Mod(lon+180, 360)+360, 360) - 180
}

// Contains reports whether the point lies in the box, on either side of the
// date line when the box crosses it.
func (b *BBox) Contains(lon, lat float64) bool {
	if lat < b.MinLat || lat > b.MaxLat {
		return false
	}
	if b.MinLon <= b.MaxLon {
		return lon >= b.MinLon && lon <= b.MaxLon
	}
	return lon >= b.MinLon || lon <= b.MaxLon
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
