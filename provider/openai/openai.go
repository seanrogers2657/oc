package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/srogers/oc/provider"
)

// OpenAIProvider implements Provider for OpenAI-compatible APIs.
type OpenAIProvider struct {
	name    string
	apiKey  string
	baseURL string
	client  *http.Client
}

// NewOpenAI creates a new OpenAI-compatible provider.
// The name parameter controls what Name() returns (e.g. "openai", "ollama").
// An optional *http.Client can be passed; if nil, http.DefaultClient is used.
func NewOpenAI(name, apiKey, baseURL string, client ...*http.Client) *OpenAIProvider {
	if name == "" {
		name = "openai"
	}
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}
	c := &http.Client{}
	if len(client) > 0 && client[0] != nil {
		c = client[0]
	}
	return &OpenAIProvider{
		name:    name,
		apiKey:  apiKey,
		baseURL: baseURL,
		client:  c,
	}
}

func (p *OpenAIProvider) Name() string { return p.name }

// ListModels fetches available models from the OpenAI-compatible /v1/models endpoint.
func (p *OpenAIProvider) ListModels(ctx context.Context) ([]string, error) {
	url := p.buildModelsURL()
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	resp, err := p.client.Do(req)
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

// buildModelsURL returns the models list endpoint, avoiding double /v1 paths.
func (p *OpenAIProvider) buildModelsURL() string {
	base := p.baseURL
	if strings.HasSuffix(base, "/v1") || strings.HasSuffix(base, "/v1/") {
		base = strings.TrimRight(base, "/")
		return base + "/models"
	}
	return base + "/v1/models"
}

// buildURL returns the chat completions endpoint, avoiding double /v1 paths.
func (p *OpenAIProvider) buildURL() string {
	base := p.baseURL
	if strings.HasSuffix(base, "/v1") || strings.HasSuffix(base, "/v1/") {
		base = strings.TrimRight(base, "/")
		return base + "/chat/completions"
	}
	return base + "/v1/chat/completions"
}

// Stream sends a chat completion request and returns streaming events.
func (p *OpenAIProvider) Stream(
	ctx context.Context,
	cfg provider.ModelConfig,
	messages []provider.Message,
	tools []provider.ToolDef,
) (<-chan provider.StreamEvent, error) {
	body := p.buildRequest(cfg, messages, tools)

	reqBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := p.buildURL()
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	resp, err := p.client.Do(req)
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
	go p.readStream(resp.Body, ch)
	return ch, nil
}

func (p *OpenAIProvider) buildRequest(
	cfg provider.ModelConfig,
	messages []provider.Message,
	tools []provider.ToolDef,
) map[string]any {
	body := map[string]any{
		"model":          cfg.Model,
		"stream":         true,
		"stream_options": map[string]any{"include_usage": true},
	}

	if cfg.Temperature != nil {
		body["temperature"] = *cfg.Temperature
	}
	if cfg.MaxTokens != nil {
		body["max_tokens"] = *cfg.MaxTokens
	}
	if cfg.TopP != nil {
		body["top_p"] = *cfg.TopP
	}

	// Convert messages
	var msgs []map[string]any
	for _, m := range messages {
		msg := map[string]any{
			"role": string(m.Role),
		}

		if m.Role == provider.RoleTool {
			msg["content"] = m.Content
			msg["tool_call_id"] = m.ToolCallID
		} else if m.Role == provider.RoleAssistant && len(m.ToolCalls) > 0 {
			if m.Content != "" {
				msg["content"] = m.Content
			} else {
				msg["content"] = nil
			}
			var tcs []map[string]any
			for _, tc := range m.ToolCalls {
				tcs = append(tcs, map[string]any{
					"id":   tc.ID,
					"type": "function",
					"function": map[string]any{
						"name":      tc.Name,
						"arguments": tc.Args,
					},
				})
			}
			msg["tool_calls"] = tcs
		} else {
			msg["content"] = m.Content
		}

		msgs = append(msgs, msg)
	}
	body["messages"] = msgs

	// Convert tools
	if len(tools) > 0 {
		var defs []map[string]any
		for _, t := range tools {
			defs = append(defs, provider.ToolDefToFunctionSchema(t))
		}
		body["tools"] = defs
	}

	return body
}

// readStream parses the SSE stream and sends StreamEvents.
func (p *OpenAIProvider) readStream(body io.ReadCloser, ch chan<- provider.StreamEvent) {
	defer close(ch)
	defer body.Close()

	reader := provider.NewSSEReader(body)

	// Accumulate tool calls by index
	toolCalls := make(map[int]*provider.ToolCall)

	for {
		sse, err := reader.Next()
		if err == io.EOF {
			return
		}
		if err != nil {
			ch <- provider.StreamEvent{Type: provider.EventError, Error: err}
			return
		}

		if sse.Data == "[DONE]" {
			return
		}

		var chunk openaiChunk
		if err := json.Unmarshal([]byte(sse.Data), &chunk); err != nil {
			ch <- provider.StreamEvent{Type: provider.EventError, Error: fmt.Errorf("parse chunk: %w", err)}
			return
		}

		if len(chunk.Choices) == 0 {
			// Usage-only chunk (some providers send this at the end)
			if chunk.Usage != nil {
				ch <- provider.StreamEvent{
					Type: provider.EventDone,
					Usage: &provider.Usage{
						InputTokens:  chunk.Usage.PromptTokens,
						OutputTokens: chunk.Usage.CompletionTokens,
						TotalTokens:  chunk.Usage.TotalTokens,
					},
				}
			}
			continue
		}

		choice := chunk.Choices[0]
		delta := choice.Delta

		// Text content
		if delta.Content != "" {
			ch <- provider.StreamEvent{Type: provider.EventText, Text: delta.Content}
		}

		// Reasoning / thinking content (some models emit this)
		reasoning := delta.Reasoning
		if reasoning == "" {
			reasoning = delta.ReasoningOllama
		}
		if reasoning != "" {
			ch <- provider.StreamEvent{Type: provider.EventReasoningDelta, Text: reasoning}
		}

		// Tool calls (accumulated by index)
		for _, tc := range delta.ToolCalls {
			existing, ok := toolCalls[tc.Index]
			if !ok {
				// New tool call
				existing = &provider.ToolCall{
					ID:   tc.ID,
					Name: tc.Function.Name,
				}
				toolCalls[tc.Index] = existing
				ch <- provider.StreamEvent{
					Type:     provider.EventToolCallStart,
					ToolCall: &provider.ToolCall{ID: tc.ID, Name: tc.Function.Name},
				}
			}

			// Accumulate args
			if tc.Function.Arguments != "" {
				existing.Args += tc.Function.Arguments
				ch <- provider.StreamEvent{
					Type:     provider.EventToolCallDelta,
					ToolCall: &provider.ToolCall{ID: existing.ID, Name: existing.Name, Args: tc.Function.Arguments},
				}
			}
		}

		// Finish reason
		if choice.FinishReason != "" {
			// Emit tool call end events
			if choice.FinishReason == "tool_calls" {
				for _, tc := range toolCalls {
					ch <- provider.StreamEvent{
						Type:     provider.EventToolCallEnd,
						ToolCall: &provider.ToolCall{ID: tc.ID, Name: tc.Name, Args: tc.Args},
					}
				}
			}

			usage := (*provider.Usage)(nil)
			if chunk.Usage != nil {
				usage = &provider.Usage{
					InputTokens:  chunk.Usage.PromptTokens,
					OutputTokens: chunk.Usage.CompletionTokens,
					TotalTokens:  chunk.Usage.TotalTokens,
				}
			}

			ch <- provider.StreamEvent{
				Type:         provider.EventDone,
				FinishReason: choice.FinishReason,
				Usage:        usage,
			}

			// Reset tool calls for potential next round
			toolCalls = make(map[int]*provider.ToolCall)
		}
	}
}

// --- OpenAI JSON structures ---

type openaiChunk struct {
	Choices []openaiChoice `json:"choices"`
	Usage   *openaiUsage   `json:"usage,omitempty"`
}

type openaiChoice struct {
	Delta        openaiDelta `json:"delta"`
	FinishReason string      `json:"finish_reason,omitempty"`
}

type openaiDelta struct {
	Content         string            `json:"content,omitempty"`
	Reasoning       string            `json:"reasoning_content,omitempty"`
	ReasoningOllama string            `json:"reasoning,omitempty"`
	ToolCalls       []openaiToolDelta `json:"tool_calls,omitempty"`
}

type openaiToolDelta struct {
	Index    int            `json:"index"`
	ID       string         `json:"id,omitempty"`
	Function openaiFunction `json:"function"`
}

type openaiFunction struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

type openaiUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}
