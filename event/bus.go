package event

import "sync"

// Topic identifies a category of event.
type Topic string

const (
	TopicPartDelta Topic = "part.delta"
	TopicPartDone  Topic = "part.done"
	TopicToolStart Topic = "tool.start"
	TopicToolDone  Topic = "tool.done"
	TopicMsgDone   Topic = "message.done"
	TopicError     Topic = "error"
	TopicStatus    Topic = "status"
)

// Payload wraps a topic + arbitrary data for delivery to subscribers.
type Payload struct {
	Topic Topic
	Data  interface{}
}

// Handler is a callback invoked when a matching event is published.
type Handler func(Payload)

// Bus holds subscriptions and dispatches events. Thread-safe.
type Bus struct {
	mu   sync.RWMutex
	subs map[Topic][]Handler
}

// NewBus creates a new event bus.
func NewBus() *Bus {
	return &Bus{subs: make(map[Topic][]Handler)}
}

// Subscribe registers a handler for a topic. Returns an unsubscribe function.
func (b *Bus) Subscribe(topic Topic, handler Handler) func() {
	b.mu.Lock()
	b.subs[topic] = append(b.subs[topic], handler)
	idx := len(b.subs[topic]) - 1
	b.mu.Unlock()

	var once sync.Once
	return func() {
		once.Do(func() {
			b.mu.Lock()
			defer b.mu.Unlock()
			handlers := b.subs[topic]
			if idx < len(handlers) {
				// Swap with last and truncate
				handlers[idx] = handlers[len(handlers)-1]
				b.subs[topic] = handlers[:len(handlers)-1]
			}
		})
	}
}

// Publish dispatches a payload to all subscribers of the given topic.
func (b *Bus) Publish(topic Topic, data interface{}) {
	b.mu.RLock()
	handlers := make([]Handler, len(b.subs[topic]))
	copy(handlers, b.subs[topic])
	b.mu.RUnlock()

	p := Payload{Topic: topic, Data: data}
	for _, h := range handlers {
		h(p)
	}
}
