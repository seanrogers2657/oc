package provider

// drains a StreamEvent channel into a slice.
func CollectStreamEvents(ch <-chan StreamEvent) []StreamEvent {
	var events []StreamEvent
	for ev := range ch {
		events = append(events, ev)
	}
	return events
}
