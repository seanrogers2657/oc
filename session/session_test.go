package session

import (
	"fmt"
	"testing"

	"github.com/srogers/oc/provider"
)

func TestStoreCreateAndGet(t *testing.T) {
	store := NewStore()
	s := store.Create(Deps{}, provider.ModelConfig{Model: "test-model"}, "")

	if s.ID == "" {
		t.Fatal("session should have an ID")
	}
	if s.Config.Model != "test-model" {
		t.Fatalf("expected model 'test-model', got %q", s.Config.Model)
	}

	got := store.Get(s.ID)
	if got != s {
		t.Fatal("Get should return the same session")
	}
}

func TestStoreActive(t *testing.T) {
	store := NewStore()
	s1 := store.Create(Deps{}, provider.ModelConfig{Model: "m1"}, "")
	_ = s1
	s2 := store.Create(Deps{}, provider.ModelConfig{Model: "m2"}, "")

	active := store.Active()
	if active != s2 {
		t.Fatal("Active should return the most recently created session")
	}
}

func TestStoreGetMissing(t *testing.T) {
	store := NewStore()
	if store.Get("nonexistent") != nil {
		t.Fatal("expected nil for missing session")
	}
}

func TestStoreActiveEmpty(t *testing.T) {
	store := NewStore()
	if store.Active() != nil {
		t.Fatal("expected nil when no sessions exist")
	}
}

func TestSessionInitialStatus(t *testing.T) {
	store := NewStore()
	s := store.Create(Deps{}, provider.ModelConfig{}, "")

	if s.GetStatus() != StatusIdle {
		t.Fatalf("expected StatusIdle, got %d", s.GetStatus())
	}
}

func TestSessionGetMessages(t *testing.T) {
	store := NewStore()
	s := store.Create(Deps{}, provider.ModelConfig{}, "")

	msgs := s.GetMessages()
	if len(msgs) != 0 {
		t.Fatalf("expected 0 messages, got %d", len(msgs))
	}

	// Add a message directly
	s.addMessage(Message{
		ID:   "msg_1",
		Role: provider.RoleUser,
		Parts: []Part{TextPart{Text: "hello"}},
	})

	msgs = s.GetMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].ID != "msg_1" {
		t.Fatalf("expected msg_1, got %s", msgs[0].ID)
	}
}

func TestSessionGetMessagesReturnsSnapshot(t *testing.T) {
	store := NewStore()
	s := store.Create(Deps{}, provider.ModelConfig{}, "")

	s.addMessage(Message{ID: "msg_1", Role: provider.RoleUser})
	snap := s.GetMessages()

	// Mutating the snapshot should not affect the session
	snap[0].ID = "mutated"

	msgs := s.GetMessages()
	if msgs[0].ID != "msg_1" {
		t.Fatal("GetMessages should return an independent copy")
	}
}

func TestSessionGetTokens(t *testing.T) {
	store := NewStore()
	s := store.Create(Deps{}, provider.ModelConfig{}, "")

	tokens := s.GetTokens()
	if tokens.TotalTokens != 0 {
		t.Fatalf("expected 0 tokens, got %d", tokens.TotalTokens)
	}
}

func TestSessionAddErrorMessage(t *testing.T) {
	store := NewStore()
	s := store.Create(Deps{}, provider.ModelConfig{}, "")

	testErr := fmt.Errorf("connection refused")
	s.addErrorMessage(testErr)

	msgs := s.GetMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	msg := msgs[0]
	if msg.Role != provider.RoleAssistant {
		t.Fatalf("expected assistant role, got %s", msg.Role)
	}
	if msg.Error == nil {
		t.Fatal("expected error to be set on message")
	}
	if msg.Error.Error() != "connection refused" {
		t.Fatalf("expected 'connection refused', got %q", msg.Error.Error())
	}
	// Should also have a TextPart with the error text
	text := extractText(msg.Parts)
	if text != "connection refused" {
		t.Fatalf("expected text part 'connection refused', got %q", text)
	}
}

func TestSessionStatusTransitions(t *testing.T) {
	store := NewStore()
	s := store.Create(Deps{}, provider.ModelConfig{}, "")

	if s.GetStatus() != StatusIdle {
		t.Fatalf("expected StatusIdle initially")
	}

	s.setStatus(StatusBusy)
	if s.GetStatus() != StatusBusy {
		t.Fatalf("expected StatusBusy after setStatus")
	}

	s.setStatus(StatusIdle)
	if s.GetStatus() != StatusIdle {
		t.Fatalf("expected StatusIdle after setStatus back")
	}
}

func TestUpdateMessageByID(t *testing.T) {
	store := NewStore()
	s := store.Create(Deps{}, provider.ModelConfig{}, "")

	s.addMessage(Message{ID: "msg_1", Role: provider.RoleUser, Parts: []Part{TextPart{Text: "first"}}})
	s.addMessage(Message{ID: "msg_2", Role: provider.RoleAssistant, Parts: []Part{TextPart{Text: "second"}}})
	s.addMessage(Message{ID: "msg_3", Role: provider.RoleTool, Parts: []Part{TextPart{Text: "third"}}})

	// Update the middle message by ID
	s.updateMessage(Message{ID: "msg_2", Role: provider.RoleAssistant, Parts: []Part{TextPart{Text: "updated"}}})

	msgs := s.GetMessages()
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(msgs))
	}
	text := extractText(msgs[1].Parts)
	if text != "updated" {
		t.Fatalf("expected 'updated', got %q", text)
	}
	// Other messages should be untouched
	if extractText(msgs[0].Parts) != "first" {
		t.Fatal("msg_1 should be unchanged")
	}
	if extractText(msgs[2].Parts) != "third" {
		t.Fatal("msg_3 should be unchanged")
	}
}

func TestUpdateMessagePartsIsolation(t *testing.T) {
	store := NewStore()
	s := store.Create(Deps{}, provider.ModelConfig{}, "")

	msg := Message{ID: "msg_1", Role: provider.RoleAssistant, Parts: []Part{TextPart{Text: "original"}}}
	s.addMessage(msg)

	// Update via updateMessage
	msg.Parts = []Part{TextPart{Text: "snapshot"}}
	s.updateMessage(msg)

	// Mutate the caller's Parts after updateMessage
	msg.Parts[0] = TextPart{Text: "mutated by caller"}

	// The stored message should still have "snapshot"
	msgs := s.GetMessages()
	text := extractText(msgs[0].Parts)
	if text != "snapshot" {
		t.Fatalf("expected stored text 'snapshot', got %q (caller mutation leaked)", text)
	}
}

func TestUpdateMessageConcurrentAccess(t *testing.T) {
	store := NewStore()
	s := store.Create(Deps{}, provider.ModelConfig{}, "")

	s.addMessage(Message{ID: "msg_1", Role: provider.RoleAssistant, Parts: []Part{TextPart{Text: "init"}}})

	done := make(chan struct{})
	// Writer goroutine: repeatedly update the message
	go func() {
		defer close(done)
		for i := 0; i < 1000; i++ {
			s.updateMessage(Message{
				ID:    "msg_1",
				Role:  provider.RoleAssistant,
				Parts: []Part{TextPart{Text: fmt.Sprintf("v%d", i)}},
			})
		}
	}()

	// Reader goroutine: repeatedly snapshot messages
	for i := 0; i < 1000; i++ {
		msgs := s.GetMessages()
		if len(msgs) != 1 {
			t.Fatalf("expected 1 message, got %d", len(msgs))
		}
		// Access parts to trigger race detector if sharing exists
		_ = extractText(msgs[0].Parts)
	}

	<-done
}

func TestUniqueSessionIDs(t *testing.T) {
	store := NewStore()
	s1 := store.Create(Deps{}, provider.ModelConfig{}, "")
	s2 := store.Create(Deps{}, provider.ModelConfig{}, "")
	if s1.ID == s2.ID {
		t.Fatal("session IDs should be unique")
	}
}
