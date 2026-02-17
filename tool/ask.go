package tool

import (
	"encoding/json"
	"fmt"
)

// AskTool lets the model ask the user a clarifying question mid-execution.
type AskTool struct{}

func NewAsk() *AskTool { return &AskTool{} }

func (t *AskTool) Name() string { return "user_prompt" }

func (t *AskTool) Description() string {
	return "Ask the user a clarifying question and wait for their response. Use this when you need more information to proceed."
}

func (t *AskTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"question": map[string]any{
				"type":        "string",
				"description": "The question to ask the user",
			},
		},
		"required": []string{"question"},
	}
}

func (t *AskTool) Execute(ctx Context, argsJSON string) Result {
	var args struct {
		Question string `json:"question"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return Result{Error: fmt.Errorf("parse args: %w", err)}
	}

	if args.Question == "" {
		return Result{Error: fmt.Errorf("question is required")}
	}

	if ctx.Prompter == nil {
		return Result{Error: fmt.Errorf("prompting is not available")}
	}

	answer, err := ctx.Prompter.Prompt(ctx.Ctx, args.Question)
	if err != nil {
		return Result{Error: fmt.Errorf("prompt: %w", err)}
	}

	return Result{
		Output: answer,
		Title:  "Asked user",
	}
}
