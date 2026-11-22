package adapters

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
)

// maxResponseBytes caps what one source may send. The largest today, USGS's
// month of earthquakes, is about 7 MB; the cap bounds memory on a small
// instance without coming near a real response.
const maxResponseBytes = 32 << 20

// StatusError is an upstream answer other than 200 OK.
type StatusError struct{ Code int }

func (e *StatusError) Error() string { return fmt.Sprintf("unexpected status %d", e.Code) }

// ErrUnreadable marks a response that arrived but could not be decoded.
var ErrUnreadable = errors.New("unreadable response")

// fetchList sends req and decodes the JSON array under key in the response
// object one record at a time, reading at most maxBody bytes. A record that
// does not decode into T is skipped and logged, so one odd record cannot
// drop the whole source; a list with records but none readable is
// ErrUnreadable, so an unreadable source is never reported as calm. A 204 No
// Content is an empty list.
func fetchList[T any](client *http.Client, req *http.Request, source, key string, maxBody int64) ([]T, error) {
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s: request failed: %w", source, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %w", source, &StatusError{Code: resp.StatusCode})
	}

	var envelope map[string]json.RawMessage
	if err := json.NewDecoder(http.MaxBytesReader(nil, resp.Body, maxBody)).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("%s: decode response: %w: %w", source, ErrUnreadable, err)
	}
	list, ok := envelope[key]
	if !ok {
		return nil, fmt.Errorf("%s: decode response: %w: no %q list", source, ErrUnreadable, key)
	}
	var raw []json.RawMessage
	if err := json.Unmarshal(list, &raw); err != nil {
		return nil, fmt.Errorf("%s: decode response: %w: %w", source, ErrUnreadable, err)
	}

	items := make([]T, 0, len(raw))
	skipped := 0
	var firstErr error
	for _, r := range raw {
		var item T
		if err := json.Unmarshal(r, &item); err != nil {
			skipped++
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		items = append(items, item)
	}
	if skipped > 0 {
		slog.Warn("skipped records that could not be read",
			"source", source, "skipped", skipped, "kept", len(items), "first_error", firstErr.Error())
		if len(items) == 0 {
			return nil, fmt.Errorf("%s: decode response: %w: none of %d records could be read: %w",
				source, ErrUnreadable, skipped, firstErr)
		}
	}
	return items, nil
}
