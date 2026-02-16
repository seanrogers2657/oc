package provider

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAnthropicStreamText(t *testing.T) {
	sseData := `event: message_start
data: {"type":"message_start","message":{"usage":{"input_tokens":12}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" world"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":8}}

event: message_stop
data: {"type":"message_stop"}

`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify headers
		if r.Header.Get("x-api-key") != "test-key" {
			t.Errorf("missing x-api-key header")
		}
		if r.Header.Get("anthropic-version") != "2023-06-01" {
			t.Errorf("missing anthropic-version header")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, sseData)
	}))
	defer srv.Close()

	p := NewAnthropic("test-key")
	// Override the URL for testing
	p.client = srv.Client()
	origStream := p.Stream

	// We need to intercept the URL. Create a wrapper.
	_ = origStream
	ch, err := streamWithURL(p, srv.URL+"/v1/messages", ModelConfig{Model: "claude-sonnet-4-20250514"}, []Message{UserMessage("hi")}, nil)
	if err != nil {
		t.Fatal(err)
	}

	events := collectStreamEvents(ch)

	var texts []string
	var doneEvent *StreamEvent

	for i := range events {
		switch events[i].Type {
		case EventText:
			texts = append(texts, events[i].Text)
		case EventDone:
			doneEvent = &events[i]
		}
	}

	if len(texts) != 2 || texts[0] != "Hello" || texts[1] != " world" {
		t.Fatalf("texts = %v", texts)
	}
	if doneEvent == nil {
		t.Fatal("no done event")
	}
	if doneEvent.FinishReason != "end_turn" {
		t.Fatalf("finish_reason = %q", doneEvent.FinishReason)
	}
	if doneEvent.Usage == nil {
		t.Fatal("no usage")
	}
	if doneEvent.Usage.InputTokens != 12 || doneEvent.Usage.OutputTokens != 8 {
		t.Fatalf("usage = %+v", doneEvent.Usage)
	}
}

func TestAnthropicStreamToolUse(t *testing.T) {
	sseData := `event: message_start
data: {"type":"message_start","message":{"usage":{"input_tokens":20}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu_123","name":"bash"}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"command\":"}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"\"ls\"}"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":15}}

event: message_stop
data: {"type":"message_stop"}

`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, sseData)
	}))
	defer srv.Close()

	p := NewAnthropic("test-key")
	ch, err := streamWithURL(p, srv.URL+"/v1/messages", ModelConfig{Model: "claude-sonnet-4-20250514"}, []Message{UserMessage("list files")}, nil)
	if err != nil {
		t.Fatal(err)
	}

	events := collectStreamEvents(ch)

	var starts, deltas, ends []StreamEvent
	var doneEvent *StreamEvent

	for i := range events {
		switch events[i].Type {
		case EventToolCallStart:
			starts = append(starts, events[i])
		case EventToolCallDelta:
			deltas = append(deltas, events[i])
		case EventToolCallEnd:
			ends = append(ends, events[i])
		case EventDone:
			doneEvent = &events[i]
		}
	}

	if len(starts) != 1 {
		t.Fatalf("expected 1 start, got %d", len(starts))
	}
	if starts[0].ToolCall.ID != "toolu_123" || starts[0].ToolCall.Name != "bash" {
		t.Fatalf("start = %+v", starts[0].ToolCall)
	}

	if len(deltas) != 2 {
		t.Fatalf("expected 2 deltas, got %d", len(deltas))
	}

	if len(ends) != 1 {
		t.Fatalf("expected 1 end, got %d", len(ends))
	}
	if ends[0].ToolCall.Args != `{"command":"ls"}` {
		t.Fatalf("end args = %q", ends[0].ToolCall.Args)
	}

	if doneEvent == nil || doneEvent.FinishReason != "tool_calls" {
		t.Fatalf("done = %+v", doneEvent)
	}
}

func TestAnthropicBuildRequestSystem(t *testing.T) {
	p := NewAnthropic("key")
	cfg := ModelConfig{Model: "claude-sonnet-4-20250514"}

	msgs := []Message{
		SystemMessage("You are helpful."),
		UserMessage("hi"),
	}

	body := p.buildRequest(cfg, msgs, nil)

	if body["system"] != "You are helpful." {
		t.Fatalf("system = %v", body["system"])
	}

	apiMsgs := body["messages"].([]map[string]any)
	if len(apiMsgs) != 1 {
		t.Fatalf("expected 1 message (system extracted), got %d", len(apiMsgs))
	}
	if apiMsgs[0]["role"] != "user" {
		t.Fatalf("msg role = %v", apiMsgs[0]["role"])
	}
}

func TestAnthropicBuildRequestToolResult(t *testing.T) {
	p := NewAnthropic("key")
	cfg := ModelConfig{Model: "claude-sonnet-4-20250514"}

	msgs := []Message{
		UserMessage("list files"),
		AssistantToolCallMessage("", []ToolCall{{ID: "toolu_1", Name: "bash", Args: `{"command":"ls"}`}}),
		ToolResultMessage("toolu_1", "bash", "file1.go"),
	}

	body := p.buildRequest(cfg, msgs, nil)
	apiMsgs := body["messages"].([]map[string]any)

	if len(apiMsgs) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(apiMsgs))
	}

	// Tool result should be role=user with tool_result content
	toolMsg := apiMsgs[2]
	if toolMsg["role"] != "user" {
		t.Fatalf("tool result role = %v", toolMsg["role"])
	}
	content := toolMsg["content"].([]map[string]any)
	if content[0]["type"] != "tool_result" {
		t.Fatalf("content type = %v", content[0]["type"])
	}
	if content[0]["tool_use_id"] != "toolu_1" {
		t.Fatalf("tool_use_id = %v", content[0]["tool_use_id"])
	}
}

func TestAnthropicStreamError(t *testing.T) {
	sseData := `event: error
data: {"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}

`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, sseData)
	}))
	defer srv.Close()

	p := NewAnthropic("test-key")
	ch, err := streamWithURL(p, srv.URL+"/v1/messages", ModelConfig{Model: "claude-sonnet-4-20250514"}, []Message{UserMessage("hi")}, nil)
	if err != nil {
		t.Fatal(err)
	}

	events := collectStreamEvents(ch)

	hasError := false
	for _, ev := range events {
		if ev.Type == EventError {
			hasError = true
		}
	}
	if !hasError {
		t.Fatal("expected error event")
	}
}

// streamWithURL is a test helper that sends a request to a custom URL
// instead of the real Anthropic API.
func streamWithURL(p *AnthropicProvider, url string, cfg ModelConfig, messages []Message, tools []ToolDef) (<-chan StreamEvent, error) {
	body := p.buildRequest(cfg, messages, tools)

	reqBytes := MarshalJSON(body)

	req, err := http.NewRequestWithContext(context.Background(), "POST", url, nil)
	if err != nil {
		return nil, err
	}
	req.Body = io.NopCloser(bytesReader(reqBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	ch := make(chan StreamEvent, 32)
	go p.readStream(resp.Body, ch)
	return ch, nil
}

type bytesReaderCloser struct {
	*bytesReaderImpl
}

type bytesReaderImpl struct {
	data []byte
	pos  int
}

func bytesReader(data []byte) *bytesReaderImpl {
	return &bytesReaderImpl{data: data}
}

func (r *bytesReaderImpl) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

func (r *bytesReaderImpl) Close() error { return nil }
