package provider

import (
	"net/http"
	"testing"
)

func TestMessageConstructors(t *testing.T) {
	t.Run("SystemMessage", func(t *testing.T) {
		m := SystemMessage("you are helpful")
		if m.Role != RoleSystem {
			t.Errorf("Role = %v, want %v", m.Role, RoleSystem)
		}
		if m.Content != "you are helpful" {
			t.Errorf("Content = %q", m.Content)
		}
	})

	t.Run("UserMessage", func(t *testing.T) {
		m := UserMessage("hello")
		if m.Role != RoleUser {
			t.Errorf("Role = %v, want %v", m.Role, RoleUser)
		}
		if m.Content != "hello" {
			t.Errorf("Content = %q", m.Content)
		}
	})

	t.Run("AssistantMessage", func(t *testing.T) {
		m := AssistantMessage("hi there")
		if m.Role != RoleAssistant {
			t.Errorf("Role = %v, want %v", m.Role, RoleAssistant)
		}
		if m.Content != "hi there" {
			t.Errorf("Content = %q", m.Content)
		}
		if len(m.ToolCalls) != 0 {
			t.Errorf("ToolCalls should be empty")
		}
	})

	t.Run("AssistantToolCallMessage", func(t *testing.T) {
		tcs := []ToolCall{{ID: "c1", Name: "bash", Args: `{"cmd":"ls"}`}}
		m := AssistantToolCallMessage("checking", tcs)
		if m.Role != RoleAssistant {
			t.Errorf("Role = %v", m.Role)
		}
		if m.Content != "checking" {
			t.Errorf("Content = %q", m.Content)
		}
		if len(m.ToolCalls) != 1 || m.ToolCalls[0].ID != "c1" {
			t.Errorf("ToolCalls = %v", m.ToolCalls)
		}
	})

	t.Run("ToolResultMessage", func(t *testing.T) {
		m := ToolResultMessage("c1", "bash", "output")
		if m.Role != RoleTool {
			t.Errorf("Role = %v", m.Role)
		}
		if m.ToolCallID != "c1" {
			t.Errorf("ToolCallID = %q", m.ToolCallID)
		}
		if m.Name != "bash" {
			t.Errorf("Name = %q", m.Name)
		}
		if m.Content != "output" {
			t.Errorf("Content = %q", m.Content)
		}
	})
}

func TestToolDefToFunctionSchema(t *testing.T) {
	td := ToolDef{
		Name:        "bash",
		Description: "Run a shell command",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command": map[string]any{"type": "string"},
			},
			"required": []string{"command"},
		},
	}

	schema := ToolDefToFunctionSchema(td)

	if schema["type"] != "function" {
		t.Errorf("type = %v", schema["type"])
	}
	fn := schema["function"].(map[string]any)
	if fn["name"] != "bash" {
		t.Errorf("name = %v", fn["name"])
	}
	if fn["description"] != "Run a shell command" {
		t.Errorf("description = %v", fn["description"])
	}
	params := fn["parameters"].(map[string]any)
	if params["type"] != "object" {
		t.Errorf("parameters.type = %v", params["type"])
	}
}

func TestToolDefToFunctionSchemaNilParams(t *testing.T) {
	td := ToolDef{Name: "test", Description: "test tool"}

	schema := ToolDefToFunctionSchema(td)
	fn := schema["function"].(map[string]any)
	params := fn["parameters"].(map[string]any)

	if params["type"] != "object" {
		t.Errorf("default parameters.type = %v, want 'object'", params["type"])
	}
}

func TestToolDefToAnthropicSchema(t *testing.T) {
	td := ToolDef{
		Name:        "read",
		Description: "Read a file",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{"type": "string"},
			},
		},
	}

	schema := ToolDefToAnthropicSchema(td)

	if schema["name"] != "read" {
		t.Errorf("name = %v", schema["name"])
	}
	if schema["description"] != "Read a file" {
		t.Errorf("description = %v", schema["description"])
	}
	if _, ok := schema["input_schema"]; !ok {
		t.Fatal("missing input_schema")
	}
	// Should NOT have "type": "function" wrapper (that's OpenAI format)
	if _, ok := schema["type"]; ok {
		t.Error("Anthropic schema should not have 'type' field")
	}
}

func TestToolDefToAnthropicSchemaNilParams(t *testing.T) {
	td := ToolDef{Name: "test", Description: "test tool"}

	schema := ToolDefToAnthropicSchema(td)
	inputSchema := schema["input_schema"].(map[string]any)

	if inputSchema["type"] != "object" {
		t.Errorf("default input_schema.type = %v, want 'object'", inputSchema["type"])
	}
}

func TestMarshalJSON(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		b := MarshalJSON(map[string]string{"key": "value"})
		if string(b) != `{"key":"value"}` {
			t.Errorf("got %s", b)
		}
	})

	t.Run("error returns empty object", func(t *testing.T) {
		// Channels can't be marshaled
		b := MarshalJSON(make(chan int))
		if string(b) != "{}" {
			t.Errorf("got %s, want {}", b)
		}
	})
}

func TestCollectStreamEvents(t *testing.T) {
	ch := make(chan StreamEvent, 3)
	ch <- StreamEvent{Type: EventText, Text: "a"}
	ch <- StreamEvent{Type: EventText, Text: "b"}
	ch <- StreamEvent{Type: EventDone, FinishReason: "stop"}
	close(ch)

	events := CollectStreamEvents(ch)
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}
	if events[0].Text != "a" || events[1].Text != "b" {
		t.Errorf("texts = %q, %q", events[0].Text, events[1].Text)
	}
	if events[2].FinishReason != "stop" {
		t.Errorf("finish_reason = %q", events[2].FinishReason)
	}
}

func TestCollectStreamEventsEmpty(t *testing.T) {
	ch := make(chan StreamEvent)
	close(ch)

	events := CollectStreamEvents(ch)
	if len(events) != 0 {
		t.Fatalf("expected 0 events, got %d", len(events))
	}
}

func TestAPIKeyAuth(t *testing.T) {
	auth := &APIKeyAuth{Key: "sk-test-123"}
	req, _ := http.NewRequest("GET", "https://example.com", nil)

	if err := auth.Authenticate(req); err != nil {
		t.Fatal(err)
	}

	if got := req.Header.Get("x-api-key"); got != "sk-test-123" {
		t.Errorf("x-api-key = %q, want %q", got, "sk-test-123")
	}
}
