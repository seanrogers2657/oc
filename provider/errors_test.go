package provider

import (
	"net/http"
	"testing"
	"time"
)

func TestRateLimitErrorMessage(t *testing.T) {
	tests := []struct {
		name string
		err  RateLimitError
		want string
	}{
		{
			name: "with retry-after",
			err:  RateLimitError{StatusCode: 429, Message: "too many requests", RetryAfter: 30 * time.Second},
			want: "rate limited (HTTP 429): retry after 30s",
		},
		{
			name: "without retry-after",
			err:  RateLimitError{StatusCode: 429, Message: "too many requests"},
			want: "rate limited (HTTP 429): too many requests",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseRetryAfter(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   time.Duration
	}{
		{
			name:   "seconds",
			header: "30",
			want:   30 * time.Second,
		},
		{
			name:   "missing header",
			header: "",
			want:   0,
		},
		{
			name:   "unparseable",
			header: "not-a-number",
			want:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{Header: http.Header{}}
			if tt.header != "" {
				resp.Header.Set("Retry-After", tt.header)
			}
			got := parseRetryAfter(resp)
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}

	// Test HTTP-date format separately since it depends on current time
	t.Run("http-date", func(t *testing.T) {
		future := time.Now().Add(10 * time.Second).UTC().Format(time.RFC1123)
		resp := &http.Response{Header: http.Header{}}
		resp.Header.Set("Retry-After", future)
		got := parseRetryAfter(resp)
		// Should be roughly 10 seconds (allow some tolerance)
		if got < 8*time.Second || got > 12*time.Second {
			t.Errorf("expected ~10s, got %v", got)
		}
	})

	t.Run("http-date in the past", func(t *testing.T) {
		past := time.Now().Add(-10 * time.Second).UTC().Format(time.RFC1123)
		resp := &http.Response{Header: http.Header{}}
		resp.Header.Set("Retry-After", past)
		got := parseRetryAfter(resp)
		if got != 0 {
			t.Errorf("expected 0 for past date, got %v", got)
		}
	})
}
