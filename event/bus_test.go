package event

import (
	"sync"
	"testing"
)

func TestBusPublishSubscribe(t *testing.T) {
	bus := NewBus()

	var received Payload
	bus.Subscribe(TopicPartDelta, func(p Payload) {
		received = p
	})

	bus.Publish(TopicPartDelta, "hello")

	if received.Topic != TopicPartDelta {
		t.Fatalf("topic = %q, want %q", received.Topic, TopicPartDelta)
	}
	if received.Data != "hello" {
		t.Fatalf("data = %v, want %q", received.Data, "hello")
	}
}

func TestBusNoSubscribers(t *testing.T) {
	bus := NewBus()
	// Should not panic
	bus.Publish(TopicError, "oops")
}

func TestBusMultipleSubscribers(t *testing.T) {
	bus := NewBus()

	count := 0
	bus.Subscribe(TopicMsgDone, func(p Payload) { count++ })
	bus.Subscribe(TopicMsgDone, func(p Payload) { count++ })
	bus.Subscribe(TopicMsgDone, func(p Payload) { count++ })

	bus.Publish(TopicMsgDone, nil)

	if count != 3 {
		t.Fatalf("expected 3 handlers called, got %d", count)
	}
}

func TestBusTopicIsolation(t *testing.T) {
	bus := NewBus()

	called := false
	bus.Subscribe(TopicToolStart, func(p Payload) { called = true })

	bus.Publish(TopicToolDone, nil) // different topic

	if called {
		t.Fatal("handler should not be called for different topic")
	}
}

func TestBusUnsubscribe(t *testing.T) {
	bus := NewBus()

	count := 0
	unsub := bus.Subscribe(TopicStatus, func(p Payload) { count++ })

	bus.Publish(TopicStatus, nil)
	if count != 1 {
		t.Fatalf("expected 1, got %d", count)
	}

	unsub()
	bus.Publish(TopicStatus, nil)
	if count != 1 {
		t.Fatalf("expected still 1 after unsub, got %d", count)
	}
}

func TestBusUnsubscribeIdempotent(t *testing.T) {
	bus := NewBus()
	unsub := bus.Subscribe(TopicError, func(p Payload) {})

	// Calling unsub multiple times should not panic
	unsub()
	unsub()
	unsub()
}

func TestBusConcurrentPublish(t *testing.T) {
	bus := NewBus()

	var mu sync.Mutex
	count := 0
	bus.Subscribe(TopicPartDelta, func(p Payload) {
		mu.Lock()
		count++
		mu.Unlock()
	})

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			bus.Publish(TopicPartDelta, nil)
		}()
	}
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	if count != 100 {
		t.Fatalf("expected 100, got %d", count)
	}
}

func TestBusConcurrentSubscribePublish(t *testing.T) {
	bus := NewBus()

	var wg sync.WaitGroup

	// Concurrent subscribes
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			bus.Subscribe(TopicPartDelta, func(p Payload) {})
		}()
	}

	// Concurrent publishes
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			bus.Publish(TopicPartDelta, "data")
		}()
	}

	wg.Wait()
	// No panic = pass
}
