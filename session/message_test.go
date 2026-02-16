package session

import (
	"testing"
)

func TestPartTypes(t *testing.T) {
	tests := []struct {
		part     Part
		expected string
	}{
		{TextPart{Text: "hello"}, "text"},
		{ReasoningPart{Text: "thinking"}, "reasoning"},
		{ToolCallPart{Tool: "bash"}, "tool_call"},
	}

	for _, tt := range tests {
		if tt.part.partType() != tt.expected {
			t.Errorf("expected partType %q, got %q", tt.expected, tt.part.partType())
		}
	}
}

func TestToolStatusValues(t *testing.T) {
	// Verify enum ordering
	if ToolPending != 0 {
		t.Error("ToolPending should be 0")
	}
	if ToolRunning != 1 {
		t.Error("ToolRunning should be 1")
	}
	if ToolCompleted != 2 {
		t.Error("ToolCompleted should be 2")
	}
	if ToolError != 3 {
		t.Error("ToolError should be 3")
	}
}

func TestExtractText(t *testing.T) {
	parts := []Part{
		TextPart{Text: "hello "},
		ReasoningPart{Text: "thinking"},
		TextPart{Text: "world"},
	}
	text := extractText(parts)
	if text != "hello world" {
		t.Errorf("expected 'hello world', got %q", text)
	}
}

func TestExtractTextEmpty(t *testing.T) {
	text := extractText(nil)
	if text != "" {
		t.Errorf("expected empty string, got %q", text)
	}
}

func TestExtractToolCalls(t *testing.T) {
	parts := []Part{
		TextPart{Text: "let me help"},
		ToolCallPart{CallID: "call_1", Tool: "bash", Args: `{"command":"ls"}`},
		ToolCallPart{CallID: "call_2", Tool: "read", Args: `{"file_path":"x"}`},
	}
	calls := extractToolCalls(parts)
	if len(calls) != 2 {
		t.Fatalf("expected 2 tool calls, got %d", len(calls))
	}
	if calls[0].Name != "bash" {
		t.Errorf("expected bash, got %s", calls[0].Name)
	}
	if calls[1].ID != "call_2" {
		t.Errorf("expected call_2, got %s", calls[1].ID)
	}
}

func TestExtractToolCallsNone(t *testing.T) {
	parts := []Part{TextPart{Text: "just text"}}
	calls := extractToolCalls(parts)
	if len(calls) != 0 {
		t.Fatalf("expected 0 tool calls, got %d", len(calls))
	}
}
