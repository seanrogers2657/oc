package tool

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// GlobTool finds files matching a pattern.
type GlobTool struct{}

func NewGlob() *GlobTool { return &GlobTool{} }

func (t *GlobTool) Name() string { return "glob" }

func (t *GlobTool) Description() string {
	return "Find files matching a glob pattern. Returns matching file paths sorted by modification time (newest first)."
}

func (t *GlobTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"pattern": map[string]any{
				"type":        "string",
				"description": "Glob pattern (e.g., \"**/*.go\", \"src/**/*.ts\")",
			},
			"path": map[string]any{
				"type":        "string",
				"description": "Directory to search in (default: working directory)",
			},
		},
		"required": []string{"pattern"},
	}
}

func (t *GlobTool) Execute(ctx Context, argsJSON string) Result {
	var args struct {
		Pattern string `json:"pattern"`
		Path    string `json:"path"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return Result{Error: fmt.Errorf("parse args: %w", err)}
	}

	if args.Pattern == "" {
		return Result{Error: fmt.Errorf("pattern is required")}
	}

	searchDir := ctx.WorkingDir
	if args.Path != "" {
		searchDir = resolvePath(ctx.WorkingDir, args.Path)
	}

	type fileEntry struct {
		path    string
		modTime int64
	}

	var matches []fileEntry

	err := filepath.WalkDir(searchDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip inaccessible
		}

		// Skip common hidden/vendor directories
		name := d.Name()
		if d.IsDir() && (name == ".git" || name == "node_modules" || name == ".svn" || name == "__pycache__") {
			return filepath.SkipDir
		}

		if d.IsDir() {
			return nil
		}

		// Get path relative to search dir
		relPath, err := filepath.Rel(searchDir, path)
		if err != nil {
			return nil
		}

		// Match against pattern
		if matchGlob(args.Pattern, relPath) {
			info, err := d.Info()
			modTime := int64(0)
			if err == nil {
				modTime = info.ModTime().Unix()
			}
			matches = append(matches, fileEntry{path: relPath, modTime: modTime})
		}

		return nil
	})

	if err != nil {
		return Result{Error: fmt.Errorf("walk: %w", err)}
	}

	// Sort by modification time (newest first)
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].modTime > matches[j].modTime
	})

	// Truncate if too many results
	const maxResults = 500
	truncated := false
	if len(matches) > maxResults {
		matches = matches[:maxResults]
		truncated = true
	}

	var b strings.Builder
	for _, m := range matches {
		fmt.Fprintln(&b, m.path)
	}
	if truncated {
		fmt.Fprintf(&b, "... (showing %d of more results)\n", maxResults)
	}

	if len(matches) == 0 {
		return Result{
			Output: "No files found",
			Title:  fmt.Sprintf("Glob %s (0 matches)", args.Pattern),
		}
	}

	return Result{
		Output: b.String(),
		Title:  fmt.Sprintf("Glob %s (%d matches)", args.Pattern, len(matches)),
	}
}

// matchGlob matches a path against a glob pattern.
// Supports ** for recursive directory matching.
func matchGlob(pattern, path string) bool {
	// Handle ** patterns by splitting into segments
	if strings.Contains(pattern, "**") {
		return matchDoublestar(pattern, path)
	}

	// Simple case: use filepath.Match
	matched, _ := filepath.Match(pattern, path)
	if matched {
		return true
	}

	// Also try matching just the filename
	matched, _ = filepath.Match(pattern, filepath.Base(path))
	return matched
}

// matchDoublestar handles ** glob patterns.
func matchDoublestar(pattern, path string) bool {
	// Split pattern by **
	parts := strings.Split(pattern, "**")

	if len(parts) == 2 {
		prefix := strings.TrimSuffix(parts[0], "/")
		suffix := strings.TrimPrefix(parts[1], "/")

		// Check if path starts with prefix
		if prefix != "" && !strings.HasPrefix(path, prefix) {
			return false
		}

		// Check if suffix matches the end of the path
		if suffix == "" {
			return true
		}

		// Try matching suffix against every possible sub-path
		remaining := path
		if prefix != "" {
			remaining = strings.TrimPrefix(path, prefix)
			remaining = strings.TrimPrefix(remaining, "/")
		}

		// Check if any tail of remaining matches the suffix
		parts := strings.Split(remaining, "/")
		for i := range parts {
			tail := strings.Join(parts[i:], "/")
			matched, _ := filepath.Match(suffix, tail)
			if matched {
				return true
			}
			// Also try matching just the last component
			if i == len(parts)-1 {
				matched, _ = filepath.Match(suffix, parts[i])
				if matched {
					return true
				}
			}
		}
	}

	return false
}
