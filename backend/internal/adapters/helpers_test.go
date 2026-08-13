package adapters

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/KOHANTIC/SentryAtlas/backend/internal/models"
)

// reqCapture records the query and headers of the last request a test server
// received. Mutex-guarded because the server handler runs on its own
// goroutine and the race detector cannot see the HTTP round-trip ordering.
type reqCapture struct {
	mu     sync.Mutex
	query  url.Values
	header http.Header
}

func (c *reqCapture) record(r *http.Request) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.query = r.URL.Query()
	c.header = r.Header.Clone()
}

func (c *reqCapture) Query() url.Values {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.query
}

func (c *reqCapture) Header() http.Header {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.header
}

// serveFixture starts a test server that answers every request with the named
// file from testdata/. If capture is non-nil, each request is recorded there.
func serveFixture(t *testing.T, name string, capture *reqCapture) *httptest.Server {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if capture != nil {
			capture.record(r)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// serveRaw starts a test server that answers every request with the given
// status and body.
func serveRaw(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// eventByID finds an event by ID or fails the test.
func eventByID(t *testing.T, events []models.Event, id string) models.Event {
	t.Helper()
	for _, e := range events {
		if e.ID == id {
			return e
		}
	}
	t.Fatalf("event %q not found; got IDs: %v", id, eventIDs(events))
	return models.Event{}
}

func eventIDs(events []models.Event) []string {
	ids := make([]string, 0, len(events))
	for _, e := range events {
		ids = append(ids, e.ID)
	}
	return ids
}
