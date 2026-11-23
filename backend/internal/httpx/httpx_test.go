package httpx

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"
)

// logged runs one request through RequestID, the trusted-header middleware and
// RequestLog, in production order, and returns what was logged.
func logged(t *testing.T, r *http.Request, next http.HandlerFunc) string {
	t.Helper()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	h := middleware.RequestID(middleware.ClientIPFromHeader(trustedHeader)(RequestLog(logger)(next)))
	h.ServeHTTP(httptest.NewRecorder(), r)
	return buf.String()
}

func TestRequestLogKeepsNoQueryStringOrAddress(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/v1/events?bbox=-122.52,37.70,-122.35,37.83&since=2026-11-01&format=json", nil)
	r.Header.Set(trustedHeader, "198.51.100.7")
	r.RemoteAddr = "100.127.2.246:40724"
	out := logged(t, r, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("12345"))
	})

	if n := strings.Count(out, "\n"); n != 1 {
		t.Fatalf("want one line, got %d: %q", n, out)
	}
	for _, leak := range []string{"bbox", "122.52", "since", "format=", "?", "198.51.100.7", "100.127.2.246"} {
		if strings.Contains(out, leak) {
			t.Errorf("log line contains %q: %s", leak, out)
		}
	}

	var line map[string]any
	if err := json.Unmarshal([]byte(out), &line); err != nil {
		t.Fatalf("log line is not JSON: %v: %q", err, out)
	}
	want := map[string]any{
		"msg":    "request",
		"method": "GET",
		"path":   "/api/v1/events",
		"status": float64(http.StatusTeapot),
		"bytes":  float64(5),
	}
	for k, v := range want {
		if line[k] != v {
			t.Errorf("%s = %v, want %v", k, line[k], v)
		}
	}
	if id, _ := line["request_id"].(string); id == "" {
		t.Error("request_id is empty")
	}
	if _, ok := line["duration_ms"].(float64); !ok {
		t.Errorf("duration_ms missing: %s", out)
	}
}

func TestRequestLogSkipsHealth(t *testing.T) {
	out := logged(t, httptest.NewRequest(http.MethodGet, "/health", nil), func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	if out != "" {
		t.Errorf("/health was logged: %s", out)
	}
}

func TestRequestLogCountsAnUnwrittenResponseAs200(t *testing.T) {
	out := logged(t, httptest.NewRequest(http.MethodGet, "/api/v1/events", nil), func(http.ResponseWriter, *http.Request) {})
	if !strings.Contains(out, `"status":200`) {
		t.Errorf("want status 200 for an empty response: %s", out)
	}
}

func TestRequestLogKeepsTheWriterFlushable(t *testing.T) {
	var flushable bool
	logged(t, httptest.NewRequest(http.MethodGet, "/api/v1/events?format=sse", nil), func(w http.ResponseWriter, _ *http.Request) {
		_, flushable = w.(http.Flusher)
	})
	if !flushable {
		t.Error("the wrapped writer lost http.Flusher, which the SSE stream needs")
	}
}

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

func TestSplitCORS(t *testing.T) {
	public := cors.Handler(cors.Options{AllowedOrigins: []string{"*"}, AllowedMethods: []string{"GET", "OPTIONS"}})
	private := cors.Handler(cors.Options{
		AllowedOrigins: []string{"https://sentryatlas.com"}, AllowedMethods: []string{"POST", "OPTIONS"},
		AllowedHeaders: []string{"Content-Type"},
	})
	h := SplitCORS("/api/v2/waitlist", private, public)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	preflight := func(path, origin, method string) http.Header {
		r := httptest.NewRequest(http.MethodOptions, path, nil)
		r.Header.Set("Origin", origin)
		r.Header.Set("Access-Control-Request-Method", method)
		r.Header.Set("Access-Control-Request-Headers", "Content-Type")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		return rec.Header()
	}
	tests := []struct {
		path, origin, method, wantOrigin string
	}{
		{"/api/v2/waitlist", "https://sentryatlas.com", "POST", "https://sentryatlas.com"},
		{"/api/v2/waitlist/confirm", "https://sentryatlas.com", "POST", "https://sentryatlas.com"},
		{"/api/v2/waitlist", "https://elsewhere.example", "POST", ""},
		{"/api/v1/events", "https://elsewhere.example", "GET", "*"},
		{"/api/v1/events", "https://sentryatlas.com", "POST", ""},
	}
	for _, tt := range tests {
		if got := preflight(tt.path, tt.origin, tt.method).Get("Access-Control-Allow-Origin"); got != tt.wantOrigin {
			t.Errorf("%s %s from %s: allow-origin %q, want %q", tt.method, tt.path, tt.origin, got, tt.wantOrigin)
		}
	}
}
