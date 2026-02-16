package tool

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// WriteTool writes or creates files.
type WriteTool struct{}

func NewWrite() *WriteTool { return &WriteTool{} }

func (t *WriteTool) Name() string { return "write" }

func (t *WriteTool) Description() string {
	return "Write content to a file. Creates the file and parent directories if they don't exist. Overwrites existing content."
}

func (t *WriteTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"file_path": map[string]any{
				"type":        "string",
				"description": "Path to the file to write",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "Content to write to the file",
			},
		},
		"required": []string{"file_path", "content"},
	}
}

func (t *WriteTool) Execute(ctx Context, argsJSON string) Result {
	var args struct {
		FilePath string `json:"file_path"`
		Content  string `json:"content"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return Result{Error: fmt.Errorf("parse args: %w", err)}
	}

	if args.FilePath == "" {
		return Result{Error: fmt.Errorf("file_path is required")}
	}

	path := resolvePath(ctx.WorkingDir, args.FilePath)

	// Create parent directories
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Result{Error: fmt.Errorf("create directories: %w", err)}
	}

	if err := os.WriteFile(path, []byte(args.Content), 0o644); err != nil {
		return Result{Error: fmt.Errorf("write file: %w", err)}
	}

	return Result{
		Output: fmt.Sprintf("Wrote %d bytes to %s", len(args.Content), path),
		Title:  fmt.Sprintf("Write %s (%d bytes)", filepath.Base(path), len(args.Content)),
	}
}
