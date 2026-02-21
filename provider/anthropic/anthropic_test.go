package anthropic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/srogers/oc/config"
	"github.com/srogers/oc/provider"
)

// newTestProvider creates an AnthropicApiProvider wired to the given test server.
func newTestProvider(t *testing.T, srv *httptest.Server) provider.Provider {
	t.Helper()
	cfg := config.Config{Provider: "anthropic", APIKey: "test-key"}
	p, err := NewAnthropicApiProvider(cfg, srv.Client(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

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

	p := newTestProvider(t, srv)
	ch, err := p.Stream(
		t.Context(),
		provider.ModelConfig{Model: "claude-sonnet-4-20250514"},
		[]provider.Message{provider.UserMessage("hi")},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	events := provider.CollectStreamEvents(ch)

	var texts []string
	var doneEvent *provider.StreamEvent

	for i := range events {
		switch events[i].Type {
		case provider.EventText:
			texts = append(texts, events[i].Text)
		case provider.EventDone:
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

	p := newTestProvider(t, srv)
	ch, err := p.Stream(
		t.Context(),
		provider.ModelConfig{Model: "claude-sonnet-4-20250514"},
		[]provider.Message{provider.UserMessage("list files")},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	events := provider.CollectStreamEvents(ch)

	var starts, deltas, ends []provider.StreamEvent
	var doneEvent *provider.StreamEvent

	for i := range events {
		switch events[i].Type {
		case provider.EventToolCallStart:
			starts = append(starts, events[i])
		case provider.EventToolCallDelta:
			deltas = append(deltas, events[i])
		case provider.EventToolCallEnd:
			ends = append(ends, events[i])
		case provider.EventDone:
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
	cfg := provider.ModelConfig{Model: "claude-sonnet-4-20250514"}

	msgs := []provider.Message{
		provider.SystemMessage("You are helpful."),
		provider.UserMessage("hi"),
	}

	body := buildRequest(cfg, msgs, nil)

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
	cfg := provider.ModelConfig{Model: "claude-sonnet-4-20250514"}

	msgs := []provider.Message{
		provider.UserMessage("list files"),
		provider.AssistantToolCallMessage("", []provider.ToolCall{{ID: "toolu_1", Name: "bash", Args: `{"command":"ls"}`}}),
		provider.ToolResultMessage("toolu_1", "bash", "file1.go"),
	}

	body := buildRequest(cfg, msgs, nil)
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

	p := newTestProvider(t, srv)
	ch, err := p.Stream(
		context.Background(),
		provider.ModelConfig{Model: "claude-sonnet-4-20250514"},
		[]provider.Message{provider.UserMessage("hi")},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	events := provider.CollectStreamEvents(ch)

	hasError := false
	for _, ev := range events {
		if ev.Type == provider.EventError {
			hasError = true
		}
	}
	if !hasError {
		t.Fatal("expected error event")
	}
}

func TestAnthropicApiProviderName(t *testing.T) {
	cfg := config.Config{Provider: "anthropic", APIKey: "key"}
	p, err := NewAnthropicApiProvider(cfg, nil, AnthropicBaseUrl)
	if err != nil {
		t.Fatal(err)
	}
	if p.Name() != "anthropic-api" {
		t.Errorf("Name() = %q, want %q", p.Name(), "anthropic-api")
	}
}

func TestAnthropicHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":{"message":"invalid api key"}}`)
	}))
	defer srv.Close()

	p := newTestProvider(t, srv)
	_, err := p.Stream(
		context.Background(),
		provider.ModelConfig{Model: "claude-sonnet-4-20250514"},
		[]provider.Message{provider.UserMessage("hi")},
		nil,
	)
	if err == nil {
		t.Fatal("expected error for 401")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error should mention status code, got: %v", err)
	}
}

func TestAnthropicRateLimitError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(w, `{"error":{"message":"rate limited"}}`)
	}))
	defer srv.Close()

	p := newTestProvider(t, srv)
	_, err := p.Stream(
		context.Background(),
		provider.ModelConfig{Model: "claude-sonnet-4-20250514"},
		[]provider.Message{provider.UserMessage("hi")},
		nil,
	)
	if err == nil {
		t.Fatal("expected error for 429")
	}

	var rle *provider.RateLimitError
	if !errors.As(err, &rle) {
		t.Fatalf("expected RateLimitError, got %T: %v", err, err)
	}
	if rle.RetryAfter != 30*time.Second {
		t.Errorf("RetryAfter = %v, want 30s", rle.RetryAfter)
	}
}

func TestAnthropicBuildRequestWithTools(t *testing.T) {
	cfg := provider.ModelConfig{Model: "claude-sonnet-4-20250514"}
	tools := []provider.ToolDef{
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

	body := buildRequest(cfg, []provider.Message{provider.UserMessage("hi")}, tools)

	toolsField, ok := body["tools"]
	if !ok {
		t.Fatal("missing tools field")
	}
	toolsList, ok := toolsField.([]map[string]any)
	if !ok || len(toolsList) != 1 {
		t.Fatalf("tools = %v", toolsField)
	}
	if toolsList[0]["name"] != "bash" {
		t.Fatalf("tool name = %v", toolsList[0]["name"])
	}
	if _, ok := toolsList[0]["input_schema"]; !ok {
		t.Fatal("missing input_schema in tool definition")
	}
}

func TestAnthropicBuildRequestConfigOptions(t *testing.T) {
	temp := 0.7
	maxTokens := 8192
	topP := 0.9
	cfg := provider.ModelConfig{
		Model:       "claude-sonnet-4-20250514",
		Temperature: &temp,
		MaxTokens:   &maxTokens,
		TopP:        &topP,
	}

	body := buildRequest(cfg, []provider.Message{provider.UserMessage("hi")}, nil)

	if body["temperature"] != 0.7 {
		t.Errorf("temperature = %v, want 0.7", body["temperature"])
	}
	if body["max_tokens"] != 8192 {
		t.Errorf("max_tokens = %v, want 8192", body["max_tokens"])
	}
	if body["top_p"] != 0.9 {
		t.Errorf("top_p = %v, want 0.9", body["top_p"])
	}
}

func TestAnthropicBuildRequestDefaultMaxTokens(t *testing.T) {
	cfg := provider.ModelConfig{Model: "claude-sonnet-4-20250514"}
	body := buildRequest(cfg, []provider.Message{provider.UserMessage("hi")}, nil)

	if body["max_tokens"] != 4096 {
		t.Errorf("default max_tokens = %v, want 4096", body["max_tokens"])
	}
}

func TestAnthropicBuildRequestAssistantWithTextAndToolCalls(t *testing.T) {
	cfg := provider.ModelConfig{Model: "claude-sonnet-4-20250514"}

	msgs := []provider.Message{
		provider.UserMessage("do something"),
		provider.AssistantToolCallMessage("Let me help.", []provider.ToolCall{
			{ID: "toolu_1", Name: "bash", Args: `{"command":"ls"}`},
		}),
	}

	body := buildRequest(cfg, msgs, nil)
	apiMsgs := body["messages"].([]map[string]any)

	if len(apiMsgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(apiMsgs))
	}

	assistantMsg := apiMsgs[1]
	content := assistantMsg["content"].([]map[string]any)
	if len(content) != 2 {
		t.Fatalf("expected 2 content blocks (text + tool_use), got %d", len(content))
	}
	if content[0]["type"] != "text" || content[0]["text"] != "Let me help." {
		t.Errorf("text block = %v", content[0])
	}
	if content[1]["type"] != "tool_use" || content[1]["id"] != "toolu_1" {
		t.Errorf("tool_use block = %v", content[1])
	}
}

func TestAnthropicBuildRequestNoSystemMessage(t *testing.T) {
	cfg := provider.ModelConfig{Model: "claude-sonnet-4-20250514"}

	body := buildRequest(cfg, []provider.Message{provider.UserMessage("hi")}, nil)

	if _, ok := body["system"]; ok {
		t.Fatal("system field should not be present when no system message")
	}
}

func TestAnthropicStreamThinking(t *testing.T) {
	sseData := `event: message_start
data: {"type":"message_start","message":{"usage":{"input_tokens":10}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"thinking"}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"Let me think..."}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: content_block_start
data: {"type":"content_block_start","index":1,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"The answer is 42."}}

event: content_block_stop
data: {"type":"content_block_stop","index":1}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":20}}

event: message_stop
data: {"type":"message_stop"}

`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, sseData)
	}))
	defer srv.Close()

	p := newTestProvider(t, srv)
	ch, err := p.Stream(
		t.Context(),
		provider.ModelConfig{Model: "claude-sonnet-4-20250514"},
		[]provider.Message{provider.UserMessage("think")},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	events := provider.CollectStreamEvents(ch)

	var hasReasoningStart, hasReasoningDelta, hasText bool
	for _, ev := range events {
		switch ev.Type {
		case provider.EventReasoningStart:
			hasReasoningStart = true
		case provider.EventReasoningDelta:
			hasReasoningDelta = true
			if ev.Text != "Let me think..." {
				t.Errorf("reasoning text = %q", ev.Text)
			}
		case provider.EventText:
			hasText = true
			if ev.Text != "The answer is 42." {
				t.Errorf("text = %q", ev.Text)
			}
		}
	}

	if !hasReasoningStart {
		t.Error("expected reasoning start event")
	}
	if !hasReasoningDelta {
		t.Error("expected reasoning delta event")
	}
	if !hasText {
		t.Error("expected text event")
	}
}

func TestAnthropicStreamRequestBody(t *testing.T) {
	// Verify the HTTP request body sent to the API is well-formed.
	var capturedBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		json.Unmarshal(bodyBytes, &capturedBody)

		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, `event: message_start
data: {"type":"message_start","message":{"usage":{"input_tokens":5}}}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":1}}

event: message_stop
data: {"type":"message_stop"}

`)
	}))
	defer srv.Close()

	p := newTestProvider(t, srv)
	ch, err := p.Stream(
		t.Context(),
		provider.ModelConfig{Model: "claude-sonnet-4-20250514"},
		[]provider.Message{
			provider.SystemMessage("Be brief."),
			provider.UserMessage("hi"),
		},
		[]provider.ToolDef{{Name: "bash", Description: "run cmd", Parameters: map[string]any{"type": "object"}}},
	)
	if err != nil {
		t.Fatal(err)
	}
	provider.CollectStreamEvents(ch)

	if capturedBody["model"] != "claude-sonnet-4-20250514" {
		t.Errorf("model = %v", capturedBody["model"])
	}
	if capturedBody["stream"] != true {
		t.Errorf("stream = %v", capturedBody["stream"])
	}
	if capturedBody["system"] != "Be brief." {
		t.Errorf("system = %v", capturedBody["system"])
	}
	msgs := capturedBody["messages"].([]any)
	if len(msgs) != 1 {
		t.Errorf("expected 1 message (system extracted), got %d", len(msgs))
	}
	tools := capturedBody["tools"].([]any)
	if len(tools) != 1 {
		t.Errorf("expected 1 tool, got %d", len(tools))
	}
}

func TestAnthropicStreamContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Hang forever — the client should cancel via context.
		select {}
	}))
	defer srv.Close()

	p := newTestProvider(t, srv)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := p.Stream(
		ctx,
		provider.ModelConfig{Model: "claude-sonnet-4-20250514"},
		[]provider.Message{provider.UserMessage("hi")},
		nil,
	)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}
