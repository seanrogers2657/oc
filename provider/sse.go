package provider

import (
	"bufio"
	"io"
	"strings"
)

// SSEEvent represents a single Server-Sent Event.
type SSEEvent struct {
	Event string // the "event:" field (empty if not set)
	Data  string // the "data:" field(s) joined by newlines
}

// SSEReader reads Server-Sent Events from an io.Reader.
// It yields SSEEvents on each empty-line boundary per the SSE spec.
type SSEReader struct {
	scanner *bufio.Scanner
}

// NewSSEReader creates a new SSE reader.
func NewSSEReader(r io.Reader) *SSEReader {
	return &SSEReader{scanner: bufio.NewScanner(r)}
}

// Next reads the next SSE event. Returns io.EOF when the stream ends.
func (r *SSEReader) Next() (SSEEvent, error) {
	var ev SSEEvent
	var dataLines []string
	hasData := false

	for r.scanner.Scan() {
		line := r.scanner.Text()

		// Empty line = event boundary
		if line == "" {
			if hasData {
				ev.Data = strings.Join(dataLines, "\n")
				return ev, nil
			}
			continue
		}

		// Comment lines (starting with ':') are ignored per SSE spec
		if strings.HasPrefix(line, ":") {
			continue
		}

		// Parse "field: value" or "field:value"
		field, value, _ := strings.Cut(line, ":")
		value = strings.TrimPrefix(value, " ") // single leading space is stripped per spec

		switch field {
		case "event":
			ev.Event = value
		case "data":
			dataLines = append(dataLines, value)
			hasData = true
		case "id", "retry":
			// Acknowledged but not used
		}
	}

	if err := r.scanner.Err(); err != nil {
		return SSEEvent{}, err
	}

	// End of stream -- if we have accumulated data, yield it
	if hasData {
		ev.Data = strings.Join(dataLines, "\n")
		return ev, nil
	}

	return SSEEvent{}, io.EOF
}
