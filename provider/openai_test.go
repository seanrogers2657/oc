package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// collectStreamEvents drains a StreamEvent channel into a slice.
func collectStreamEvents(ch <-chan StreamEvent) []StreamEvent {
	var events []StreamEvent
	for ev := range ch {
		events = append(events, ev)
	}
	return events
}

func TestOpenAIStreamText(t *testing.T) {
	// Simulate a simple text response
	sseData := `data: {"choices":[{"delta":{"content":"Hello"},"finish_reason":null}]}

data: {"choices":[{"delta":{"content":" world"},"finish_reason":null}]}

data: {"choices":[{"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}

data: [DONE]

`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, sseData)
	}))
	defer srv.Close()

	p := NewOpenAI("test-key", srv.URL)
	cfg := ModelConfig{Model: "gpt-4o"}

	ch, err := p.Stream(context.Background(), cfg, []Message{UserMessage("hi")}, nil)
	if err != nil {
		t.Fatal(err)
	}

	events := collectStreamEvents(ch)

	// Should have: text("Hello"), text(" world"), done
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
	if doneEvent.FinishReason != "stop" {
		t.Fatalf("finish_reason = %q, want %q", doneEvent.FinishReason, "stop")
	}
	if doneEvent.Usage == nil {
		t.Fatal("no usage in done event")
	}
	if doneEvent.Usage.InputTokens != 10 || doneEvent.Usage.OutputTokens != 5 {
		t.Fatalf("usage = %+v", doneEvent.Usage)
	}
}

func TestOpenAIStreamToolCall(t *testing.T) {
	// Simulate a tool call response
	sseData := `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_abc","function":{"name":"bash","arguments":""}}]},"finish_reason":null}]}

data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"command\":"}}]},"finish_reason":null}]}

data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"ls\"}"}}]},"finish_reason":null}]}

data: {"choices":[{"delta":{},"finish_reason":"tool_calls"}]}

data: [DONE]

`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, sseData)
	}))
	defer srv.Close()

	p := NewOpenAI("test-key", srv.URL)
	cfg := ModelConfig{Model: "gpt-4o"}

	ch, err := p.Stream(context.Background(), cfg, []Message{UserMessage("list files")}, nil)
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
		t.Fatalf("expected 1 tool call start, got %d", len(starts))
	}
	if starts[0].ToolCall.ID != "call_abc" || starts[0].ToolCall.Name != "bash" {
		t.Fatalf("start = %+v", starts[0].ToolCall)
	}

	if len(deltas) != 2 {
		t.Fatalf("expected 2 tool call deltas, got %d", len(deltas))
	}

	if len(ends) != 1 {
		t.Fatalf("expected 1 tool call end, got %d", len(ends))
	}
	if ends[0].ToolCall.Args != `{"command":"ls"}` {
		t.Fatalf("end args = %q, want %q", ends[0].ToolCall.Args, `{"command":"ls"}`)
	}

	if doneEvent == nil || doneEvent.FinishReason != "tool_calls" {
		t.Fatalf("done event: %+v", doneEvent)
	}
}

func TestOpenAIStreamAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":{"message":"invalid api key"}}`)
	}))
	defer srv.Close()

	p := NewOpenAI("bad-key", srv.URL)
	cfg := ModelConfig{Model: "gpt-4o"}

	_, err := p.Stream(context.Background(), cfg, []Message{UserMessage("hi")}, nil)
	if err == nil {
		t.Fatal("expected error for 401")
	}
}

func TestOpenAIBuildRequestWithTools(t *testing.T) {
	p := NewOpenAI("key", "")
	cfg := ModelConfig{Model: "gpt-4o"}
	tools := []ToolDef{
		{
			Name:        "bash",
			Description: "Run a shell command",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"command": map[string]any{"type": "string"},
				},
				"required": []string{"command"},
			},
		},
	}

	body := p.buildRequest(cfg, []Message{UserMessage("hi")}, tools)

	toolsField, ok := body["tools"]
	if !ok {
		t.Fatal("missing tools field")
	}
	toolsList, ok := toolsField.([]map[string]any)
	if !ok || len(toolsList) != 1 {
		t.Fatalf("tools = %v", toolsField)
	}
	if toolsList[0]["type"] != "function" {
		t.Fatalf("tool type = %v", toolsList[0]["type"])
	}
}

func TestOpenAIBuildRequestToolResult(t *testing.T) {
	p := NewOpenAI("key", "")
	cfg := ModelConfig{Model: "gpt-4o"}

	msgs := []Message{
		UserMessage("list files"),
		AssistantToolCallMessage("", []ToolCall{{ID: "call_1", Name: "bash", Args: `{"command":"ls"}`}}),
		ToolResultMessage("call_1", "bash", "file1.go\nfile2.go"),
	}

	body := p.buildRequest(cfg, msgs, nil)
	apiMsgs := body["messages"].([]map[string]any)

	if len(apiMsgs) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(apiMsgs))
	}

	// Tool result message
	toolMsg := apiMsgs[2]
	if toolMsg["role"] != "tool" {
		t.Fatalf("tool msg role = %v", toolMsg["role"])
	}
	if toolMsg["tool_call_id"] != "call_1" {
		t.Fatalf("tool_call_id = %v", toolMsg["tool_call_id"])
	}
}
