package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// AnthropicProvider implements Provider for the Anthropic Messages API.
type AnthropicProvider struct {
	apiKey string
	client *http.Client
}

// NewAnthropic creates a new Anthropic provider.
func NewAnthropic(apiKey string) *AnthropicProvider {
	return &AnthropicProvider{
		apiKey: apiKey,
		client: &http.Client{},
	}
}

func (p *AnthropicProvider) Name() string { return "anthropic" }

// Stream sends a messages request and returns streaming events.
func (p *AnthropicProvider) Stream(ctx context.Context, cfg ModelConfig, messages []Message, tools []ToolDef) (<-chan StreamEvent, error) {
	body := p.buildRequest(cfg, messages, tools)

	reqBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(reqBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		errBody, _ := io.ReadAll(resp.Body)
		if resp.StatusCode == http.StatusTooManyRequests {
			return nil, &RateLimitError{
				StatusCode: resp.StatusCode,
				Message:    string(errBody),
				RetryAfter: parseRetryAfter(resp),
			}
		}
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(errBody))
	}

	ch := make(chan StreamEvent, 32)
	go p.readStream(resp.Body, ch)
	return ch, nil
}

func (p *AnthropicProvider) buildRequest(cfg ModelConfig, messages []Message, tools []ToolDef) map[string]any {
	body := map[string]any{
		"model":  cfg.Model,
		"stream": true,
	}

	maxTokens := 4096
	if cfg.MaxTokens != nil {
		maxTokens = *cfg.MaxTokens
	}
	body["max_tokens"] = maxTokens

	if cfg.Temperature != nil {
		body["temperature"] = *cfg.Temperature
	}
	if cfg.TopP != nil {
		body["top_p"] = *cfg.TopP
	}

	// Extract system message and convert the rest
	var system string
	var apiMsgs []map[string]any

	for _, m := range messages {
		if m.Role == RoleSystem {
			system = m.Content
			continue
		}

		if m.Role == RoleAssistant && len(m.ToolCalls) > 0 {
			// Assistant message with tool use
			var content []map[string]any
			if m.Content != "" {
				content = append(content, map[string]any{
					"type": "text",
					"text": m.Content,
				})
			}
			for _, tc := range m.ToolCalls {
				var input any
				if err := json.Unmarshal([]byte(tc.Args), &input); err != nil {
					input = map[string]any{}
				}
				content = append(content, map[string]any{
					"type":  "tool_use",
					"id":    tc.ID,
					"name":  tc.Name,
					"input": input,
				})
			}
			apiMsgs = append(apiMsgs, map[string]any{
				"role":    "assistant",
				"content": content,
			})
		} else if m.Role == RoleTool {
			// Tool result
			apiMsgs = append(apiMsgs, map[string]any{
				"role": "user",
				"content": []map[string]any{
					{
						"type":        "tool_result",
						"tool_use_id": m.ToolCallID,
						"content":     m.Content,
					},
				},
			})
		} else {
			apiMsgs = append(apiMsgs, map[string]any{
				"role":    string(m.Role),
				"content": m.Content,
			})
		}
	}

	if system != "" {
		body["system"] = system
	}
	body["messages"] = apiMsgs

	// Convert tools
	if len(tools) > 0 {
		var defs []map[string]any
		for _, t := range tools {
			defs = append(defs, ToolDefToAnthropicSchema(t))
		}
		body["tools"] = defs
	}

	return body
}

// readStream parses the Anthropic SSE stream and sends StreamEvents.
func (p *AnthropicProvider) readStream(body io.ReadCloser, ch chan<- StreamEvent) {
	defer close(ch)
	defer body.Close()

	reader := NewSSEReader(body)

	// Track current content block for accumulating tool call args
	var currentBlock *anthropicContentBlock
	var usage *Usage

	for {
		sse, err := reader.Next()
		if err == io.EOF {
			return
		}
		if err != nil {
			ch <- StreamEvent{Type: EventError, Error: err}
			return
		}

		switch sse.Event {
		case "message_start":
			var msg anthropicMessageStart
			if err := json.Unmarshal([]byte(sse.Data), &msg); err == nil && msg.Message.Usage != nil {
				usage = &Usage{
					InputTokens: msg.Message.Usage.InputTokens,
				}
			}

		case "content_block_start":
			var block anthropicBlockStart
			if err := json.Unmarshal([]byte(sse.Data), &block); err != nil {
				continue
			}
			switch block.ContentBlock.Type {
			case "text":
				// Text block starting
			case "thinking":
				ch <- StreamEvent{Type: EventReasoningStart}
			case "tool_use":
				currentBlock = &anthropicContentBlock{
					Type: "tool_use",
					ID:   block.ContentBlock.ID,
					Name: block.ContentBlock.Name,
				}
				ch <- StreamEvent{
					Type:     EventToolCallStart,
					ToolCall: &ToolCall{ID: block.ContentBlock.ID, Name: block.ContentBlock.Name},
				}
			}

		case "content_block_delta":
			var delta anthropicBlockDelta
			if err := json.Unmarshal([]byte(sse.Data), &delta); err != nil {
				continue
			}
			switch delta.Delta.Type {
			case "text_delta":
				ch <- StreamEvent{Type: EventText, Text: delta.Delta.Text}
			case "thinking_delta":
				ch <- StreamEvent{Type: EventReasoningDelta, Text: delta.Delta.Thinking}
			case "input_json_delta":
				if currentBlock != nil {
					currentBlock.PartialJSON += delta.Delta.PartialJSON
					ch <- StreamEvent{
						Type:     EventToolCallDelta,
						ToolCall: &ToolCall{ID: currentBlock.ID, Name: currentBlock.Name, Args: delta.Delta.PartialJSON},
					}
				}
			}

		case "content_block_stop":
			if currentBlock != nil && currentBlock.Type == "tool_use" {
				ch <- StreamEvent{
					Type:     EventToolCallEnd,
					ToolCall: &ToolCall{ID: currentBlock.ID, Name: currentBlock.Name, Args: currentBlock.PartialJSON},
				}
				currentBlock = nil
			}
			// If we were in a thinking block, signal end
			// (We don't track thinking blocks explicitly, but this is fine)

		case "message_delta":
			var delta anthropicMsgDelta
			if err := json.Unmarshal([]byte(sse.Data), &delta); err == nil {
				if delta.Usage != nil && usage != nil {
					usage.OutputTokens = delta.Usage.OutputTokens
					usage.TotalTokens = usage.InputTokens + usage.OutputTokens
				}

				finishReason := "stop"
				if delta.Delta.StopReason == "tool_use" {
					finishReason = "tool_calls"
				} else if delta.Delta.StopReason != "" {
					finishReason = delta.Delta.StopReason
				}

				ch <- StreamEvent{
					Type:         EventDone,
					FinishReason: finishReason,
					Usage:        usage,
				}
			}

		case "message_stop":
			// Stream complete

		case "error":
			ch <- StreamEvent{
				Type:  EventError,
				Error: fmt.Errorf("anthropic stream error: %s", sse.Data),
			}
			return
		}
	}
}

// --- Anthropic JSON structures ---

type anthropicContentBlock struct {
	Type        string
	ID          string
	Name        string
	PartialJSON string // accumulated input JSON for tool_use
}

type anthropicMessageStart struct {
	Message struct {
		Usage *anthropicInputUsage `json:"usage,omitempty"`
	} `json:"message"`
}

type anthropicInputUsage struct {
	InputTokens int `json:"input_tokens"`
}

type anthropicBlockStart struct {
	ContentBlock struct {
		Type string `json:"type"`
		ID   string `json:"id,omitempty"`
		Name string `json:"name,omitempty"`
		Text string `json:"text,omitempty"`
	} `json:"content_block"`
}

type anthropicBlockDelta struct {
	Delta struct {
		Type        string `json:"type"`
		Text        string `json:"text,omitempty"`
		Thinking    string `json:"thinking,omitempty"`
		PartialJSON string `json:"partial_json,omitempty"`
	} `json:"delta"`
}

type anthropicMsgDelta struct {
	Delta struct {
		StopReason string `json:"stop_reason,omitempty"`
	} `json:"delta"`
	Usage *anthropicOutputUsage `json:"usage,omitempty"`
}

type anthropicOutputUsage struct {
	OutputTokens int `json:"output_tokens"`
}
