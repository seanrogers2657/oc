package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"

	"github.com/srogers/oc/provider"
)

// ListModels fetches available models from the Anthropic /v1/models endpoint.
func ListModels(
	ctx context.Context,
	client http.Client,
	auth provider.Authenticator,
	baseUrl string,
) ([]string, error) {
	url := baseUrl + "/v1/models"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("anthropic-version", "2023-06-01")
	if err := auth.Authenticate(req); err != nil {
		return nil, fmt.Errorf("authenticate: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	models := make([]string, len(result.Data))
	for i, m := range result.Data {
		models[i] = m.ID
	}
	sort.Strings(models)
	return models, nil
}

// Stream sends a messages request and returns streaming events.
func Stream(
	ctx context.Context,
	cfg provider.ModelConfig,
	messages []provider.Message,
	tools []provider.ToolDef,
	client http.Client,
	auth provider.Authenticator,
	baseUrl string,
) (<-chan provider.StreamEvent, error) {
	body := buildRequest(cfg, messages, tools)

	reqBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	url := baseUrl + "/v1/messages"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")

	if err := auth.Authenticate(req); err != nil {
		return nil, fmt.Errorf("authenticate: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		errBody, _ := io.ReadAll(resp.Body)
		if resp.StatusCode == http.StatusTooManyRequests {
			return nil, &provider.RateLimitError{
				StatusCode: resp.StatusCode,
				Message:    string(errBody),
				RetryAfter: provider.ParseRetryAfter(resp),
			}
		}
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(errBody))
	}

	ch := make(chan provider.StreamEvent, 32)
	go readStream(resp.Body, ch)
	return ch, nil
}

func buildRequest(
	cfg provider.ModelConfig,
	messages []provider.Message,
	tools []provider.ToolDef,
) map[string]any {
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
		if m.Role == provider.RoleSystem {
			system = m.Content
			continue
		}

		if m.Role == provider.RoleAssistant && len(m.ToolCalls) > 0 {
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
		} else if m.Role == provider.RoleTool {
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
			defs = append(defs, provider.ToolDefToAnthropicSchema(t))
		}
		body["tools"] = defs
	}

	return body
}

// readStream parses the Anthropic SSE stream and sends StreamEvents.
func readStream(
	body io.ReadCloser,
	ch chan<- provider.StreamEvent,
) {
	defer close(ch)
	defer body.Close()

	reader := provider.NewSSEReader(body)

	// Track current content block for accumulating tool call args
	var currentBlock *anthropicContentBlock
	var usage *provider.Usage

	for {
		sse, err := reader.Next()
		if err == io.EOF {
			return
		}
		if err != nil {
			ch <- provider.StreamEvent{Type: provider.EventError, Error: err}
			return
		}

		switch sse.Event {
		case "message_start":
			var msg anthropicMessageStart
			if err := json.Unmarshal([]byte(sse.Data), &msg); err == nil && msg.Message.Usage != nil {
				usage = &provider.Usage{
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
				ch <- provider.StreamEvent{Type: provider.EventReasoningStart}
			case "tool_use":
				currentBlock = &anthropicContentBlock{
					Type: "tool_use",
					ID:   block.ContentBlock.ID,
					Name: block.ContentBlock.Name,
				}
				ch <- provider.StreamEvent{
					Type:     provider.EventToolCallStart,
					ToolCall: &provider.ToolCall{ID: block.ContentBlock.ID, Name: block.ContentBlock.Name},
				}
			}

		case "content_block_delta":
			var delta anthropicBlockDelta
			if err := json.Unmarshal([]byte(sse.Data), &delta); err != nil {
				continue
			}
			switch delta.Delta.Type {
			case "text_delta":
				ch <- provider.StreamEvent{Type: provider.EventText, Text: delta.Delta.Text}
			case "thinking_delta":
				ch <- provider.StreamEvent{Type: provider.EventReasoningDelta, Text: delta.Delta.Thinking}
			case "input_json_delta":
				if currentBlock != nil {
					currentBlock.PartialJSON += delta.Delta.PartialJSON
					ch <- provider.StreamEvent{
						Type:     provider.EventToolCallDelta,
						ToolCall: &provider.ToolCall{ID: currentBlock.ID, Name: currentBlock.Name, Args: delta.Delta.PartialJSON},
					}
				}
			}

		case "content_block_stop":
			if currentBlock != nil && currentBlock.Type == "tool_use" {
				ch <- provider.StreamEvent{
					Type:     provider.EventToolCallEnd,
					ToolCall: &provider.ToolCall{ID: currentBlock.ID, Name: currentBlock.Name, Args: currentBlock.PartialJSON},
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

				ch <- provider.StreamEvent{
					Type:         provider.EventDone,
					FinishReason: finishReason,
					Usage:        usage,
				}
			}

		case "message_stop":
			// Stream complete

		case "error":
			ch <- provider.StreamEvent{
				Type:  provider.EventError,
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
