package models

// EventTypes is the canonical list of event types the API can emit.
// Adapters map upstream categories into these. Two are fallbacks that are
// nonetheless real, requestable types: "weather" covers NWS alerts with no
// more specific class, and "other" covers upstream categories no adapter
// maps yet.
var EventTypes = []string{
	"earthquake",
	"wildfire",
	"volcano",
	"storm",
	"flood",
	"cyclone",
	"tornado",
	"hurricane",
	"winter_storm",
	"tsunami",
	"drought",
	"iceberg",
	"landslide",
	"weather",
	"other",
}

var validEventTypes = func() map[string]struct{} {
	m := make(map[string]struct{}, len(EventTypes))
	for _, t := range EventTypes {
		m[t] = struct{}{}
	}
	return m
}()

// IsValidEventType reports whether t is a known event type.
func IsValidEventType(t string) bool {
	_, ok := validEventTypes[t]
	return ok
}
