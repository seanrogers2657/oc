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
	ID         string
	Config     provider.ModelConfig
	WorkingDir string // inherited from where oc was invoked
	deps       Deps
	mu         sync.RWMutex
	messages   []Message
	status     SessionStatus
	tokens        provider.Usage
	contextTokens int // input tokens from the most recent request
	cancel        func() // cancel current stream
}

// GetMessages returns a snapshot of the conversation history.
// Parts slices are deep-copied so the caller cannot observe
// mid-mutation state from the session goroutine.
func (s *Session) GetMessages() []Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Message, len(s.messages))
	copy(out, s.messages)
	for i, msg := range out {
		if len(msg.Parts) > 0 {
			parts := make([]Part, len(msg.Parts))
			copy(parts, msg.Parts)
			out[i].Parts = parts
		}
	}
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

// GetContextTokens returns the input token count from the most recent request.
func (s *Session) GetContextTokens() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.contextTokens
}

// GetModel returns the current model ID.
func (s *Session) GetModel() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Config.Model
}

// SetModel changes the model used for subsequent requests.
func (s *Session) SetModel(model string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Config.Model = model
}

// AddLocalMessage appends a display-only message that is not sent to the model.
func (s *Session) AddLocalMessage(text string) {
	msg := Message{
		ID:        fmt.Sprintf("msg_%d", time.Now().UnixNano()),
		SessionID: s.ID,
		Role:      provider.RoleAssistant,
		Parts:     []Part{TextPart{Text: text}},
		CreatedAt: time.Now(),
		Local:     true,
	}
	s.addMessage(msg)
}

// ClearMessages removes all messages from the conversation history.
func (s *Session) ClearMessages() {
	s.mu.Lock()
	s.messages = nil
	s.mu.Unlock()
}

// addMessage appends a message and returns it.
func (s *Session) addMessage(msg Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append(s.messages, msg)
}

// updateMessage finds the message by ID and replaces it,
// deep-copying Parts to break backing-array sharing with the caller.
func (s *Session) updateMessage(msg Message) {
	if len(msg.Parts) > 0 {
		parts := make([]Part, len(msg.Parts))
		copy(parts, msg.Parts)
		msg.Parts = parts
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := len(s.messages) - 1; i >= 0; i-- {
		if s.messages[i].ID == msg.ID {
			s.messages[i] = msg
			return
		}
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
func (st *Store) Create(deps Deps, config provider.ModelConfig, workingDir string) *Session {
	id := fmt.Sprintf("session_%d", st.nextID.Add(1))
	s := &Session{
		ID:         id,
		Config:     config,
		WorkingDir: workingDir,
		deps:       deps,
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
