package provider

import (
	"io"
	"strings"
	"testing"
)

func TestSSEReaderBasic(t *testing.T) {
	input := "data: hello\n\ndata: world\n\n"
	r := NewSSEReader(strings.NewReader(input))

	ev, err := r.Next()
	if err != nil {
		t.Fatal(err)
	}
	if ev.Data != "hello" {
		t.Fatalf("data = %q, want %q", ev.Data, "hello")
	}

	ev, err = r.Next()
	if err != nil {
		t.Fatal(err)
	}
	if ev.Data != "world" {
		t.Fatalf("data = %q, want %q", ev.Data, "world")
	}

	_, err = r.Next()
	if err != io.EOF {
		t.Fatalf("expected EOF, got %v", err)
	}
}

func TestSSEReaderEventField(t *testing.T) {
	input := "event: message\ndata: {\"text\":\"hi\"}\n\n"
	r := NewSSEReader(strings.NewReader(input))

	ev, err := r.Next()
	if err != nil {
		t.Fatal(err)
	}
	if ev.Event != "message" {
		t.Fatalf("event = %q, want %q", ev.Event, "message")
	}
	if ev.Data != `{"text":"hi"}` {
		t.Fatalf("data = %q", ev.Data)
	}
}

func TestSSEReaderMultiDataLines(t *testing.T) {
	// Multiple data: lines get joined with newlines
	input := "data: line1\ndata: line2\ndata: line3\n\n"
	r := NewSSEReader(strings.NewReader(input))

	ev, err := r.Next()
	if err != nil {
		t.Fatal(err)
	}
	if ev.Data != "line1\nline2\nline3" {
		t.Fatalf("data = %q, want %q", ev.Data, "line1\nline2\nline3")
	}
}

func TestSSEReaderComment(t *testing.T) {
	// Lines starting with : are comments
	input := ": this is a comment\ndata: hello\n\n"
	r := NewSSEReader(strings.NewReader(input))

	ev, err := r.Next()
	if err != nil {
		t.Fatal(err)
	}
	if ev.Data != "hello" {
		t.Fatalf("data = %q, want %q", ev.Data, "hello")
	}
}

func TestSSEReaderNoSpaceAfterColon(t *testing.T) {
	input := "data:nospace\n\n"
	r := NewSSEReader(strings.NewReader(input))

	ev, err := r.Next()
	if err != nil {
		t.Fatal(err)
	}
	if ev.Data != "nospace" {
		t.Fatalf("data = %q, want %q", ev.Data, "nospace")
	}
}

func TestSSEReaderDONE(t *testing.T) {
	// OpenAI sends "data: [DONE]" as the final event
	input := "data: {\"choices\":[]}\n\ndata: [DONE]\n\n"
	r := NewSSEReader(strings.NewReader(input))

	ev, err := r.Next()
	if err != nil {
		t.Fatal(err)
	}
	if ev.Data != `{"choices":[]}` {
		t.Fatalf("first event data = %q", ev.Data)
	}

	ev, err = r.Next()
	if err != nil {
		t.Fatal(err)
	}
	if ev.Data != "[DONE]" {
		t.Fatalf("second event data = %q, want [DONE]", ev.Data)
	}
}

func TestSSEReaderEmptyStream(t *testing.T) {
	r := NewSSEReader(strings.NewReader(""))
	_, err := r.Next()
	if err != io.EOF {
		t.Fatalf("expected EOF, got %v", err)
	}
}

func TestSSEReaderMultipleEmptyLines(t *testing.T) {
	// Multiple empty lines between events
	input := "\n\n\ndata: hello\n\n\n\ndata: world\n\n"
	r := NewSSEReader(strings.NewReader(input))

	ev, err := r.Next()
	if err != nil {
		t.Fatal(err)
	}
	if ev.Data != "hello" {
		t.Fatalf("data = %q, want %q", ev.Data, "hello")
	}

	ev, err = r.Next()
	if err != nil {
		t.Fatal(err)
	}
	if ev.Data != "world" {
		t.Fatalf("data = %q, want %q", ev.Data, "world")
	}
}

func TestSSEReaderNoTrailingNewline(t *testing.T) {
	// Data at end of stream with no trailing empty line
	input := "data: last"
	r := NewSSEReader(strings.NewReader(input))

	ev, err := r.Next()
	if err != nil {
		t.Fatal(err)
	}
	if ev.Data != "last" {
		t.Fatalf("data = %q, want %q", ev.Data, "last")
	}
}
