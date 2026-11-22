package httpx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
)

// trustedHeader is the header production trusts; the tests use the same one.
const trustedHeader = "Do-Connecting-IP"

// spoofed sets every client-address header a visitor can send, with values
// that must never become the key.
func spoofed(r *http.Request) {
	r.Header.Set("X-Forwarded-For", "203.0.113.4")
	r.Header.Set("True-Client-IP", "203.0.113.2")
	r.Header.Set("X-Real-IP", "203.0.113.3")
	r.Header.Set("Forwarded", "for=203.0.113.6")
	r.Header.Set("X-Client-IP", "203.0.113.7")
}

// keyFor runs r through the trusted-header middleware and returns its rate-limit key.
func keyFor(t *testing.T, r *http.Request) string {
	t.Helper()
	var key string
	h := middleware.ClientIPFromHeader(trustedHeader)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		var err error
		if key, err = RateLimitKey(r); err != nil {
			t.Fatalf("RateLimitKey: %v", err)
		}
	}))
	h.ServeHTTP(httptest.NewRecorder(), r)
	return key
}

func TestRateLimitKeyComesFromTheTrustedHeaderOnly(t *testing.T) {
	cases := []struct {
		name, header, want string
	}{
		{"ipv4", "198.51.100.7", "198.51.100.7"},
		{"ipv6 grouped by /64", "2001:db8:1:2:3:4:5:6", "2001:db8:1:2::"},
		{"same /64, other host", "2001:db8:1:2:ffff:ffff:ffff:ffff", "2001:db8:1:2::"},
		{"v4-mapped ipv6 folds to ipv4", "::ffff:198.51.100.7", "198.51.100.7"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/api/v1/events", nil)
			spoofed(r)
			r.Header.Set(trustedHeader, c.header)
			if got := keyFor(t, r); got != c.want {
				t.Errorf("key = %q, want %q", got, c.want)
			}
		})
	}
}

func TestRateLimitKeyWithoutTheTrustedHeaderIsEmptyAndWarns(t *testing.T) {
	warnedNoClientIP.Store(false)
	t.Cleanup(func() { warnedNoClientIP.Store(false) })

	r := httptest.NewRequest(http.MethodGet, "/api/v1/events", nil)
	spoofed(r)
	if got := keyFor(t, r); got != "" {
		t.Errorf("key = %q, want empty: spoofable headers must not be used", got)
	}
	if !warnedNoClientIP.Load() {
		t.Error("missing client address was not logged")
	}

	r = httptest.NewRequest(http.MethodGet, "/api/v1/events", nil)
	r.Header.Set(trustedHeader, "not an address")
	if got := keyFor(t, r); got != "" {
		t.Errorf("key = %q, want empty for an unparseable header", got)
	}
}

func TestRateLimitIgnoresSpoofedHeadersAndAnswersInTheAPIShape(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	h := middleware.ClientIPFromHeader(trustedHeader)(
		httprate.LimitBy(1, time.Minute, RateLimitKey, httprate.WithLimitHandler(TooManyRequests))(ok))

	do := func(client string, spoof bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodGet, "/api/v1/events", nil)
		r.Header.Set(trustedHeader, client)
		if spoof {
			spoofed(r)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		return rec
	}

	if rec := do("198.51.100.7", false); rec.Code != http.StatusOK {
		t.Fatalf("first request: status %d, want 200", rec.Code)
	}

	rec := do("198.51.100.7", true)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("second request with spoofed headers: status %d, want 429", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("429 body is not JSON: %v: %q", err, rec.Body.String())
	}
	if body["error"] != "rate limit exceeded" || len(body) != 1 {
		t.Errorf("429 body = %v, want {\"error\": \"rate limit exceeded\"}", body)
	}

	if rec := do("198.51.100.8", false); rec.Code != http.StatusOK {
		t.Errorf("another client: status %d, want 200", rec.Code)
	}
}
