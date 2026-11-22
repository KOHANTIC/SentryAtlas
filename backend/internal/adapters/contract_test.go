package adapters

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/KOHANTIC/SentryAtlas/backend/internal/event"
)

// skipCounter counts, by source, the records fetchList skipped as
// unreadable, from its warning.
type skipCounter struct {
	mu sync.Mutex
	n  map[string]int64
}

func (c *skipCounter) Enabled(context.Context, slog.Level) bool { return true }
func (c *skipCounter) WithAttrs([]slog.Attr) slog.Handler       { return c }
func (c *skipCounter) WithGroup(string) slog.Handler            { return c }
func (c *skipCounter) Handle(_ context.Context, r slog.Record) error {
	if r.Message != "skipped records that could not be read" {
		return nil
	}
	var source string
	var skipped int64
	r.Attrs(func(a slog.Attr) bool {
		switch a.Key {
		case "source":
			source = a.Value.String()
		case "skipped":
			skipped = a.Value.Int64()
		}
		return true
	})
	c.mu.Lock()
	defer c.mu.Unlock()
	c.n[source] += skipped
	return nil
}

// TestLiveContract holds the adapters to what the sources send today, where
// the fixtures only prove the day they were recorded: every source answers
// and reads with no record skipped, every event is valid, and no field that
// every recorded record has is gone from every live one. New fields are
// listed, not failed. It needs the network, so it runs only when asked, as
// the weekly source check does:
//
//	LIVE_CONTRACT=1 go test ./internal/adapters -run LiveContract -count=1 -v
func TestLiveContract(t *testing.T) {
	if os.Getenv("LIVE_CONTRACT") == "" {
		t.Skip("set LIVE_CONTRACT=1 to check the adapters against the live sources")
	}
	skips := &skipCounter{n: map[string]int64{}}
	prev := slog.Default()
	slog.SetDefault(slog.New(skips))
	defer slog.SetDefault(prev)

	for _, s := range liveSources {
		t.Run(s.source, func(t *testing.T) {
			rt := &recordingTransport{}
			client := &http.Client{Timeout: 2 * time.Minute, Transport: rt}
			a, ok := s.adapter(client).(interface {
				FetchV2(context.Context) ([]event.Event, error)
			})
			if !ok {
				t.Fatal("the adapter has no FetchV2")
			}
			events, err := a.FetchV2(context.Background())
			if err != nil {
				t.Fatalf("fetch failed: %v", err)
			}
			invalid := 0
			for _, e := range events {
				if err := e.Validate(); err != nil {
					if invalid++; invalid <= 5 {
						t.Errorf("invalid event %s: %v", e.ID, err)
					}
				}
			}
			if invalid > 5 {
				t.Errorf("and %d more invalid events", invalid-5)
			}
			skips.mu.Lock()
			skipped := skips.n[s.source]
			skips.mu.Unlock()
			if skipped > 0 {
				t.Errorf("%d records could not be read", skipped)
			}

			var live []any
			for _, body := range rt.bodies {
				if len(body) == 0 {
					continue
				}
				var page map[string]any
				if err := json.Unmarshal(body, &page); err != nil {
					t.Fatalf("parse response: %v", err)
				}
				list, _ := page[s.key].([]any)
				live = append(live, list...)
			}
			data, err := os.ReadFile(filepath.Join("testdata", "live", s.source+".json"))
			if err != nil {
				t.Fatal(err)
			}
			var recorded map[string]any
			if err := json.Unmarshal(data, &recorded); err != nil {
				t.Fatal(err)
			}
			list, _ := recorded[s.key].([]any)
			gone, added := fieldChanges(list, live)
			for _, f := range gone {
				t.Errorf("field %q is in every recorded record and in no live one: renamed or removed", f)
			}
			for _, f := range added {
				t.Logf("new field %q", f)
			}
			t.Logf("%d records live, %d events, %d skipped, %d invalid", len(live), len(events), skipped, invalid)
		})
	}
}

// fieldChanges compares the fields of the recorded and live records, at the
// top and inside properties: gone are those every recorded record has and
// no live one does; added are those in some live record and no recorded one.
func fieldChanges(recorded, live []any) (gone, added []string) {
	count := func(records []any) map[string]int {
		n := map[string]int{}
		for _, r := range records {
			for f := range fields(r) {
				n[f]++
			}
		}
		return n
	}
	rec, now := count(recorded), count(live)
	for f, n := range rec {
		if n == len(recorded) && now[f] == 0 {
			gone = append(gone, f)
		}
	}
	for f := range now {
		if rec[f] == 0 {
			added = append(added, f)
		}
	}
	sort.Strings(gone)
	sort.Strings(added)
	return gone, added
}

// fields names a record's keys, with those of its properties object as
// "properties.<key>".
func fields(record any) map[string]bool {
	out := map[string]bool{}
	m, _ := record.(map[string]any)
	for k, v := range m {
		out[k] = true
		if props, ok := v.(map[string]any); ok && k == "properties" {
			for pk := range props {
				out["properties."+pk] = true
			}
		}
	}
	return out
}

func TestFieldChanges(t *testing.T) {
	t.Parallel()
	recorded := []any{
		map[string]any{"id": "1", "properties": map[string]any{"mag": 1.0, "place": "a"}},
		map[string]any{"id": "2", "properties": map[string]any{"mag": 2.0}},
	}
	live := []any{
		map[string]any{"id": "3", "properties": map[string]any{"magnitude": 1.0, "place": "b", "felt": 3}},
	}
	gone, added := fieldChanges(recorded, live)
	if len(gone) != 1 || gone[0] != "properties.mag" {
		t.Errorf("gone = %v, want [properties.mag]: place was not in every recorded record", gone)
	}
	if len(added) != 2 || added[0] != "properties.felt" || added[1] != "properties.magnitude" {
		t.Errorf("added = %v, want [properties.felt properties.magnitude]", added)
	}
}
