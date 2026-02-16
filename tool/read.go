package tool

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ReadTool reads file contents with line numbers.
type ReadTool struct{}

func NewRead() *ReadTool { return &ReadTool{} }

func (t *ReadTool) Name() string { return "read" }

func (t *ReadTool) Description() string {
	return "Read the contents of a file. Returns the file with line numbers. Optionally specify offset and limit for large files."
}

func (t *ReadTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"file_path": map[string]any{
				"type":        "string",
				"description": "Path to the file to read",
			},
			"offset": map[string]any{
				"type":        "integer",
				"description": "Line number to start reading from (1-based, default 1)",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Maximum number of lines to read (default 2000)",
			},
		},
		"required": []string{"file_path"},
	}
}

func (t *ReadTool) Execute(ctx Context, argsJSON string) Result {
	var args struct {
		FilePath string `json:"file_path"`
		Offset   int    `json:"offset"`
		Limit    int    `json:"limit"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return Result{Error: fmt.Errorf("parse args: %w", err)}
	}

	if args.FilePath == "" {
		return Result{Error: fmt.Errorf("file_path is required")}
	}

	path := resolvePath(ctx.WorkingDir, args.FilePath)

	data, err := os.ReadFile(path)
	if err != nil {
		return Result{Error: fmt.Errorf("read file: %w", err)}
	}

	lines := strings.Split(string(data), "\n")

	// Defaults
	offset := args.Offset
	if offset < 1 {
		offset = 1
	}
	limit := args.Limit
	if limit <= 0 {
		limit = 2000
	}

	// Apply offset (1-based)
	startIdx := offset - 1
	if startIdx > len(lines) {
		startIdx = len(lines)
	}
	endIdx := startIdx + limit
	if endIdx > len(lines) {
		endIdx = len(lines)
	}

	// Format with line numbers
	var b strings.Builder
	for i := startIdx; i < endIdx; i++ {
		line := lines[i]
		// Truncate very long lines
		if len(line) > 2000 {
			line = line[:2000] + "..."
		}
		fmt.Fprintf(&b, "%6d\t%s\n", i+1, line)
	}

	totalLines := len(lines)
	shown := endIdx - startIdx
	title := fmt.Sprintf("Read %s (%d/%d lines)", filepath.Base(path), shown, totalLines)

	return Result{
		Output: b.String(),
		Title:  title,
	}
}
