package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"

	"github.com/KOHANTIC/SentryAtlas/backend/internal/adapters"
)

// errInternal marks a fetch that failed inside SentryAtlas, such as a
// recovered panic, as opposed to one the source caused.
var errInternal = errors.New("internal error")

// Reason is the short account of a source's failure that API clients see in
// sources[].error. It names what happened in words, without the upstream
// addresses and Go error text the logs keep; the first match wins.
func Reason(err error) string {
	var status *adapters.StatusError
	var tooLarge *http.MaxBytesError
	var netErr net.Error
	switch {
	case errors.Is(err, errInternal):
		return "failed inside SentryAtlas"
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &netErr) && netErr.Timeout():
		return "timed out"
	case errors.As(err, &status):
		return fmt.Sprintf("answered HTTP %d", status.Code)
	case errors.As(err, &tooLarge):
		return "sent more data than SentryAtlas accepts"
	case errors.Is(err, adapters.ErrUnreadable):
		return "sent a response SentryAtlas could not read"
	default:
		// What remains comes from the HTTP client: connection, DNS and TLS failures.
		return "could not be reached"
	}
}
