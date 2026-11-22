package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/KOHANTIC/SentryAtlas/backend/internal/adapters"
	"github.com/KOHANTIC/SentryAtlas/backend/internal/models"
)

// panicAdapter panics on every fetch, as an adapter bug would, such as the
// NWS centroid's old out-of-range index.
type panicAdapter struct{ source string }

func (p panicAdapter) FetchEvents(context.Context) ([]models.Event, error) {
	panic("index out of range [1] with length 1")
}
func (p panicAdapter) Source() string           { return p.source }
func (p panicAdapter) SupportedTypes() []string { return []string{"flood"} }

func TestAPanickingAdapterIsAFailedSource(t *testing.T) {
	t.Parallel()
	good := &fakeAdapter{source: "good", types: []string{"earthquake"}, events: []models.Event{
		evt("g-1", "earthquake", baseTime, 1, 1),
	}}
	s := newTestService(t, panicAdapter{source: "broken"}, good)

	// Twice: the second request shows the service, and singleflight, still work.
	for i := 1; i <= 2; i++ {
		events, statuses, err := s.GetEvents(context.Background(), adapters.FetchParams{})
		if err != nil {
			t.Fatalf("GetEvents #%d: %v, want the other source's events", i, err)
		}
		if len(events) != 1 {
			t.Errorf("GetEvents #%d: got %d events, want 1", i, len(events))
		}
		if st := statusBySource(t, statuses, "broken"); st.OK || st.Error != "failed inside SentryAtlas" {
			t.Errorf("GetEvents #%d: broken status = %+v, want a failure inside SentryAtlas", i, st)
		}
	}

	batches := collectBatches(t, s, context.Background(), adapters.FetchParams{})
	if b := batchBySource(t, batches, "broken"); b.Err == nil || Reason(b.Err) != "failed inside SentryAtlas" {
		t.Errorf("stream batch for broken = %+v, want a failure inside SentryAtlas", b)
	}
	if b := batchBySource(t, batches, "good"); b.Err != nil || len(b.Events) != 1 {
		t.Errorf("stream batch for good = %+v, want its event", b)
	}
}

// timeoutError is a network error that timed out, as the HTTP client reports
// a request over its Timeout.
type timeoutError struct{}

func (timeoutError) Error() string   { return "Client.Timeout exceeded while awaiting headers" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

func TestReasonIsShortAndHasNoAddress(t *testing.T) {
	t.Parallel()
	const addr = "https://upstream.example/feed?key=1"
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"panic", fmt.Errorf("usgs: %w: index out of range", errInternal), "failed inside SentryAtlas"},
		{"deadline", fmt.Errorf("usgs: request failed: %w", &url.Error{Op: "Get", URL: addr, Err: context.DeadlineExceeded}), "timed out"},
		{"client timeout", fmt.Errorf("usgs: request failed: %w", &url.Error{Op: "Get", URL: addr, Err: timeoutError{}}), "timed out"},
		{"status", fmt.Errorf("usgs: %w", &adapters.StatusError{Code: 503}), "answered HTTP 503"},
		{"too large", fmt.Errorf("usgs: decode response: %w: %w", adapters.ErrUnreadable, &http.MaxBytesError{Limit: 64}), "sent more data than SentryAtlas accepts"},
		{"unreadable", fmt.Errorf("usgs: decode response: %w: invalid character '<'", adapters.ErrUnreadable), "sent a response SentryAtlas could not read"},
		{"connection", fmt.Errorf("usgs: request failed: %w", &url.Error{Op: "Get", URL: addr, Err: errors.New("dial tcp: connection refused")}), "could not be reached"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := Reason(c.err)
			if got != c.want {
				t.Errorf("Reason = %q, want %q", got, c.want)
			}
			if strings.Contains(got, "://") || strings.Contains(got, "upstream.example") {
				t.Errorf("Reason %q leaks the upstream address", got)
			}
		})
	}
}
