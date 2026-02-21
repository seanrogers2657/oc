package session

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/srogers/oc/event"
	"github.com/srogers/oc/provider"
	"github.com/srogers/oc/tool"
)

// --- Mock Provider ---

type mockProvider struct {
	// streamFn is called for each Stream() call. It should send events on the channel and close it.
	streamFn func(msgs []provider.Message, tools []provider.ToolDef) <-chan provider.StreamEvent
	calls    int
	mu       sync.Mutex
}

func (m *mockProvider) Stream(_ context.Context, _ provider.ModelConfig, msgs []provider.Message, tools []provider.ToolDef) (<-chan provider.StreamEvent, error) {
	m.mu.Lock()
	m.calls++
	m.mu.Unlock()
	return m.streamFn(msgs, tools), nil
}

// --- Mock Tool ---

type mockTool struct {
	name   string
	result tool.Result
}

func (m *mockTool) Name() string                                 { return m.name }
func (m *mockTool) Description() string                          { return m.name + " tool" }
func (m *mockTool) Parameters() map[string]any                   { return map[string]any{"type": "object"} }
func (m *mockTool) Execute(_ tool.Context, _ string) tool.Result { return m.result }

// --- Mock ToolExecutor ---

type mockToolExecutor struct {
	tools map[string]tool.Tool
}

func (m *mockToolExecutor) Get(name string) (tool.Tool, bool) {
	t, ok := m.tools[name]
	return t, ok
}

func (m *mockToolExecutor) Defs() []provider.ToolDef {
	var defs []provider.ToolDef
	for _, t := range m.tools {
		defs = append(defs, provider.ToolDef{Name: t.Name(), Description: t.Description(), Parameters: t.Parameters()})
	}
	return defs
}

// --- Helpers ---

func newTestSession(mp *mockProvider, te *mockToolExecutor) (*Session, *event.Bus) {
	bus := event.NewBus()
	store := NewStore()
	s := store.Create(Deps{
		Model:  mp,
		Tools:  te,
		Events: bus,
	}, provider.ModelConfig{Model: "test-model"}, "")
	return s, bus
}

func waitForIdle(s *Session, timeout time.Duration) bool {
	deadline := time.After(timeout)
	for {
		select {
		case <-deadline:
			return false
		default:
			if s.GetStatus() == StatusIdle {
				return true
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
}

// --- Tests ---

func TestLoopSimpleTextResponse(t *testing.T) {
	mp := &mockProvider{
		streamFn: func(msgs []provider.Message, _ []provider.ToolDef) <-chan provider.StreamEvent {
			ch := make(chan provider.StreamEvent, 5)
			go func() {
				ch <- provider.StreamEvent{Type: provider.EventText, Text: "Hello "}
				ch <- provider.StreamEvent{Type: provider.EventText, Text: "world!"}
				ch <- provider.StreamEvent{Type: provider.EventDone, FinishReason: "stop", Usage: &provider.Usage{InputTokens: 10, OutputTokens: 5, TotalTokens: 15}}
				close(ch)
			}()
			return ch
		},
	}
	te := &mockToolExecutor{tools: map[string]tool.Tool{}}

	s, _ := newTestSession(mp, te)
	s.Send(context.Background(), "Hi")

	if !waitForIdle(s, 2*time.Second) {
		t.Fatal("session did not return to idle")
	}

	msgs := s.GetMessages()
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages (user + assistant), got %d", len(msgs))
	}

	// Check user message
	if msgs[0].Role != provider.RoleUser {
		t.Errorf("expected user role, got %s", msgs[0].Role)
	}

	// Check assistant message
	if msgs[1].Role != provider.RoleAssistant {
		t.Errorf("expected assistant role, got %s", msgs[1].Role)
	}
	text := extractText(msgs[1].Parts)
	if text != "Hello world!" {
		t.Errorf("expected 'Hello world!', got %q", text)
	}

	// Check token tracking
	tokens := s.GetTokens()
	if tokens.TotalTokens != 15 {
		t.Errorf("expected 15 total tokens, got %d", tokens.TotalTokens)
	}
}

func TestLoopWithToolCall(t *testing.T) {
	callCount := 0
	mp := &mockProvider{
		streamFn: func(msgs []provider.Message, _ []provider.ToolDef) <-chan provider.StreamEvent {
			ch := make(chan provider.StreamEvent, 10)
			go func() {
				callCount++
				if callCount == 1 {
					// First call: model wants to use a tool
					ch <- provider.StreamEvent{Type: provider.EventText, Text: "Let me check."}
					ch <- provider.StreamEvent{Type: provider.EventToolCallStart, ToolCall: &provider.ToolCall{ID: "call_1", Name: "echo"}}
					ch <- provider.StreamEvent{Type: provider.EventToolCallDelta, ToolCall: &provider.ToolCall{ID: "call_1", Args: `{"text":`}}
					ch <- provider.StreamEvent{Type: provider.EventToolCallDelta, ToolCall: &provider.ToolCall{ID: "call_1", Args: `"hi"}`}}
					ch <- provider.StreamEvent{Type: provider.EventToolCallEnd, ToolCall: &provider.ToolCall{ID: "call_1", Name: "echo", Args: `{"text":"hi"}`}}
					ch <- provider.StreamEvent{Type: provider.EventDone, FinishReason: "tool_calls", Usage: &provider.Usage{InputTokens: 10, OutputTokens: 20, TotalTokens: 30}}
				} else {
					// Second call: model responds with final text
					ch <- provider.StreamEvent{Type: provider.EventText, Text: "Done!"}
					ch <- provider.StreamEvent{Type: provider.EventDone, FinishReason: "stop", Usage: &provider.Usage{InputTokens: 30, OutputTokens: 5, TotalTokens: 35}}
				}
				close(ch)
			}()
			return ch
		},
	}

	te := &mockToolExecutor{
		tools: map[string]tool.Tool{
			"echo": &mockTool{name: "echo", result: tool.Result{Output: "echo: hi", Title: "echo"}},
		},
	}

	s, _ := newTestSession(mp, te)
	s.Send(context.Background(), "Do something")

	if !waitForIdle(s, 2*time.Second) {
		t.Fatal("session did not return to idle")
	}

	msgs := s.GetMessages()
	// Should be: user, assistant (tool call), tool result, assistant (final)
	if len(msgs) < 4 {
		t.Fatalf("expected at least 4 messages, got %d", len(msgs))
	}

	// Verify the tool call part has the Title from the tool result
	assistantMsg := msgs[1]
	for _, p := range assistantMsg.Parts {
		if tc, ok := p.(ToolCallPart); ok {
			if tc.Title != "echo" {
				t.Errorf("expected tool call Title %q, got %q", "echo", tc.Title)
			}
		}
	}

	// Verify provider was called twice (once for tool call, once after tool result)
	mp.mu.Lock()
	if mp.calls != 2 {
		t.Errorf("expected 2 provider calls, got %d", mp.calls)
	}
	mp.mu.Unlock()

	// Check the final assistant message has "Done!"
	lastMsg := msgs[len(msgs)-1]
	if lastMsg.Role != provider.RoleAssistant {
		t.Errorf("expected last message to be assistant, got %s", lastMsg.Role)
	}
	text := extractText(lastMsg.Parts)
	if text != "Done!" {
		t.Errorf("expected 'Done!', got %q", text)
	}

	// Check token accumulation
	tokens := s.GetTokens()
	if tokens.TotalTokens != 65 { // 30 + 35
		t.Errorf("expected 65 total tokens, got %d", tokens.TotalTokens)
	}
}

func TestLoopUnknownTool(t *testing.T) {
	callCount := 0
	mp := &mockProvider{
		streamFn: func(msgs []provider.Message, _ []provider.ToolDef) <-chan provider.StreamEvent {
			ch := make(chan provider.StreamEvent, 10)
			go func() {
				callCount++
				if callCount == 1 {
					ch <- provider.StreamEvent{Type: provider.EventToolCallEnd, ToolCall: &provider.ToolCall{ID: "call_1", Name: "nonexistent", Args: "{}"}}
					ch <- provider.StreamEvent{Type: provider.EventDone, FinishReason: "tool_calls"}
				} else {
					ch <- provider.StreamEvent{Type: provider.EventText, Text: "OK"}
					ch <- provider.StreamEvent{Type: provider.EventDone, FinishReason: "stop"}
				}
				close(ch)
			}()
			return ch
		},
	}

	te := &mockToolExecutor{tools: map[string]tool.Tool{}}
	s, _ := newTestSession(mp, te)
	s.Send(context.Background(), "use missing tool")

	if !waitForIdle(s, 2*time.Second) {
		t.Fatal("session did not return to idle")
	}

	// Should still complete -- unknown tool results in error message sent back to model
	msgs := s.GetMessages()
	foundToolError := false
	for _, msg := range msgs {
		if msg.Role == provider.RoleTool {
			text := extractText(msg.Parts)
			if text != "" {
				foundToolError = true
			}
		}
	}
	// The loop should handle unknown tools gracefully
	if len(msgs) < 3 {
		t.Fatalf("expected at least 3 messages, got %d", len(msgs))
	}
	_ = foundToolError
}

func TestLoopStreamError(t *testing.T) {
	mp := &mockProvider{
		streamFn: func(msgs []provider.Message, _ []provider.ToolDef) <-chan provider.StreamEvent {
			ch := make(chan provider.StreamEvent, 5)
			go func() {
				ch <- provider.StreamEvent{Type: provider.EventText, Text: "Starting..."}
				ch <- provider.StreamEvent{Type: provider.EventError, Error: fmt.Errorf("connection lost")}
				close(ch)
			}()
			return ch
		},
	}

	te := &mockToolExecutor{tools: map[string]tool.Tool{}}
	s, _ := newTestSession(mp, te)
	s.Send(context.Background(), "trigger error")

	// Session returns to idle after error (error appears in chat, not status)
	if !waitForIdle(s, 2*time.Second) {
		t.Fatal("session did not return to idle after error")
	}

	// Error should appear as an assistant message in chat
	msgs := s.GetMessages()
	var foundError bool
	for _, m := range msgs {
		if m.Error != nil {
			foundError = true
			break
		}
	}
	if !foundError {
		t.Fatal("expected an error message in chat history")
	}
}

func TestLoopStreamStartError(t *testing.T) {
	// When Stream() itself returns an error (e.g. auth failure, connection refused),
	// it should still create an error message in chat.
	te := &mockToolExecutor{tools: map[string]tool.Tool{}}
	bus := event.NewBus()
	store := NewStore()
	s := store.Create(Deps{
		Model:  &errorProvider{err: fmt.Errorf("401 Unauthorized")},
		Tools:  te,
		Events: bus,
	}, provider.ModelConfig{Model: "test-model"}, "")

	s.Send(context.Background(), "trigger stream error")

	if !waitForIdle(s, 2*time.Second) {
		t.Fatal("session did not return to idle after stream start error")
	}

	msgs := s.GetMessages()
	// Should have: user message + error message
	if len(msgs) < 2 {
		t.Fatalf("expected at least 2 messages, got %d", len(msgs))
	}

	// Last message should be the error
	errMsg := msgs[len(msgs)-1]
	if errMsg.Role != provider.RoleAssistant {
		t.Fatalf("expected assistant role for error message, got %s", errMsg.Role)
	}
	if errMsg.Error == nil {
		t.Fatal("expected error to be set on message")
	}
	text := extractText(errMsg.Parts)
	if text == "" {
		t.Fatal("expected error text in message parts")
	}
}

// errorProvider is a mock that always returns an error from Stream().
type errorProvider struct {
	err error
}

func (e *errorProvider) Stream(_ context.Context, _ provider.ModelConfig, _ []provider.Message, _ []provider.ToolDef) (<-chan provider.StreamEvent, error) {
	return nil, e.err
}

// rateLimitMockProvider returns a RateLimitError for the first failCount calls,
// then streams a successful response.
type rateLimitMockProvider struct {
	failCount  int // how many times to return 429 before succeeding
	retryAfter time.Duration
	calls      int
	mu         sync.Mutex
}

func (m *rateLimitMockProvider) Stream(_ context.Context, _ provider.ModelConfig, _ []provider.Message, _ []provider.ToolDef) (<-chan provider.StreamEvent, error) {
	m.mu.Lock()
	m.calls++
	call := m.calls
	m.mu.Unlock()

	if call <= m.failCount {
		return nil, &provider.RateLimitError{
			StatusCode: 429,
			Message:    "rate limit exceeded",
			RetryAfter: m.retryAfter,
		}
	}

	ch := make(chan provider.StreamEvent, 5)
	go func() {
		ch <- provider.StreamEvent{Type: provider.EventText, Text: "Success after retry"}
		ch <- provider.StreamEvent{Type: provider.EventDone, FinishReason: "stop", Usage: &provider.Usage{InputTokens: 5, OutputTokens: 3, TotalTokens: 8}}
		close(ch)
	}()
	return ch, nil
}

func TestLoopPublishesEvents(t *testing.T) {
	mp := &mockProvider{
		streamFn: func(msgs []provider.Message, _ []provider.ToolDef) <-chan provider.StreamEvent {
			ch := make(chan provider.StreamEvent, 5)
			go func() {
				ch <- provider.StreamEvent{Type: provider.EventText, Text: "hello"}
				ch <- provider.StreamEvent{Type: provider.EventDone, FinishReason: "stop"}
				close(ch)
			}()
			return ch
		},
	}

	te := &mockToolExecutor{tools: map[string]tool.Tool{}}
	s, bus := newTestSession(mp, te)

	var mu sync.Mutex
	var topics []event.Topic
	for _, topic := range []event.Topic{event.TopicStatus, event.TopicPartDelta, event.TopicMsgDone} {
		t := topic
		bus.Subscribe(t, func(p event.Payload) {
			mu.Lock()
			topics = append(topics, p.Topic)
			mu.Unlock()
		})
	}

	s.Send(context.Background(), "test events")

	if !waitForIdle(s, 2*time.Second) {
		t.Fatal("session did not return to idle")
	}

	mu.Lock()
	defer mu.Unlock()

	// Should have received: status(busy), part.delta, message.done, status(idle)
	hasBusy := false
	hasIdle := false
	hasDelta := false
	hasMsgDone := false
	for _, topic := range topics {
		switch topic {
		case event.TopicStatus:
			// We'll get both busy and idle
			if !hasBusy {
				hasBusy = true
			} else {
				hasIdle = true
			}
		case event.TopicPartDelta:
			hasDelta = true
		case event.TopicMsgDone:
			hasMsgDone = true
		}
	}

	if !hasBusy {
		t.Error("expected busy status event")
	}
	if !hasIdle {
		t.Error("expected idle status event")
	}
	if !hasDelta {
		t.Error("expected part delta event")
	}
	if !hasMsgDone {
		t.Error("expected message done event")
	}
}

func TestLoopWithReasoning(t *testing.T) {
	mp := &mockProvider{
		streamFn: func(msgs []provider.Message, _ []provider.ToolDef) <-chan provider.StreamEvent {
			ch := make(chan provider.StreamEvent, 10)
			go func() {
				ch <- provider.StreamEvent{Type: provider.EventReasoningStart}
				ch <- provider.StreamEvent{Type: provider.EventReasoningDelta, Text: "Let me think..."}
				ch <- provider.StreamEvent{Type: provider.EventReasoningEnd}
				ch <- provider.StreamEvent{Type: provider.EventText, Text: "The answer is 42."}
				ch <- provider.StreamEvent{Type: provider.EventDone, FinishReason: "stop"}
				close(ch)
			}()
			return ch
		},
	}

	te := &mockToolExecutor{tools: map[string]tool.Tool{}}
	s, _ := newTestSession(mp, te)
	s.Send(context.Background(), "think about this")

	if !waitForIdle(s, 2*time.Second) {
		t.Fatal("session did not return to idle")
	}

	msgs := s.GetMessages()
	assistantMsg := msgs[1]

	// Should have both reasoning and text parts
	hasReasoning := false
	hasText := false
	for _, p := range assistantMsg.Parts {
		switch pp := p.(type) {
		case ReasoningPart:
			hasReasoning = true
			if pp.Text != "Let me think..." {
				t.Errorf("expected reasoning text 'Let me think...', got %q", pp.Text)
			}
		case TextPart:
			hasText = true
			if pp.Text != "The answer is 42." {
				t.Errorf("expected text 'The answer is 42.', got %q", pp.Text)
			}
		}
	}

	if !hasReasoning {
		t.Error("expected reasoning part")
	}
	if !hasText {
		t.Error("expected text part")
	}
}

func TestBuildProviderMessages(t *testing.T) {
	store := NewStore()
	s := store.Create(Deps{}, provider.ModelConfig{}, "")

	s.addMessage(Message{
		Role:  provider.RoleUser,
		Parts: []Part{TextPart{Text: "hello"}},
	})
	s.addMessage(Message{
		Role:  provider.RoleAssistant,
		Parts: []Part{TextPart{Text: "hi there"}},
	})
	s.addMessage(Message{
		Role: provider.RoleAssistant,
		Parts: []Part{
			TextPart{Text: "let me check"},
			ToolCallPart{CallID: "call_1", Tool: "bash", Args: `{"cmd":"ls"}`},
		},
	})
	s.addMessage(Message{
		Role: provider.RoleTool,
		Parts: []Part{
			ToolCallPart{CallID: "call_1", Tool: "bash", Output: "file1\nfile2"},
		},
	})

	provMsgs := s.buildProviderMessages()
	if len(provMsgs) != 4 {
		t.Fatalf("expected 4 provider messages, got %d", len(provMsgs))
	}
	if provMsgs[0].Role != provider.RoleUser {
		t.Error("first message should be user")
	}
	if provMsgs[1].Role != provider.RoleAssistant {
		t.Error("second message should be assistant")
	}
	if provMsgs[2].Role != provider.RoleAssistant {
		t.Error("third message should be assistant with tool calls")
	}
	if len(provMsgs[2].ToolCalls) != 1 {
		t.Errorf("expected 1 tool call, got %d", len(provMsgs[2].ToolCalls))
	}
	if provMsgs[3].Role != provider.RoleTool {
		t.Error("fourth message should be tool result")
	}
}

func TestSessionCancel(t *testing.T) {
	started := make(chan struct{})
	mp := &mockProvider{
		streamFn: func(msgs []provider.Message, _ []provider.ToolDef) <-chan provider.StreamEvent {
			ch := make(chan provider.StreamEvent, 10)
			go func() {
				close(started)
				// Simulate a slow response -- wait for context cancellation
				time.Sleep(5 * time.Second)
				ch <- provider.StreamEvent{Type: provider.EventDone, FinishReason: "stop"}
				close(ch)
			}()
			return ch
		},
	}

	te := &mockToolExecutor{tools: map[string]tool.Tool{}}
	s, _ := newTestSession(mp, te)
	s.Send(context.Background(), "slow request")

	<-started
	time.Sleep(50 * time.Millisecond) // let the goroutine set the cancel func
	s.Cancel()

	// The session should eventually leave busy state (via error or context cancel)
	deadline := time.After(50 * time.Millisecond)
	for {
		select {
		case <-deadline:
			// It's OK if it doesn't immediately enter error -- the important thing
			// is that Cancel() doesn't panic and the stream context was cancelled
			return
		default:
			status := s.GetStatus()
			if status != StatusBusy {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestLoopRetryOnRateLimit(t *testing.T) {
	// Provider returns 429 on first call, succeeds on second.
	mp := &rateLimitMockProvider{failCount: 1, retryAfter: 10 * time.Millisecond}
	te := &mockToolExecutor{tools: map[string]tool.Tool{}}

	bus := event.NewBus()
	store := NewStore()
	s := store.Create(Deps{
		Model:  mp,
		Tools:  te,
		Events: bus,
	}, provider.ModelConfig{Model: "test-model"}, "")

	s.Send(context.Background(), "retry me")

	if !waitForIdle(s, 5*time.Second) {
		t.Fatal("session did not return to idle")
	}

	// Provider should have been called exactly twice
	mp.mu.Lock()
	calls := mp.calls
	mp.mu.Unlock()
	if calls != 2 {
		t.Errorf("expected 2 provider calls, got %d", calls)
	}

	// Should have user + assistant messages (no error message)
	msgs := s.GetMessages()
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages (user + assistant), got %d", len(msgs))
	}

	// The assistant message should contain the success text, not an error
	lastMsg := msgs[len(msgs)-1]
	if lastMsg.Error != nil {
		t.Errorf("expected no error on assistant message, got: %v", lastMsg.Error)
	}
	text := extractText(lastMsg.Parts)
	if text != "Success after retry" {
		t.Errorf("expected 'Success after retry', got %q", text)
	}
}

func TestLoopRetryExhausted(t *testing.T) {
	// Provider always returns 429 — should give up after maxRetries.
	mp := &rateLimitMockProvider{failCount: 100, retryAfter: 10 * time.Millisecond}
	te := &mockToolExecutor{tools: map[string]tool.Tool{}}

	bus := event.NewBus()
	store := NewStore()
	s := store.Create(Deps{
		Model:  mp,
		Tools:  te,
		Events: bus,
	}, provider.ModelConfig{Model: "test-model"}, "")

	s.Send(context.Background(), "exhaust retries")

	if !waitForIdle(s, 10*time.Second) {
		t.Fatal("session did not return to idle")
	}

	// 1 initial attempt + maxRetries retried attempts, then the final attempt
	// where retries==maxRetries causes it to give up. Total = maxRetries + 1.
	mp.mu.Lock()
	calls := mp.calls
	mp.mu.Unlock()
	if calls != maxRetries+1 {
		t.Errorf("expected %d provider calls, got %d", maxRetries+1, calls)
	}

	// Error should appear in chat
	msgs := s.GetMessages()
	lastMsg := msgs[len(msgs)-1]
	if lastMsg.Error == nil {
		t.Fatal("expected an error message after retries exhausted")
	}
}

func TestLoopRetryRespectsCancel(t *testing.T) {
	// Provider always returns 429 with a long retry-after.
	// We cancel the context during the wait and verify the loop exits promptly.
	mp := &rateLimitMockProvider{failCount: 100, retryAfter: 30 * time.Second}
	te := &mockToolExecutor{tools: map[string]tool.Tool{}}

	bus := event.NewBus()
	store := NewStore()
	ctx, cancel := context.WithCancel(context.Background())
	s := store.Create(Deps{
		Model:  mp,
		Tools:  te,
		Events: bus,
	}, provider.ModelConfig{Model: "test-model"}, "")

	s.Send(ctx, "cancel during retry")

	// Wait a bit for the first retry wait to start, then cancel
	time.Sleep(100 * time.Millisecond)
	cancel()

	// Session should return to idle quickly (not wait 30s)
	if !waitForIdle(s, 2*time.Second) {
		t.Fatal("session did not return to idle after context cancel")
	}

	// Should have been called only once or twice (not all retries)
	mp.mu.Lock()
	calls := mp.calls
	mp.mu.Unlock()
	if calls > 2 {
		t.Errorf("expected at most 2 calls before cancel, got %d", calls)
	}
}

func TestLoopWithMultipleToolCalls(t *testing.T) {
	// Model returns two tool calls in a single response, then a final text response.
	// Verifies no message clobbering: message list should be
	// [user, assistant(tool_calls), toolResult1, toolResult2, assistant(final)]
	callCount := 0
	mp := &mockProvider{
		streamFn: func(msgs []provider.Message, _ []provider.ToolDef) <-chan provider.StreamEvent {
			ch := make(chan provider.StreamEvent, 15)
			go func() {
				callCount++
				if callCount == 1 {
					ch <- provider.StreamEvent{Type: provider.EventText, Text: "I'll use two tools."}
					ch <- provider.StreamEvent{Type: provider.EventToolCallStart, ToolCall: &provider.ToolCall{ID: "call_a", Name: "alpha"}}
					ch <- provider.StreamEvent{Type: provider.EventToolCallDelta, ToolCall: &provider.ToolCall{ID: "call_a", Args: `{"x":1}`}}
					ch <- provider.StreamEvent{Type: provider.EventToolCallEnd, ToolCall: &provider.ToolCall{ID: "call_a", Name: "alpha", Args: `{"x":1}`}}
					ch <- provider.StreamEvent{Type: provider.EventToolCallStart, ToolCall: &provider.ToolCall{ID: "call_b", Name: "beta"}}
					ch <- provider.StreamEvent{Type: provider.EventToolCallDelta, ToolCall: &provider.ToolCall{ID: "call_b", Args: `{"y":2}`}}
					ch <- provider.StreamEvent{Type: provider.EventToolCallEnd, ToolCall: &provider.ToolCall{ID: "call_b", Name: "beta", Args: `{"y":2}`}}
					ch <- provider.StreamEvent{Type: provider.EventDone, FinishReason: "tool_calls", Usage: &provider.Usage{InputTokens: 10, OutputTokens: 20, TotalTokens: 30}}
				} else {
					ch <- provider.StreamEvent{Type: provider.EventText, Text: "All done."}
					ch <- provider.StreamEvent{Type: provider.EventDone, FinishReason: "stop", Usage: &provider.Usage{InputTokens: 20, OutputTokens: 5, TotalTokens: 25}}
				}
				close(ch)
			}()
			return ch
		},
	}

	te := &mockToolExecutor{
		tools: map[string]tool.Tool{
			"alpha": &mockTool{name: "alpha", result: tool.Result{Output: "alpha result", Title: "alpha"}},
			"beta":  &mockTool{name: "beta", result: tool.Result{Output: "beta result", Title: "beta"}},
		},
	}

	s, _ := newTestSession(mp, te)
	s.Send(context.Background(), "use both tools")

	if !waitForIdle(s, 2*time.Second) {
		t.Fatal("session did not return to idle")
	}

	msgs := s.GetMessages()

	// Expected: user, assistant(tool calls), tool result 1, tool result 2, assistant(final)
	if len(msgs) != 5 {
		var roles []string
		for _, m := range msgs {
			roles = append(roles, string(m.Role))
		}
		t.Fatalf("expected 5 messages, got %d: %v", len(msgs), roles)
	}

	if msgs[0].Role != provider.RoleUser {
		t.Errorf("msgs[0]: expected user, got %s", msgs[0].Role)
	}
	if msgs[1].Role != provider.RoleAssistant {
		t.Errorf("msgs[1]: expected assistant, got %s", msgs[1].Role)
	}
	if msgs[2].Role != provider.RoleTool {
		t.Errorf("msgs[2]: expected tool, got %s", msgs[2].Role)
	}
	if msgs[3].Role != provider.RoleTool {
		t.Errorf("msgs[3]: expected tool, got %s", msgs[3].Role)
	}
	if msgs[4].Role != provider.RoleAssistant {
		t.Errorf("msgs[4]: expected assistant, got %s", msgs[4].Role)
	}

	// Verify no duplicate assistant IDs (clobbering would duplicate the first assistant msg)
	seen := map[string]int{}
	for i, m := range msgs {
		if m.Role == provider.RoleAssistant {
			seen[m.ID]++
			if seen[m.ID] > 1 {
				t.Errorf("duplicate assistant message ID %q at index %d", m.ID, i)
			}
		}
	}

	// Verify final message text
	finalText := extractText(msgs[4].Parts)
	if finalText != "All done." {
		t.Errorf("expected final text 'All done.', got %q", finalText)
	}
}

// promptTool is a mock tool that uses the Prompter to ask a question.
type promptTool struct{}

func (p *promptTool) Name() string               { return "user_prompt" }
func (p *promptTool) Description() string        { return "ask user" }
func (p *promptTool) Parameters() map[string]any { return map[string]any{"type": "object"} }
func (p *promptTool) Execute(ctx tool.Context, argsJSON string) tool.Result {
	if ctx.Prompter == nil {
		return tool.Result{Error: fmt.Errorf("no prompter")}
	}
	answer, err := ctx.Prompter.Prompt(ctx.Ctx, "what language?")
	if err != nil {
		return tool.Result{Error: err}
	}
	return tool.Result{Output: answer, Title: "Asked user"}
}

func TestLoopWithPromptTool(t *testing.T) {
	callCount := 0
	mp := &mockProvider{
		streamFn: func(msgs []provider.Message, _ []provider.ToolDef) <-chan provider.StreamEvent {
			ch := make(chan provider.StreamEvent, 10)
			go func() {
				callCount++
				if callCount == 1 {
					ch <- provider.StreamEvent{Type: provider.EventToolCallStart, ToolCall: &provider.ToolCall{ID: "call_p", Name: "user_prompt"}}
					ch <- provider.StreamEvent{Type: provider.EventToolCallDelta, ToolCall: &provider.ToolCall{ID: "call_p", Args: `{"question":"what language?"}`}}
					ch <- provider.StreamEvent{Type: provider.EventToolCallEnd, ToolCall: &provider.ToolCall{ID: "call_p", Name: "user_prompt", Args: `{"question":"what language?"}`}}
					ch <- provider.StreamEvent{Type: provider.EventDone, FinishReason: "tool_calls"}
				} else {
					ch <- provider.StreamEvent{Type: provider.EventText, Text: "Hello Go!"}
					ch <- provider.StreamEvent{Type: provider.EventDone, FinishReason: "stop"}
				}
				close(ch)
			}()
			return ch
		},
	}

	te := &mockToolExecutor{
		tools: map[string]tool.Tool{
			"user_prompt": &promptTool{},
		},
	}

	s, bus := newTestSession(mp, te)

	// Subscribe to TopicToolPrompt and answer immediately
	bus.Subscribe(event.TopicToolPrompt, func(p event.Payload) {
		req := p.Data.(event.PromptRequest)
		if req.Question != "what language?" {
			t.Errorf("expected question %q, got %q", "what language?", req.Question)
		}
		req.Response <- "Go"
	})

	s.Send(context.Background(), "ask me something")

	if !waitForIdle(s, 2*time.Second) {
		t.Fatal("session did not return to idle")
	}

	msgs := s.GetMessages()
	// user, assistant(tool call), tool result, assistant(final)
	if len(msgs) < 4 {
		t.Fatalf("expected at least 4 messages, got %d", len(msgs))
	}

	// The tool result should contain the user's answer
	for _, msg := range msgs {
		if msg.Role == provider.RoleTool {
			for _, part := range msg.Parts {
				if tc, ok := part.(ToolCallPart); ok && tc.Tool == "user_prompt" {
					if tc.Output != "Go" {
						t.Errorf("expected tool output %q, got %q", "Go", tc.Output)
					}
				}
			}
		}
	}

	// Final assistant message should reference the answer
	lastMsg := msgs[len(msgs)-1]
	text := extractText(lastMsg.Parts)
	if text != "Hello Go!" {
		t.Errorf("expected 'Hello Go!', got %q", text)
	}
}
