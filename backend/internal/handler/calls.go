package handler

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// callCounts counts v1 calls by format and by the site they come from, to
// learn whether anyone besides the map uses v1. It keeps counts only: no
// address, query or user agent.
type callCounts struct {
	mu       sync.Mutex
	byFormat map[string]int
	bySite   map[string]int
}

func (c *callCounts) add(format string, r *http.Request) {
	site := callerSite(r)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.byFormat == nil {
		c.byFormat, c.bySite = map[string]int{}, map[string]int{}
	}
	c.byFormat[format]++
	c.bySite[site]++
}

// take returns the counts so far and starts again from zero.
func (c *callCounts) take() (byFormat, bySite map[string]int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	byFormat, bySite = c.byFormat, c.bySite
	c.byFormat, c.bySite = nil, nil
	return byFormat, bySite
}

// callerSite says where a call comes from, by its Origin or Referer:
// "sentryatlas" for our own pages, "other" for another site, and "none"
// for a script or server, which sends neither.
func callerSite(r *http.Request) string {
	from := r.Header.Get("Origin")
	if from == "" {
		from = r.Header.Get("Referer")
	}
	if from == "" {
		return "none"
	}
	u, err := url.Parse(from)
	if err != nil {
		return "other"
	}
	host := u.Hostname()
	if host == "sentryatlas.com" || strings.HasSuffix(host, ".sentryatlas.com") {
		return "sentryatlas"
	}
	return "other"
}

// ReportCalls logs the v1 call counts once every interval, then starts them
// again, until ctx ends.
func (h *EventsHandler) ReportCalls(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		byFormat, bySite := h.calls.take()
		slog.Info("v1 calls", "period", every.String(),
			"geojson", byFormat["geojson"], "json", byFormat["json"], "sse", byFormat["sse"],
			"from_sentryatlas", bySite["sentryatlas"], "from_other_sites", bySite["other"], "from_no_site", bySite["none"])
	}
}
