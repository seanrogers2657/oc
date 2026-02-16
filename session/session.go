package session

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/srogers/oc/provider"
)

// SessionStatus indicates the current state of a session.
type SessionStatus int

const (
	StatusIdle SessionStatus = iota // waiting for user input
	StatusBusy                      // model is streaming / tools are running
)

// Deps holds the injected port implementations for a session.
type Deps struct {
	Model  ModelClient
	Tools  ToolExecutor
	Events EventSink
}

// Session holds conversation state and orchestrates the model loop.
type Session struct {
	ID       string
	Config   provider.ModelConfig
	deps     Deps
	mu       sync.RWMutex
	messages []Message
	status   SessionStatus
	tokens   provider.Usage
	cancel   func() // cancel current stream
}

// GetMessages returns a snapshot of the conversation history.
func (s *Session) GetMessages() []Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Message, len(s.messages))
	copy(out, s.messages)
	return out
}

// GetStatus returns the current session status.
func (s *Session) GetStatus() SessionStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

// GetTokens returns total token usage for this session.
func (s *Session) GetTokens() provider.Usage {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tokens
}

// addMessage appends a message and returns it.
func (s *Session) addMessage(msg Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append(s.messages, msg)
}

// updateLastMessage replaces the last message.
func (s *Session) updateLastMessage(msg Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.messages) > 0 {
		s.messages[len(s.messages)-1] = msg
	}
}

// setStatus updates the session status.
func (s *Session) setStatus(status SessionStatus) {
	s.mu.Lock()
	s.status = status
	s.mu.Unlock()
}

// addErrorMessage appends an assistant message with the error so it appears in chat.
func (s *Session) addErrorMessage(err error) {
	msg := Message{
		ID:        fmt.Sprintf("msg_%d", time.Now().UnixNano()),
		SessionID: s.ID,
		Role:      provider.RoleAssistant,
		Parts:     []Part{TextPart{Text: err.Error()}},
		CreatedAt: time.Now(),
		Error:     err,
	}
	s.addMessage(msg)
}

// Store manages sessions (in-memory).
type Store struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	active   string
	nextID   atomic.Int64
}

// NewStore creates an empty session store.
func NewStore() *Store {
	return &Store{sessions: make(map[string]*Session)}
}

// Create makes a new session with the given dependencies and returns it.
func (st *Store) Create(deps Deps, config provider.ModelConfig) *Session {
	id := fmt.Sprintf("session_%d", st.nextID.Add(1))
	s := &Session{
		ID:     id,
		Config: config,
		deps:   deps,
	}

	st.mu.Lock()
	st.sessions[id] = s
	st.active = id
	st.mu.Unlock()

	return s
}

// Get returns a session by ID.
func (st *Store) Get(id string) *Session {
	st.mu.RLock()
	defer st.mu.RUnlock()
	return st.sessions[id]
}

// Active returns the currently active session.
func (st *Store) Active() *Session {
	st.mu.RLock()
	defer st.mu.RUnlock()
	return st.sessions[st.active]
}
