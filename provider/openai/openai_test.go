package openai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/srogers/oc/provider"
)

// newTestOpenAI creates an OpenAIProvider wired to the given test server.
func newTestOpenAI(srv *httptest.Server) *OpenAIProvider {
	return NewOpenAI("", "test-key", srv.URL, srv.Client())
}

func TestOpenAIStreamText(t *testing.T) {
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

	p := newTestOpenAI(srv)
	cfg := provider.ModelConfig{Model: "gpt-4o"}

	ch, err := p.Stream(context.Background(), cfg, []provider.Message{provider.UserMessage("hi")}, nil)
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

	p := newTestOpenAI(srv)
	cfg := provider.ModelConfig{Model: "gpt-4o"}

	ch, err := p.Stream(context.Background(), cfg, []provider.Message{provider.UserMessage("list files")}, nil)
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

	p := newTestOpenAI(srv)
	cfg := provider.ModelConfig{Model: "gpt-4o"}

	_, err := p.Stream(context.Background(), cfg, []provider.Message{provider.UserMessage("hi")}, nil)
	if err == nil {
		t.Fatal("expected error for 401")
	}
}

func TestOpenAIBuildRequestWithTools(t *testing.T) {
	p := NewOpenAI("", "key", "")
	cfg := provider.ModelConfig{Model: "gpt-4o"}
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

	body := p.buildRequest(cfg, []provider.Message{provider.UserMessage("hi")}, tools)

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
	p := NewOpenAI("", "key", "")
	cfg := provider.ModelConfig{Model: "gpt-4o"}

	msgs := []provider.Message{
		provider.UserMessage("list files"),
		provider.AssistantToolCallMessage("", []provider.ToolCall{{ID: "call_1", Name: "bash", Args: `{"command":"ls"}`}}),
		provider.ToolResultMessage("call_1", "bash", "file1.go\nfile2.go"),
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

func TestOpenAIProviderName(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"", "openai"},
		{"openai", "openai"},
		{"ollama", "ollama"},
		{"lm-studio", "lm-studio"},
	}
	for _, tt := range tests {
		p := NewOpenAI(tt.name, "", "")
		if got := p.Name(); got != tt.want {
			t.Errorf("NewOpenAI(%q, ...).Name() = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestBuildURL(t *testing.T) {
	tests := []struct {
		baseURL string
		want    string
	}{
		{"https://api.openai.com", "https://api.openai.com/v1/chat/completions"},
		{"http://localhost:11434", "http://localhost:11434/v1/chat/completions"},
		{"http://localhost:11434/v1", "http://localhost:11434/v1/chat/completions"},
		{"http://localhost:11434/v1/", "http://localhost:11434/v1/chat/completions"},
		{"http://localhost:1234/v1", "http://localhost:1234/v1/chat/completions"},
	}
	for _, tt := range tests {
		p := NewOpenAI("", "", tt.baseURL)
		if got := p.buildURL(); got != tt.want {
			t.Errorf("buildURL(%q) = %q, want %q", tt.baseURL, got, tt.want)
		}
	}
}

func TestOpenAIBuildRequestIncludesStreamOptions(t *testing.T) {
	p := NewOpenAI("", "key", "")
	cfg := provider.ModelConfig{Model: "gpt-4o"}
	body := p.buildRequest(cfg, []provider.Message{provider.UserMessage("hi")}, nil)

	so, ok := body["stream_options"]
	if !ok {
		t.Fatal("missing stream_options")
	}
	soMap, ok := so.(map[string]any)
	if !ok {
		t.Fatalf("stream_options is %T, want map[string]any", so)
	}
	if soMap["include_usage"] != true {
		t.Fatalf("include_usage = %v, want true", soMap["include_usage"])
	}
}

func TestOpenAIRateLimitError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(w, `{"error":{"message":"rate limited"}}`)
	}))
	defer srv.Close()

	p := newTestOpenAI(srv)
	_, err := p.Stream(
		context.Background(),
		provider.ModelConfig{Model: "gpt-4o"},
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
	if rle.RetryAfter != 60*time.Second {
		t.Errorf("RetryAfter = %v, want 60s", rle.RetryAfter)
	}
}

func TestOpenAIBuildRequestConfigOptions(t *testing.T) {
	temp := 0.5
	maxTokens := 2048
	topP := 0.95
	p := NewOpenAI("", "key", "")
	cfg := provider.ModelConfig{
		Model:       "gpt-4o",
		Temperature: &temp,
		MaxTokens:   &maxTokens,
		TopP:        &topP,
	}

	body := p.buildRequest(cfg, []provider.Message{provider.UserMessage("hi")}, nil)

	if body["temperature"] != 0.5 {
		t.Errorf("temperature = %v, want 0.5", body["temperature"])
	}
	if body["max_tokens"] != 2048 {
		t.Errorf("max_tokens = %v, want 2048", body["max_tokens"])
	}
	if body["top_p"] != 0.95 {
		t.Errorf("top_p = %v, want 0.95", body["top_p"])
	}
}

func TestOpenAIBuildRequestNoOptionalFields(t *testing.T) {
	p := NewOpenAI("", "key", "")
	cfg := provider.ModelConfig{Model: "gpt-4o"}
	body := p.buildRequest(cfg, []provider.Message{provider.UserMessage("hi")}, nil)

	if _, ok := body["temperature"]; ok {
		t.Error("temperature should not be set when nil")
	}
	if _, ok := body["max_tokens"]; ok {
		t.Error("max_tokens should not be set when nil")
	}
	if _, ok := body["top_p"]; ok {
		t.Error("top_p should not be set when nil")
	}
	if _, ok := body["tools"]; ok {
		t.Error("tools should not be set when empty")
	}
}

func TestOpenAIStreamAuthorizationHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer my-key" {
			t.Errorf("Authorization = %q, want %q", got, "Bearer my-key")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
	}))
	defer srv.Close()

	p := NewOpenAI("", "my-key", srv.URL, srv.Client())
	ch, err := p.Stream(
		context.Background(),
		provider.ModelConfig{Model: "gpt-4o"},
		[]provider.Message{provider.UserMessage("hi")},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	provider.CollectStreamEvents(ch)
}

func TestOpenAIStreamNoApiKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "" {
			t.Errorf("Authorization should be empty, got %q", got)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
	}))
	defer srv.Close()

	p := NewOpenAI("ollama", "", srv.URL, srv.Client())
	ch, err := p.Stream(
		context.Background(),
		provider.ModelConfig{Model: "llama3"},
		[]provider.Message{provider.UserMessage("hi")},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	provider.CollectStreamEvents(ch)
}

func TestOpenAIStreamRequestBody(t *testing.T) {
	var capturedBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		json.Unmarshal(bodyBytes, &capturedBody)
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
	}))
	defer srv.Close()

	p := NewOpenAI("", "key", srv.URL, srv.Client())
	ch, err := p.Stream(
		context.Background(),
		provider.ModelConfig{Model: "gpt-4o"},
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

	if capturedBody["model"] != "gpt-4o" {
		t.Errorf("model = %v", capturedBody["model"])
	}
	if capturedBody["stream"] != true {
		t.Errorf("stream = %v", capturedBody["stream"])
	}
	msgs := capturedBody["messages"].([]any)
	// OpenAI keeps system message inline (unlike Anthropic)
	if len(msgs) != 2 {
		t.Errorf("expected 2 messages, got %d", len(msgs))
	}
	firstMsg := msgs[0].(map[string]any)
	if firstMsg["role"] != "system" {
		t.Errorf("first message role = %v, want system", firstMsg["role"])
	}
	tools := capturedBody["tools"].([]any)
	if len(tools) != 1 {
		t.Errorf("expected 1 tool, got %d", len(tools))
	}
}

func TestOpenAIStreamReasoning(t *testing.T) {
	sseData := `data: {"choices":[{"delta":{"reasoning_content":"Let me think..."},"finish_reason":null}]}

data: {"choices":[{"delta":{"content":"42"},"finish_reason":null}]}

data: {"choices":[{"delta":{},"finish_reason":"stop"}]}

data: [DONE]

`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, sseData)
	}))
	defer srv.Close()

	p := newTestOpenAI(srv)
	ch, err := p.Stream(
		context.Background(),
		provider.ModelConfig{Model: "o1"},
		[]provider.Message{provider.UserMessage("think")},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	events := provider.CollectStreamEvents(ch)

	var hasReasoning, hasText bool
	for _, ev := range events {
		switch ev.Type {
		case provider.EventReasoningDelta:
			hasReasoning = true
			if ev.Text != "Let me think..." {
				t.Errorf("reasoning = %q", ev.Text)
			}
		case provider.EventText:
			hasText = true
			if ev.Text != "42" {
				t.Errorf("text = %q", ev.Text)
			}
		}
	}

	if !hasReasoning {
		t.Error("expected reasoning event")
	}
	if !hasText {
		t.Error("expected text event")
	}
}

func TestOpenAIStreamUsageOnlyChunk(t *testing.T) {
	// Some providers send a separate usage chunk with no choices.
	sseData := `data: {"choices":[{"delta":{"content":"hi"},"finish_reason":null}]}

data: {"choices":[{"delta":{},"finish_reason":"stop"}]}

data: {"usage":{"prompt_tokens":5,"completion_tokens":2,"total_tokens":7}}

data: [DONE]

`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, sseData)
	}))
	defer srv.Close()

	p := newTestOpenAI(srv)
	ch, err := p.Stream(
		context.Background(),
		provider.ModelConfig{Model: "gpt-4o"},
		[]provider.Message{provider.UserMessage("hi")},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	events := provider.CollectStreamEvents(ch)

	// Should get two done events: one from finish_reason, one from usage-only chunk
	var doneEvents []provider.StreamEvent
	for _, ev := range events {
		if ev.Type == provider.EventDone {
			doneEvents = append(doneEvents, ev)
		}
	}

	if len(doneEvents) < 1 {
		t.Fatal("expected at least 1 done event")
	}

	// The usage-only chunk should have usage data
	var hasUsage bool
	for _, d := range doneEvents {
		if d.Usage != nil && d.Usage.InputTokens == 5 {
			hasUsage = true
		}
	}
	if !hasUsage {
		t.Error("expected done event with usage from standalone chunk")
	}
}

func TestOpenAIStreamContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {}
	}))
	defer srv.Close()

	p := newTestOpenAI(srv)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := p.Stream(
		ctx,
		provider.ModelConfig{Model: "gpt-4o"},
		[]provider.Message{provider.UserMessage("hi")},
		nil,
	)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

func TestOpenAIBuildRequestAssistantWithTextAndToolCalls(t *testing.T) {
	p := NewOpenAI("", "key", "")
	cfg := provider.ModelConfig{Model: "gpt-4o"}

	msgs := []provider.Message{
		provider.UserMessage("do something"),
		provider.AssistantToolCallMessage("Let me help.", []provider.ToolCall{
			{ID: "call_1", Name: "bash", Args: `{"command":"ls"}`},
		}),
	}

	body := p.buildRequest(cfg, msgs, nil)
	apiMsgs := body["messages"].([]map[string]any)

	assistantMsg := apiMsgs[1]
	if assistantMsg["content"] != "Let me help." {
		t.Errorf("content = %v", assistantMsg["content"])
	}
	tcs := assistantMsg["tool_calls"].([]map[string]any)
	if len(tcs) != 1 {
		t.Fatalf("expected 1 tool_call, got %d", len(tcs))
	}
	if tcs[0]["id"] != "call_1" {
		t.Errorf("tool call id = %v", tcs[0]["id"])
	}
}

func TestOpenAIBuildRequestAssistantNoContentWithToolCalls(t *testing.T) {
	p := NewOpenAI("", "key", "")
	cfg := provider.ModelConfig{Model: "gpt-4o"}

	msgs := []provider.Message{
		provider.UserMessage("do something"),
		provider.AssistantToolCallMessage("", []provider.ToolCall{
			{ID: "call_1", Name: "bash", Args: `{"command":"ls"}`},
		}),
	}

	body := p.buildRequest(cfg, msgs, nil)
	apiMsgs := body["messages"].([]map[string]any)

	assistantMsg := apiMsgs[1]
	// When content is empty, it should be nil (not empty string) for OpenAI compat
	if assistantMsg["content"] != nil {
		t.Errorf("content = %v, want nil", assistantMsg["content"])
	}
}
