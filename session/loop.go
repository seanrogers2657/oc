package session

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/srogers/oc/event"
	"github.com/srogers/oc/provider"
	"github.com/srogers/oc/tool"
)

const (
	maxRetries       = 5
	initialBackoff   = 5 * time.Second
	maxBackoff       = 60 * time.Second
)

// Send processes a user message: appends it to history, then runs the
// stream-tool loop until the model stops or an error occurs.
// Runs in its own goroutine -- the TUI stays responsive via events.
func (s *Session) Send(ctx context.Context, text string) {
	// Create and append user message
	userMsg := Message{
		ID:        fmt.Sprintf("msg_%d", time.Now().UnixNano()),
		SessionID: s.ID,
		Role:      provider.RoleUser,
		Parts:     []Part{TextPart{Text: text}},
		CreatedAt: time.Now(),
	}
	s.addMessage(userMsg)

	// Set busy before spawning goroutine to avoid race with status checks
	s.setStatus(StatusBusy)
	s.deps.Events.Publish(event.TopicStatus, "busy")

	// Run the loop in a goroutine
	go s.runLoop(ctx)
}

// Cancel stops the current stream if one is running.
func (s *Session) Cancel() {
	s.mu.RLock()
	cancel := s.cancel
	s.mu.RUnlock()
	if cancel != nil {
		cancel()
	}
}

// runLoop is the core agentic loop: stream -> process -> tool execute -> repeat.
func (s *Session) runLoop(ctx context.Context) {

	defer func() {
		s.mu.Lock()
		s.cancel = nil
		s.mu.Unlock()
	}()

	retries := 0

	for {
		streamCtx, cancel := context.WithCancel(ctx)
		s.mu.Lock()
		s.cancel = cancel
		s.mu.Unlock()

		finished, err := s.streamOnce(streamCtx)
		cancel()

		if err != nil {
			var rle *provider.RateLimitError
			if errors.As(err, &rle) && retries < maxRetries {
				retries++
				wait := retryBackoff(rle.RetryAfter, retries)

				s.deps.Events.Publish(event.TopicStatus,
					fmt.Sprintf("Rate limited, retrying in %ds... (attempt %d/%d)",
						int(wait.Seconds()), retries, maxRetries))

				timer := time.NewTimer(wait)
				select {
				case <-ctx.Done():
					timer.Stop()
					s.setStatus(StatusIdle)
					s.deps.Events.Publish(event.TopicStatus, "idle")
					return
				case <-timer.C:
				}
				continue
			}

			// Non-retryable error, or retries exhausted
			s.addErrorMessage(err)
			s.setStatus(StatusIdle)
			s.deps.Events.Publish(event.TopicError, err.Error())
			return
		}

		// Successful stream resets the retry counter
		retries = 0

		if finished {
			s.setStatus(StatusIdle)
			s.deps.Events.Publish(event.TopicStatus, "idle")
			return
		}

		// Not finished means tool_calls -- loop continues
	}
}

// retryBackoff computes the wait duration for a rate limit retry.
// Uses the server's Retry-After if provided, otherwise exponential backoff.
func retryBackoff(retryAfter time.Duration, attempt int) time.Duration {
	if retryAfter > 0 {
		return retryAfter
	}
	backoff := initialBackoff
	for i := 1; i < attempt; i++ {
		backoff *= 2
	}
	if backoff > maxBackoff {
		backoff = maxBackoff
	}
	return backoff
}

// streamOnce runs one provider stream, processes all events, and returns
// (true, nil) if the model said "stop", (false, nil) if it said "tool_calls"
// (meaning we need to loop again), or (false, err) on error.
func (s *Session) streamOnce(ctx context.Context) (bool, error) {
	// Build provider message history from our rich messages
	providerMsgs := s.buildProviderMessages()
	toolDefs := s.deps.Tools.Defs()

	ch, err := s.deps.Model.Stream(ctx, s.Config, providerMsgs, toolDefs)
	if err != nil {
		return false, fmt.Errorf("stream: %w", err)
	}

	// Create a new assistant message to accumulate into
	assistantMsg := Message{
		ID:        fmt.Sprintf("msg_%d", time.Now().UnixNano()),
		SessionID: s.ID,
		Role:      provider.RoleAssistant,
		ModelID:   s.Config.Model,
		CreatedAt: time.Now(),
	}
	s.addMessage(assistantMsg)

	// Track active tool calls being accumulated
	activeToolCalls := make(map[int]*provider.ToolCall) // index -> tool call

	var finishReason string

	for ev := range ch {
		switch ev.Type {
		case provider.EventText:
			assistantMsg = s.appendText(&assistantMsg, ev.Text)
			s.updateMessage(assistantMsg)
			s.deps.Events.Publish(event.TopicPartDelta, ev.Text)

		case provider.EventReasoningStart:
			assistantMsg.Parts = append(assistantMsg.Parts, ReasoningPart{})
			s.updateMessage(assistantMsg)

		case provider.EventReasoningDelta:
			assistantMsg = s.appendReasoning(&assistantMsg, ev.Text)
			s.updateMessage(assistantMsg)

		case provider.EventReasoningEnd:
			s.deps.Events.Publish(event.TopicPartDone, "reasoning")

		case provider.EventToolCallStart:
			if ev.ToolCall != nil {
				idx := len(activeToolCalls)
				activeToolCalls[idx] = &provider.ToolCall{
					ID:   ev.ToolCall.ID,
					Name: ev.ToolCall.Name,
				}
			}

		case provider.EventToolCallDelta:
			if ev.ToolCall != nil {
				// Find the tool call to append args to
				for _, tc := range activeToolCalls {
					if tc.ID == ev.ToolCall.ID || (ev.ToolCall.ID == "" && len(activeToolCalls) == 1) {
						tc.Args += ev.ToolCall.Args
						break
					}
				}
			}

		case provider.EventToolCallEnd:
			// Tool call is complete -- add ToolCallPart to message
			if ev.ToolCall != nil {
				tc := ev.ToolCall
				part := ToolCallPart{
					CallID: tc.ID,
					Tool:   tc.Name,
					Args:   tc.Args,
					Status: ToolPending,
				}
				assistantMsg.Parts = append(assistantMsg.Parts, part)
				s.updateMessage(assistantMsg)
			}

		case provider.EventDone:
			finishReason = ev.FinishReason
			if ev.Usage != nil {
				assistantMsg.Tokens = *ev.Usage
				s.mu.Lock()
				s.tokens.InputTokens += ev.Usage.InputTokens
				s.tokens.OutputTokens += ev.Usage.OutputTokens
				s.tokens.TotalTokens += ev.Usage.TotalTokens
				s.mu.Unlock()
			}
			s.updateMessage(assistantMsg)

		case provider.EventError:
			if ev.Error != nil {
				assistantMsg.Error = ev.Error
				s.updateMessage(assistantMsg)
				return false, ev.Error
			}
		}
	}

	s.deps.Events.Publish(event.TopicMsgDone, assistantMsg.ID)

	// If the model requested tool calls, execute them
	if finishReason == "tool_calls" {
		if err := s.executeToolCalls(ctx, &assistantMsg); err != nil {
			return false, err
		}
		return false, nil // loop again
	}

	return true, nil // done
}

// executeToolCalls runs all tool calls in the assistant message and appends
// tool result messages to the conversation.
func (s *Session) executeToolCalls(ctx context.Context, assistantMsg *Message) error {
	for i, part := range assistantMsg.Parts {
		tc, ok := part.(ToolCallPart)
		if !ok || tc.Status != ToolPending {
			continue
		}

		// Mark as running
		tc.Status = ToolRunning
		tc.Start = time.Now()
		assistantMsg.Parts[i] = tc
		s.updateMessage(*assistantMsg)
		s.deps.Events.Publish(event.TopicToolStart, tc.Tool)

		// Execute the tool
		t, found := s.deps.Tools.Get(tc.Tool)
		if !found {
			tc.Status = ToolError
			tc.Error = fmt.Sprintf("unknown tool: %s", tc.Tool)
			tc.End = time.Now()
			assistantMsg.Parts[i] = tc
			s.updateMessage(*assistantMsg)
			s.deps.Events.Publish(event.TopicToolDone, tc.Tool)

			// Append error as tool result message
			toolMsg := Message{
				ID:        fmt.Sprintf("msg_%d", time.Now().UnixNano()),
				SessionID: s.ID,
				Role:      provider.RoleTool,
				Parts:     []Part{TextPart{Text: tc.Error}},
				CreatedAt: time.Now(),
			}
			s.addMessage(toolMsg)
			continue
		}

		result := t.Execute(tool.Context{
			SessionID:  s.ID,
			WorkingDir: s.WorkingDir,
			Ctx:        ctx,
		}, tc.Args)

		tc.End = time.Now()
		tc.Title = result.Title
		if result.Error != nil {
			tc.Status = ToolError
			tc.Error = result.Error.Error()
			tc.Output = result.Output
		} else {
			tc.Status = ToolCompleted
			tc.Output = result.Output
		}
		assistantMsg.Parts[i] = tc
		s.updateMessage(*assistantMsg)
		s.deps.Events.Publish(event.TopicToolDone, tc.Tool)

		// Append tool result to history
		content := result.Output
		if result.Error != nil {
			content = fmt.Sprintf("Error: %s\n%s", result.Error, result.Output)
		}
		toolMsg := Message{
			ID:        fmt.Sprintf("msg_%d", time.Now().UnixNano()),
			SessionID: s.ID,
			Role:      provider.RoleTool,
			Parts: []Part{ToolCallPart{
				CallID: tc.CallID,
				Tool:   tc.Tool,
				Output: content,
				Status: tc.Status,
			}},
			CreatedAt: time.Now(),
		}
		s.addMessage(toolMsg)
	}

	return nil
}

// appendText appends text to the last TextPart, or creates a new one.
func (s *Session) appendText(msg *Message, text string) Message {
	for i := len(msg.Parts) - 1; i >= 0; i-- {
		if tp, ok := msg.Parts[i].(TextPart); ok {
			tp.Text += text
			msg.Parts[i] = tp
			return *msg
		}
	}
	msg.Parts = append(msg.Parts, TextPart{Text: text})
	return *msg
}

// appendReasoning appends text to the last ReasoningPart.
func (s *Session) appendReasoning(msg *Message, text string) Message {
	for i := len(msg.Parts) - 1; i >= 0; i-- {
		if rp, ok := msg.Parts[i].(ReasoningPart); ok {
			rp.Text += text
			msg.Parts[i] = rp
			return *msg
		}
	}
	msg.Parts = append(msg.Parts, ReasoningPart{Text: text})
	return *msg
}

// buildProviderMessages converts the rich session messages to provider wire format.
func (s *Session) buildProviderMessages() []provider.Message {
	s.mu.RLock()
	msgs := make([]Message, len(s.messages))
	copy(msgs, s.messages)
	s.mu.RUnlock()

	var result []provider.Message
	for _, msg := range msgs {
		switch msg.Role {
		case provider.RoleUser:
			text := extractText(msg.Parts)
			result = append(result, provider.UserMessage(text))

		case provider.RoleAssistant:
			text := extractText(msg.Parts)
			toolCalls := extractToolCalls(msg.Parts)
			if len(toolCalls) > 0 {
				result = append(result, provider.AssistantToolCallMessage(text, toolCalls))
			} else {
				result = append(result, provider.AssistantMessage(text))
			}

		case provider.RoleTool:
			for _, part := range msg.Parts {
				if tc, ok := part.(ToolCallPart); ok {
					result = append(result, provider.ToolResultMessage(tc.CallID, tc.Tool, tc.Output))
				}
			}

		case provider.RoleSystem:
			text := extractText(msg.Parts)
			result = append(result, provider.SystemMessage(text))
		}
	}

	return result
}

// extractText concatenates all TextPart content from a message's parts.
func extractText(parts []Part) string {
	var text string
	for _, p := range parts {
		if tp, ok := p.(TextPart); ok {
			text += tp.Text
		}
	}
	return text
}

// extractToolCalls pulls ToolCallParts out as provider.ToolCall values.
func extractToolCalls(parts []Part) []provider.ToolCall {
	var calls []provider.ToolCall
	for _, p := range parts {
		if tc, ok := p.(ToolCallPart); ok {
			calls = append(calls, provider.ToolCall{
				ID:   tc.CallID,
				Name: tc.Tool,
				Args: tc.Args,
			})
		}
	}
	return calls
}
