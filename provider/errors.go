package provider

import (
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// RateLimitError is returned when the API responds with HTTP 429.
type RateLimitError struct {
	StatusCode int
	Message    string
	RetryAfter time.Duration // from Retry-After header, or 0 if absent
}

func (e *RateLimitError) Error() string {
	if e.RetryAfter > 0 {
		return fmt.Sprintf("rate limited (HTTP %d): retry after %s", e.StatusCode, e.RetryAfter)
	}
	return fmt.Sprintf("rate limited (HTTP %d): %s", e.StatusCode, e.Message)
}

// ParseRetryAfter reads the Retry-After header from an HTTP response.
// It supports both delay-seconds and HTTP-date formats.
// Returns 0 if the header is missing or unparseable.
func ParseRetryAfter(resp *http.Response) time.Duration {
	val := resp.Header.Get("Retry-After")
	if val == "" {
		return 0
	}

	// Try as seconds first (most common for API rate limits)
	if seconds, err := strconv.Atoi(val); err == nil {
		return time.Duration(seconds) * time.Second
	}

	// Try as HTTP-date (RFC 1123)
	if t, err := time.Parse(time.RFC1123, val); err == nil {
		d := time.Until(t)
		if d > 0 {
			return d
		}
		return 0
	}

	return 0
}
