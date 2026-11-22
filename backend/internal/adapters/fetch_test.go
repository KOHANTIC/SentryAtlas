package adapters

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// guardCase is one source with one readable record and one of the wrong
// shape, as each source could plausibly send it.
type guardCase struct {
	source string
	key    string // the list in the response object
	good   string
	bad    string
	// build makes the adapter against srv; maxBody > 0 lowers its size limit.
	build func(t *testing.T, srv *httptest.Server, maxBody int64) Adapter
}

var guardCases = []guardCase{
	{
		source: "usgs", key: "features",
		good: `{"id":"us1","geometry":{"type":"Point","coordinates":[1,2,3]},"properties":{"mag":4.5,"time":1786320000000,"updated":1786320000000,"title":"M 4.5","place":"p","url":"u"}}`,
		bad:  `{"id":"us2","geometry":{"type":"Point","coordinates":[1,2]},"properties":{"mag":"4.5"}}`,
		build: func(t *testing.T, srv *httptest.Server, maxBody int64) Adapter {
			a := newTestUSGS(t, srv)
			if maxBody > 0 {
				a.maxBody = maxBody
			}
			return a
		},
	},
	{
		source: "eonet", key: "events",
		good: `{"id":"E1","title":"t","categories":[{"id":"wildfires"}],"geometry":[{"date":"2026-08-08T00:00:00Z","type":"Point","coordinates":[1,2]}]}`,
		bad:  `{"id":"E2","title":"t","categories":[{"id":"wildfires"}],"geometry":[{"date":"2026-08-08T00:00:00Z","type":"Polygon","coordinates":[[[1,2],[3,4],[5,6],[1,2]]]}]}`,
		build: func(t *testing.T, srv *httptest.Server, maxBody int64) Adapter {
			a := newTestEONET(t, srv)
			if maxBody > 0 {
				a.maxBody = maxBody
			}
			return a
		},
	},
	{
		source: "noaa", key: "features",
		good: `{"geometry":null,"properties":{"id":"n1","event":"Flood Warning","headline":"h","sent":"2026-08-08T00:00:00Z"}}`,
		bad:  `{"geometry":null,"properties":{"id":"n2","event":5}}`,
		build: func(t *testing.T, srv *httptest.Server, maxBody int64) Adapter {
			a := newTestNOAA(t, srv)
			if maxBody > 0 {
				a.maxBody = maxBody
			}
			return a
		},
	},
	{
		source: "gdacs", key: "features",
		good: `{"geometry":{"type":"Point","coordinates":[1,2]},"properties":{"eventtype":"EQ","eventid":1,"name":"n","alertlevel":"Green","fromdate":"2026-08-08T00:00:00"}}`,
		bad:  `{"geometry":{"type":"Point","coordinates":[1,2]},"properties":{"eventtype":"EQ","eventid":2,"severity":"high"}}`,
		build: func(t *testing.T, srv *httptest.Server, maxBody int64) Adapter {
			a := newTestGDACS(t, srv)
			if maxBody > 0 {
				a.maxBody = maxBody
			}
			return a
		},
	},
}

// list wraps records in the source's response object.
func (c guardCase) list(records ...string) string {
	return `{"` + c.key + `":[` + strings.Join(records, ",") + `]}`
}

func TestOneUnreadableRecordKeepsTheRest(t *testing.T) {
	t.Parallel()
	for _, c := range guardCases {
		t.Run(c.source, func(t *testing.T) {
			t.Parallel()
			a := c.build(t, serveRaw(t, http.StatusOK, c.list(c.bad, c.good)), 0)
			events, err := a.FetchEvents(context.Background())
			if err != nil {
				t.Fatalf("FetchEvents: %v, want the readable record kept", err)
			}
			if len(events) != 1 {
				t.Errorf("got %d events, want 1: the readable record, without the other", len(events))
			}
		})
	}
}

func TestNoReadableRecordIsUnreadable(t *testing.T) {
	t.Parallel()
	for _, c := range guardCases {
		t.Run(c.source, func(t *testing.T) {
			t.Parallel()
			for name, body := range map[string]string{
				"every record bad": c.list(c.bad, c.bad),
				"no list":          `{"something_else":[]}`,
				"not json":         `<html>maintenance</html>`,
			} {
				a := c.build(t, serveRaw(t, http.StatusOK, body), 0)
				_, err := a.FetchEvents(context.Background())
				if !errors.Is(err, ErrUnreadable) {
					t.Errorf("%s: err = %v, want ErrUnreadable", name, err)
				}
			}
		})
	}
}

func TestAnEmptyListIsNotAnError(t *testing.T) {
	t.Parallel()
	for _, c := range guardCases {
		t.Run(c.source, func(t *testing.T) {
			t.Parallel()
			a := c.build(t, serveRaw(t, http.StatusOK, c.list()), 0)
			events, err := a.FetchEvents(context.Background())
			if err != nil || len(events) != 0 {
				t.Errorf("got %d events, err %v; want none and no error: a calm source is not a failure", len(events), err)
			}
		})
	}
}

func TestAnOversizedResponseIsRefused(t *testing.T) {
	t.Parallel()
	for _, c := range guardCases {
		t.Run(c.source, func(t *testing.T) {
			t.Parallel()
			a := c.build(t, serveRaw(t, http.StatusOK, c.list(c.good, c.good, c.good)), 64)
			_, err := a.FetchEvents(context.Background())
			var tooLarge *http.MaxBytesError
			if !errors.As(err, &tooLarge) {
				t.Errorf("err = %v, want *http.MaxBytesError", err)
			}
		})
	}
}

func TestANon200IsAStatusError(t *testing.T) {
	t.Parallel()
	for _, c := range guardCases {
		t.Run(c.source, func(t *testing.T) {
			t.Parallel()
			a := c.build(t, serveRaw(t, http.StatusServiceUnavailable, "busy"), 0)
			_, err := a.FetchEvents(context.Background())
			var status *StatusError
			if !errors.As(err, &status) || status.Code != http.StatusServiceUnavailable {
				t.Errorf("err = %v, want a StatusError with 503", err)
			}
		})
	}
}
