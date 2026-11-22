package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCallerSite(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		origin, referer, want string
	}{
		{"", "", "none"},
		{"https://sentryatlas.com", "", "sentryatlas"},
		{"https://map.sentryatlas.com", "", "sentryatlas"},
		{"", "https://map.sentryatlas.com/?types=flood", "sentryatlas"},
		{"", "https://example.org/dashboard", "other"},
		{"https://notsentryatlas.com", "", "other"},
		{"http://localhost:3000", "", "other"},
		{"%%%", "", "other"},
	} {
		r := httptest.NewRequest(http.MethodGet, "/api/v1/events", nil)
		if c.origin != "" {
			r.Header.Set("Origin", c.origin)
		}
		if c.referer != "" {
			r.Header.Set("Referer", c.referer)
		}
		if got := callerSite(r); got != c.want {
			t.Errorf("Origin %q, Referer %q: %q, want %q", c.origin, c.referer, got, c.want)
		}
	}
}

func TestV1CallsAreCountedByFormatAndSite(t *testing.T) {
	t.Parallel()
	h := newTestHandler(t, &fakeAdapter{source: "alpha", events: makeEvents(1, "alpha")})
	get := func(query, origin string) {
		r := httptest.NewRequest(http.MethodGet, "/api/v1/events"+query, nil)
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		h.GetEvents(httptest.NewRecorder(), r)
	}
	get("", "https://map.sentryatlas.com")
	get("?format=sse", "https://map.sentryatlas.com")
	get("?format=json", "")
	get("?format=geojson", "https://example.org")
	get("?format=xml", "") // a 400 is not a call

	byFormat, bySite := h.calls.take()
	if byFormat["geojson"] != 2 || byFormat["json"] != 1 || byFormat["sse"] != 1 || len(byFormat) != 3 {
		t.Errorf("by format = %v, want geojson 2, json 1, sse 1", byFormat)
	}
	if bySite["sentryatlas"] != 2 || bySite["other"] != 1 || bySite["none"] != 1 {
		t.Errorf("by site = %v, want sentryatlas 2, other 1, none 1", bySite)
	}
	if f, s := h.calls.take(); len(f) != 0 || len(s) != 0 {
		t.Errorf("after take: %v %v, want the counts started again", f, s)
	}
}

func TestReportCallsStopsWithItsContext(t *testing.T) {
	t.Parallel()
	h := newTestHandler(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { h.ReportCalls(ctx, time.Millisecond); close(done) }()
	time.Sleep(5 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("ReportCalls still running after its context ended")
	}
}
