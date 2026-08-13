package models

import "testing"

func TestIsValidEventTypeAcceptsAllCanonicalTypes(t *testing.T) {
	t.Parallel()
	for _, typ := range EventTypes {
		if !IsValidEventType(typ) {
			t.Errorf("IsValidEventType(%q) = false, want true for canonical type", typ)
		}
	}
}

func TestIsValidEventTypeRejectsUnknown(t *testing.T) {
	t.Parallel()
	for _, typ := range []string{"", "sharknado", "EARTHQUAKE", "earthquake ", "weather,other"} {
		if IsValidEventType(typ) {
			t.Errorf("IsValidEventType(%q) = true, want false", typ)
		}
	}
}

func TestEventTypesHasNoDuplicates(t *testing.T) {
	t.Parallel()
	seen := make(map[string]struct{}, len(EventTypes))
	for _, typ := range EventTypes {
		if _, dup := seen[typ]; dup {
			t.Errorf("EventTypes contains duplicate %q", typ)
		}
		seen[typ] = struct{}{}
	}
}

func TestEventTypesIncludesFallbacks(t *testing.T) {
	t.Parallel()
	// "weather" and "other" are fallback classifications adapters emit; they
	// must stay requestable or the handler would reject what the API serves.
	for _, typ := range []string{"weather", "other"} {
		if !IsValidEventType(typ) {
			t.Errorf("fallback type %q missing from EventTypes", typ)
		}
	}
}
