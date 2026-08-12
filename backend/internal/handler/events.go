package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/KOHANTIC/SentryAtlas/backend/internal/adapters"
	"github.com/KOHANTIC/SentryAtlas/backend/internal/models"
	"github.com/KOHANTIC/SentryAtlas/backend/internal/service"
)

type EventsHandler struct {
	service *service.EventsService
}

func NewEventsHandler(svc *service.EventsService) *EventsHandler {
	return &EventsHandler{service: svc}
}

func (h *EventsHandler) GetEvents(w http.ResponseWriter, r *http.Request) {
	params, format, err := parseQueryParams(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if format == "sse" {
		h.streamEvents(w, r, params)
		return
	}

	events, sources, err := h.service.GetEvents(r.Context(), params)
	if err != nil {
		if errors.Is(err, service.ErrAllSourcesFailed) {
			writeError(w, http.StatusBadGateway, "all upstream sources failed")
		} else {
			writeError(w, http.StatusInternalServerError, "failed to fetch events")
		}
		return
	}

	var data []byte
	switch format {
	case "json":
		data, err = models.MarshalEventsJSON(events, sources)
		w.Header().Set("Content-Type", "application/json")
	default:
		data, err = models.MarshalGeoJSON(events, sources)
		w.Header().Set("Content-Type", "application/geo+json")
	}

	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal response")
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func (h *EventsHandler) streamEvents(w http.ResponseWriter, r *http.Request, params adapters.FetchParams) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := make(chan service.StreamBatch, 4)
	go func() {
		h.service.StreamEvents(r.Context(), params, ch)
		close(ch)
	}()

	total := 0
	sources := make([]models.SourceStatus, 0, 4)
	for batch := range ch {
		if batch.Err != nil {
			sources = append(sources, models.SourceStatus{
				Source: batch.Source,
				Error:  batch.Err.Error(),
			})
			continue
		}
		sources = append(sources, models.SourceStatus{
			Source: batch.Source,
			OK:     true,
		})
		if len(batch.Events) == 0 {
			continue
		}
		total += len(batch.Events)
		data, err := models.MarshalGeoJSON(batch.Events, nil)
		if err != nil {
			continue
		}
		fmt.Fprintf(w, "event: features\ndata: %s\n\n", data)
		flusher.Flush()
	}

	doneData, _ := json.Marshal(map[string]any{"total": total, "sources": sources})
	fmt.Fprintf(w, "event: done\ndata: %s\n\n", doneData)
	flusher.Flush()
}

func parseQueryParams(r *http.Request) (adapters.FetchParams, string, error) {
	q := r.URL.Query()
	var params adapters.FetchParams

	if types := q.Get("types"); types != "" {
		// Validated and deduplicated: unknown values are rejected rather
		// than passed through, because raw user input would otherwise mint
		// unbounded cache keys (each triggering a fresh upstream fetch).
		seen := make(map[string]struct{})
		for _, t := range strings.Split(types, ",") {
			t = strings.TrimSpace(t)
			if !models.IsValidEventType(t) {
				return params, "", fmt.Errorf("invalid type %q: valid types are %s", t, strings.Join(models.EventTypes, ", "))
			}
			if _, dup := seen[t]; dup {
				continue
			}
			seen[t] = struct{}{}
			params.Types = append(params.Types, t)
		}
	}

	if bboxStr := q.Get("bbox"); bboxStr != "" {
		bbox, err := parseBBox(bboxStr)
		if err != nil {
			return params, "", fmt.Errorf("invalid bbox: %w", err)
		}
		params.BBox = bbox
	}

	if sinceStr := q.Get("since"); sinceStr != "" {
		t, err := time.Parse(time.RFC3339, sinceStr)
		if err != nil {
			t, err = time.Parse("2006-01-02", sinceStr)
			if err != nil {
				return params, "", fmt.Errorf("invalid since: expected RFC3339 or YYYY-MM-DD")
			}
		}
		params.Since = t
	}

	if limitStr := q.Get("limit"); limitStr != "" {
		limit, err := strconv.Atoi(limitStr)
		if err != nil || limit < 1 {
			return params, "", fmt.Errorf("invalid limit: must be a positive integer")
		}
		if limit > 1000 {
			limit = 1000
		}
		params.Limit = limit
	}

	format := q.Get("format")
	if format != "" && format != "geojson" && format != "json" && format != "sse" {
		return params, "", fmt.Errorf("invalid format: must be 'geojson', 'json', or 'sse'")
	}
	if format == "" {
		format = "geojson"
	}

	return params, format, nil
}

func parseBBox(s string) (*adapters.BBox, error) {
	parts := strings.Split(s, ",")
	if len(parts) != 4 {
		return nil, fmt.Errorf("expected 4 values: minLon,minLat,maxLon,maxLat")
	}

	vals := make([]float64, 4)
	for i, p := range parts {
		v, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if err != nil {
			return nil, fmt.Errorf("value %d is not a valid number", i+1)
		}
		vals[i] = v
	}

	return &adapters.BBox{
		MinLon: vals[0],
		MinLat: vals[1],
		MaxLon: vals[2],
		MaxLat: vals[3],
	}, nil
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	// Encoded, not formatted into a template: a message containing a quote
	// must not be able to break out of the JSON string.
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
