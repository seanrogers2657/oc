package tool

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EditTool performs string replacement edits on files.
type EditTool struct{}

func NewEdit() *EditTool { return &EditTool{} }

func (t *EditTool) Name() string { return "edit" }

func (t *EditTool) Description() string {
	return "Edit a file by replacing an exact string match. The old_string must appear exactly once in the file (unless replace_all is true)."
}

func (t *EditTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"file_path": map[string]any{
				"type":        "string",
				"description": "Path to the file to edit",
			},
			"old_string": map[string]any{
				"type":        "string",
				"description": "The exact string to find and replace",
			},
			"new_string": map[string]any{
				"type":        "string",
				"description": "The string to replace it with",
			},
			"replace_all": map[string]any{
				"type":        "boolean",
				"description": "Replace all occurrences (default false)",
			},
		},
		"required": []string{"file_path", "old_string", "new_string"},
	}
}

func (t *EditTool) Execute(ctx Context, argsJSON string) Result {
	var args struct {
		FilePath   string `json:"file_path"`
		OldString  string `json:"old_string"`
		NewString  string `json:"new_string"`
		ReplaceAll bool   `json:"replace_all"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return Result{Error: fmt.Errorf("parse args: %w", err)}
	}

	if args.FilePath == "" {
		return Result{Error: fmt.Errorf("file_path is required")}
	}
	if args.OldString == "" {
		return Result{Error: fmt.Errorf("old_string is required")}
	}
	if args.OldString == args.NewString {
		return Result{Error: fmt.Errorf("old_string and new_string are identical")}
	}

	path := resolvePath(ctx.WorkingDir, args.FilePath)

	data, err := os.ReadFile(path)
	if err != nil {
		return Result{Error: fmt.Errorf("read file: %w", err)}
	}

	content := string(data)
	count := strings.Count(content, args.OldString)

	if count == 0 {
		return Result{Error: fmt.Errorf("old_string not found in %s", filepath.Base(path))}
	}

	if count > 1 && !args.ReplaceAll {
		return Result{Error: fmt.Errorf("old_string found %d times in %s (use replace_all to replace all occurrences)", count, filepath.Base(path))}
	}

	var newContent string
	if args.ReplaceAll {
		newContent = strings.ReplaceAll(content, args.OldString, args.NewString)
	} else {
		newContent = strings.Replace(content, args.OldString, args.NewString, 1)
	}

	if err := os.WriteFile(path, []byte(newContent), 0o644); err != nil {
		return Result{Error: fmt.Errorf("write file: %w", err)}
	}

	title := fmt.Sprintf("Edit %s (%d replacement", filepath.Base(path), count)
	if count != 1 {
		title += "s"
	}
	title += ")"

	// Show a snippet around the edited region, like READ does
	output := editSnippet(newContent, args.NewString)

	return Result{
		Output: output,
		Title:  title,
	}
}

// editSnippet returns line-numbered snippets of content around every
// occurrence of target, with 3 lines of context on each side.
// Overlapping or adjacent windows are merged; disjoint ones are
// separated by a "..." line.
func editSnippet(content, target string) string {
	const contextLines = 3

	lines := strings.Split(content, "\n")
	targetLineSpan := strings.Count(target, "\n")

	// Find every occurrence and build [from, to) windows.
	type window struct{ from, to int }
	var windows []window

	searchFrom := 0
	for {
		pos := strings.Index(content[searchFrom:], target)
		if pos < 0 {
			break
		}
		pos += searchFrom

		startLine := strings.Count(content[:pos], "\n")
		endLine := startLine + targetLineSpan

		from := startLine - contextLines
		if from < 0 {
			from = 0
		}
		to := endLine + contextLines + 1
		if to > len(lines) {
			to = len(lines)
		}

		windows = append(windows, window{from, to})
		searchFrom = pos + len(target)
	}

	if len(windows) == 0 {
		return content
	}

	// Merge overlapping/adjacent windows.
	merged := []window{windows[0]}
	for _, w := range windows[1:] {
		last := &merged[len(merged)-1]
		if w.from <= last.to {
			if w.to > last.to {
				last.to = w.to
			}
		} else {
			merged = append(merged, w)
		}
	}

	// Render each window, separated by "...".
	var b strings.Builder
	for i, w := range merged {
		if i > 0 {
			b.WriteString("   ...\n")
		}
		for j := w.from; j < w.to; j++ {
			line := lines[j]
			if len(line) > 2000 {
				line = line[:2000] + "..."
			}
			fmt.Fprintf(&b, "%6d  %s\n", j+1, line)
		}
	}
	return b.String()
}
