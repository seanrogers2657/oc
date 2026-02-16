package provider

import "encoding/json"

// SystemMessage creates a system message.
func SystemMessage(content string) Message {
	return Message{Role: RoleSystem, Content: content}
}

// UserMessage creates a user message.
func UserMessage(content string) Message {
	return Message{Role: RoleUser, Content: content}
}

// AssistantMessage creates an assistant message with text content.
func AssistantMessage(content string) Message {
	return Message{Role: RoleAssistant, Content: content}
}

// AssistantToolCallMessage creates an assistant message with tool calls.
func AssistantToolCallMessage(content string, toolCalls []ToolCall) Message {
	return Message{Role: RoleAssistant, Content: content, ToolCalls: toolCalls}
}

// ToolResultMessage creates a tool result message.
func ToolResultMessage(callID, name, content string) Message {
	return Message{Role: RoleTool, Content: content, ToolCallID: callID, Name: name}
}

// ToolDefToFunctionSchema converts a ToolDef to the JSON structure expected
// by OpenAI-compatible APIs for the "tools" field.
func ToolDefToFunctionSchema(td ToolDef) map[string]any {
	fn := map[string]any{
		"name":        td.Name,
		"description": td.Description,
	}
	if td.Parameters != nil {
		fn["parameters"] = td.Parameters
	} else {
		fn["parameters"] = map[string]any{"type": "object", "properties": map[string]any{}}
	}
	return map[string]any{
		"type":     "function",
		"function": fn,
	}
}

// ToolDefToAnthropicSchema converts a ToolDef to the JSON structure expected
// by the Anthropic API for the "tools" field.
func ToolDefToAnthropicSchema(td ToolDef) map[string]any {
	schema := map[string]any{
		"name":        td.Name,
		"description": td.Description,
	}
	if td.Parameters != nil {
		schema["input_schema"] = td.Parameters
	} else {
		schema["input_schema"] = map[string]any{"type": "object", "properties": map[string]any{}}
	}
	return schema
}

// MarshalJSON is a helper to marshal any value to a JSON byte slice,
// returning an empty object on error.
func MarshalJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return []byte("{}")
	}
	return b
}
