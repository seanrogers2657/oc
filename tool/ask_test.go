package tool

import (
	"context"
	"strings"
	"testing"
)

// mockPrompter implements Prompter for testing.
type mockPrompter struct {
	answer string
	err    error
}

func (m *mockPrompter) Prompt(_ context.Context, _ string) (string, error) {
	return m.answer, m.err
}

func TestAskReturnsUserAnswer(t *testing.T) {
	ask := NewAsk()
	ctx := Context{
		Ctx:      context.Background(),
		Prompter: &mockPrompter{answer: "Go"},
	}

	result := ask.Execute(ctx, `{"question":"What language?"}`)
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if result.Output != "Go" {
		t.Errorf("expected output %q, got %q", "Go", result.Output)
	}
	if result.Title != "Asked user" {
		t.Errorf("expected title %q, got %q", "Asked user", result.Title)
	}
}

func TestAskNoPrompter(t *testing.T) {
	ask := NewAsk()
	ctx := Context{
		Ctx:      context.Background(),
		Prompter: nil,
	}

	result := ask.Execute(ctx, `{"question":"What language?"}`)
	if result.Error == nil {
		t.Fatal("expected error when Prompter is nil")
	}
	if !strings.Contains(result.Error.Error(), "not available") {
		t.Errorf("expected 'not available' error, got: %v", result.Error)
	}
}

func TestAskEmptyQuestion(t *testing.T) {
	ask := NewAsk()
	ctx := Context{
		Ctx:      context.Background(),
		Prompter: &mockPrompter{answer: "irrelevant"},
	}

	result := ask.Execute(ctx, `{"question":""}`)
	if result.Error == nil {
		t.Fatal("expected error for empty question")
	}
}

func TestAskBadJSON(t *testing.T) {
	ask := NewAsk()
	ctx := Context{Ctx: context.Background()}

	result := ask.Execute(ctx, `not json`)
	if result.Error == nil {
		t.Fatal("expected error for bad JSON")
	}
}

func TestAskCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	ask := NewAsk()
	toolCtx := Context{
		Ctx: ctx,
		Prompter: &mockPrompter{
			err: ctx.Err(),
		},
	}

	result := ask.Execute(toolCtx, `{"question":"What?"}`)
	if result.Error == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestAskToolMetadata(t *testing.T) {
	ask := NewAsk()
	if ask.Name() != "user_prompt" {
		t.Errorf("expected name %q, got %q", "user_prompt", ask.Name())
	}
	if ask.Description() == "" {
		t.Error("expected non-empty description")
	}
	params := ask.Parameters()
	if params == nil {
		t.Fatal("expected non-nil parameters")
	}
}
