package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/KOHANTIC/SentryAtlas/backend/internal/event"
	"github.com/KOHANTIC/SentryAtlas/backend/internal/store"
)

type healthBody struct {
	Status  string         `json:"status"`
	Sources []healthSource `json:"sources"`
}

func getHealth(t *testing.T, st *store.Store) (int, healthBody) {
	t.Helper()
	rec := httptest.NewRecorder()
	Health(st)(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	var b healthBody
	if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", cc)
	}
	return rec.Code, b
}

func TestHealthIsStartingUntilEverySourceIsTried(t *testing.T) {
	t.Parallel()
	st := store.New(store.Source{Name: "usgs"}, store.Source{Name: "noaa"})
	if code, b := getHealth(t, st); code != http.StatusServiceUnavailable || b.Status != "starting" || len(b.Sources) != 2 {
		t.Errorf("before any poll: %d %+v, want 503 starting with both sources", code, b)
	}

	if err := st.Put("usgs", []event.Event{{ID: "usgs-1"}}, time.Now().Add(-90*time.Second)); err != nil {
		t.Fatal(err)
	}
	if code, _ := getHealth(t, st); code != http.StatusServiceUnavailable {
		t.Errorf("with noaa untried: %d, want 503", code)
	}

	st.Fail("noaa", "timed out")
	code, b := getHealth(t, st)
	if code != http.StatusOK || b.Status != "ok" {
		t.Fatalf("both tried: %d %+v, want 200 ok: a failed source is reported, not a reason to refuse traffic", code, b)
	}
	usgs, noaa := b.Sources[0], b.Sources[1]
	if !usgs.OK || usgs.Count != 1 || usgs.AgeSeconds == nil || *usgs.AgeSeconds < 89 || *usgs.AgeSeconds > 120 {
		t.Errorf("usgs = %+v, want ok, 1 event, about 90 s old", usgs)
	}
	if noaa.OK || noaa.Error != "timed out" || noaa.AgeSeconds != nil {
		t.Errorf("noaa = %+v, want its failure and no age, since it never answered", noaa)
	}
}
