package handler

import (
	"encoding/json"
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

	events, err := h.service.GetEvents(r.Context(), params)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch events")
		return
	}

	var data []byte
	switch format {
	case "json":
		data, err = models.MarshalEventsJSON(events)
		w.Header().Set("Content-Type", "application/json")
	default:
		data, err = models.MarshalGeoJSON(events)
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

	ch := make(chan []models.Event, 4)
	go func() {
		h.service.StreamEvents(r.Context(), params, ch)
		close(ch)
	}()

	total := 0
	for batch := range ch {
		total += len(batch)
		data, err := models.MarshalGeoJSON(batch)
		if err != nil {
			continue
		}
		fmt.Fprintf(w, "event: features\ndata: %s\n\n", data)
		flusher.Flush()
	}

	doneData, _ := json.Marshal(map[string]int{"total": total})
	fmt.Fprintf(w, "event: done\ndata: %s\n\n", doneData)
	flusher.Flush()
}

func parseQueryParams(r *http.Request) (adapters.FetchParams, string, error) {
	q := r.URL.Query()
	var params adapters.FetchParams

	if types := q.Get("types"); types != "" {
		params.Types = strings.Split(types, ",")
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
	fmt.Fprintf(w, `{"error":"%s"}`, message)
}
