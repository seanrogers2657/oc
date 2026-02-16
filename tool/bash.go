package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// BashTool executes shell commands.
type BashTool struct{}

func NewBash() *BashTool { return &BashTool{} }

func (t *BashTool) Name() string { return "bash" }

func (t *BashTool) Description() string {
	return "Execute a shell command and return stdout/stderr. Use for running programs, installing packages, or system operations."
}

func (t *BashTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command": map[string]any{
				"type":        "string",
				"description": "The shell command to execute",
			},
			"timeout": map[string]any{
				"type":        "integer",
				"description": "Timeout in seconds (default 120)",
			},
		},
		"required": []string{"command"},
	}
}

func (t *BashTool) Execute(ctx Context, argsJSON string) Result {
	var args struct {
		Command string `json:"command"`
		Timeout int    `json:"timeout"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return Result{Error: fmt.Errorf("parse args: %w", err)}
	}

	if args.Command == "" {
		return Result{Error: fmt.Errorf("command is required")}
	}

	timeout := 120 * time.Second
	if args.Timeout > 0 {
		timeout = time.Duration(args.Timeout) * time.Second
	}

	cmdCtx := ctx.Ctx
	if cmdCtx == nil {
		cmdCtx = context.Background()
	}
	cmdCtx, cancel := context.WithTimeout(cmdCtx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "sh", "-c", args.Command)
	cmd.Dir = ctx.WorkingDir

	output, err := cmd.CombinedOutput()
	out := string(output)

	// Truncate very long output
	const maxOutput = 100_000
	if len(out) > maxOutput {
		out = out[:maxOutput] + "\n... (truncated)"
	}

	if err != nil {
		exitErr := ""
		if cmd.ProcessState != nil {
			exitErr = fmt.Sprintf(" (exit code %d)", cmd.ProcessState.ExitCode())
		}
		return Result{
			Output: out,
			Title:  fmt.Sprintf("bash: %s%s", truncateCmd(args.Command), exitErr),
			Error:  fmt.Errorf("%w%s", err, exitErr),
		}
	}

	return Result{
		Output: out,
		Title:  fmt.Sprintf("bash: %s", truncateCmd(args.Command)),
	}
}

func truncateCmd(cmd string) string {
	cmd = strings.TrimSpace(cmd)
	if len(cmd) > 60 {
		return cmd[:57] + "..."
	}
	return cmd
}
