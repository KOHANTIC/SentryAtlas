package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/KOHANTIC/SentryAtlas/backend/internal/store"
)

type healthSource struct {
	Source string `json:"source"`
	OK     bool   `json:"ok"`
	Stale  bool   `json:"stale"`
	// AgeSeconds is how old the copy in use is; absent if the source has
	// never answered.
	AgeSeconds *int64 `json:"age_seconds,omitempty"`
	Count      int    `json:"count"`
	Error      string `json:"error,omitempty"`
}

// Health answers 503 "starting" until every source has been polled once, so
// a new deploy takes traffic only once it has data, then 200 "ok"; both
// report each source's state and the age of its copy.
func Health(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		now := time.Now()
		body := struct {
			Status  string         `json:"status"`
			Sources []healthSource `json:"sources"`
		}{Status: "ok"}
		for _, c := range st.Read(now) {
			s := healthSource{Source: c.Status.Source, OK: c.Status.OK, Stale: c.Status.Stale, Count: c.Status.Count, Error: c.Status.Error}
			if at := c.Status.FetchedAt; at != nil {
				age := int64(now.Sub(*at).Seconds())
				s.AgeSeconds = &age
			}
			body.Sources = append(body.Sources, s)
		}
		code := http.StatusOK
		if !st.Ready() {
			body.Status, code = "starting", http.StatusServiceUnavailable
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(code)
		// The status is already sent; a failed write means the probe has gone.
		_ = json.NewEncoder(w).Encode(body)
	}
}
