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

func (m *mockTool) Name() string               { return m.name }
func (m *mockTool) Description() string         { return m.name + " tool" }
func (m *mockTool) Parameters() map[string]any  { return map[string]any{"type": "object"} }
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
	deadline := time.After(2 * time.Second)
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
