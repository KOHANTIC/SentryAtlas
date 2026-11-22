package adapters

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

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
		{"panic", fmt.Errorf("usgs: %w: index out of range", ErrInternal), "failed inside SentryAtlas"},
		{"deadline", fmt.Errorf("usgs: request failed: %w", &url.Error{Op: "Get", URL: addr, Err: context.DeadlineExceeded}), "timed out"},
		{"client timeout", fmt.Errorf("usgs: request failed: %w", &url.Error{Op: "Get", URL: addr, Err: timeoutError{}}), "timed out"},
		{"status", fmt.Errorf("usgs: %w", &StatusError{Code: 503}), "answered HTTP 503"},
		{"too large", fmt.Errorf("usgs: decode response: %w: %w", ErrUnreadable, &http.MaxBytesError{Limit: 64}), "sent more data than SentryAtlas accepts"},
		{"unreadable", fmt.Errorf("usgs: decode response: %w: invalid character '<'", ErrUnreadable), "sent a response SentryAtlas could not read"},
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
