// Package httpx holds the API's own HTTP middleware and handlers that sit
// outside any one endpoint.
package httpx

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
)

// RequestLog writes one line per request to logger: method, path, status,
// bytes, duration and request id. It leaves out the query string, which holds
// a visitor's map view, and the client address, and skips /health so the
// platform's probes do not fill the log.
func RequestLog(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/health" {
				next.ServeHTTP(w, r)
				return
			}
			// The wrapper keeps Flush and Unwrap, which the SSE stream needs.
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			start := time.Now()
			next.ServeHTTP(ww, r)

			status := ww.Status()
			if status == 0 {
				// Nothing written: net/http sends 200.
				status = http.StatusOK
			}
			logger.LogAttrs(r.Context(), slog.LevelInfo, "request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", status),
				slog.Int("bytes", ww.BytesWritten()),
				slog.Int64("duration_ms", time.Since(start).Milliseconds()),
				slog.String("request_id", middleware.GetReqID(r.Context())),
			)
		})
	}
}

// warnedNoClientIP keeps the missing-address warning to one line per process.
var warnedNoClientIP atomic.Bool

// RateLimitKey keys the rate limiter on the client address stored by one of
// chi's ClientIPFrom* middlewares, with IPv6 grouped by /64 so one host cannot
// step through its own prefix to reset its limit.
//
// A request with no stored address gets the empty key, which every such
// request shares: in production that means the trusted header is missing,
// so it is logged once to make the misconfiguration visible.
func RateLimitKey(r *http.Request) (string, error) {
	ip := middleware.GetClientIP(r.Context())
	if ip == "" && warnedNoClientIP.CompareAndSwap(false, true) {
		slog.Warn("request without a client address: it shares one rate-limit bucket with all such requests; check CLIENT_IP_HEADER")
	}
	return httprate.CanonicalizeIP(ip), nil
}

// TooManyRequests answers a request over the rate limit in the API's error shape.
func TooManyRequests(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusTooManyRequests)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "rate limit exceeded"})
}

// SplitCORS sends requests whose path starts with prefix through private and
// all others through public. CORS must answer preflight requests before any
// route matches, so the two policies cannot sit on route groups.
func SplitCORS(prefix string, private, public func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		priv, pub := private(next), public(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, prefix) {
				priv.ServeHTTP(w, r)
				return
			}
			pub.ServeHTTP(w, r)
		})
	}
}
