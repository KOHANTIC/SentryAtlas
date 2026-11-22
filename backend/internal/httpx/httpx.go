// Package httpx holds the API's own HTTP middleware and handlers that sit
// outside any one endpoint.
package httpx

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync/atomic"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
)

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
