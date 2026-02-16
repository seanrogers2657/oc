package tool

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// GrepTool searches file contents with regex.
type GrepTool struct{}

func NewGrep() *GrepTool { return &GrepTool{} }

func (t *GrepTool) Name() string { return "grep" }

func (t *GrepTool) Description() string {
	return "Search file contents using a regular expression. Returns matching lines with file paths and line numbers."
}

func (t *GrepTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"pattern": map[string]any{
				"type":        "string",
				"description": "Regular expression pattern to search for",
			},
			"path": map[string]any{
				"type":        "string",
				"description": "File or directory to search in (default: working directory)",
			},
			"glob": map[string]any{
				"type":        "string",
				"description": "Glob pattern to filter files (e.g., \"*.go\", \"*.ts\")",
			},
			"context": map[string]any{
				"type":        "integer",
				"description": "Number of context lines before and after each match (default 0)",
			},
		},
		"required": []string{"pattern"},
	}
}

func (t *GrepTool) Execute(ctx Context, argsJSON string) Result {
	var args struct {
		Pattern string `json:"pattern"`
		Path    string `json:"path"`
		Glob    string `json:"glob"`
		Context int    `json:"context"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return Result{Error: fmt.Errorf("parse args: %w", err)}
	}

	if args.Pattern == "" {
		return Result{Error: fmt.Errorf("pattern is required")}
	}

	re, err := regexp.Compile(args.Pattern)
	if err != nil {
		return Result{Error: fmt.Errorf("invalid regex: %w", err)}
	}

	searchPath := ctx.WorkingDir
	if args.Path != "" {
		searchPath = resolvePath(ctx.WorkingDir, args.Path)
	}

	info, err := os.Stat(searchPath)
	if err != nil {
		return Result{Error: fmt.Errorf("stat: %w", err)}
	}

	var b strings.Builder
	matchCount := 0
	fileCount := 0
	const maxMatches = 500

	if !info.IsDir() {
		// Search single file
		n := searchFile(&b, searchPath, "", re, args.Context, maxMatches)
		matchCount += n
		if n > 0 {
			fileCount = 1
		}
	} else {
		// Walk directory
		filepath.WalkDir(searchPath, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				name := d.Name()
				if d.IsDir() && (name == ".git" || name == "node_modules" || name == ".svn" || name == "__pycache__") {
					return filepath.SkipDir
				}
				return nil
			}

			// Skip binary-looking files
			if isBinaryFilename(d.Name()) {
				return nil
			}

			// Apply glob filter
			if args.Glob != "" {
				matched, _ := filepath.Match(args.Glob, d.Name())
				if !matched {
					return nil
				}
			}

			relPath, _ := filepath.Rel(searchPath, path)
			if matchCount >= maxMatches {
				return filepath.SkipAll
			}

			n := searchFile(&b, path, relPath, re, args.Context, maxMatches-matchCount)
			matchCount += n
			if n > 0 {
				fileCount++
			}

			return nil
		})
	}

	if matchCount == 0 {
		return Result{
			Output: "No matches found",
			Title:  fmt.Sprintf("Grep %q (0 matches)", args.Pattern),
		}
	}

	if matchCount >= maxMatches {
		fmt.Fprintf(&b, "\n... (showing first %d matches)\n", maxMatches)
	}

	return Result{
		Output: b.String(),
		Title:  fmt.Sprintf("Grep %q (%d matches in %d files)", args.Pattern, matchCount, fileCount),
	}
}

func searchFile(b *strings.Builder, absPath, displayPath string, re *regexp.Regexp, contextLines, maxMatches int) int {
	f, err := os.Open(absPath)
	if err != nil {
		return 0
	}
	defer f.Close()

	if displayPath == "" {
		displayPath = absPath
	}

	scanner := bufio.NewScanner(f)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	matchCount := 0
	for i, line := range lines {
		if matchCount >= maxMatches {
			break
		}
		if !re.MatchString(line) {
			continue
		}

		matchCount++

		// Print context before
		start := i - contextLines
		if start < 0 {
			start = 0
		}
		for j := start; j < i; j++ {
			fmt.Fprintf(b, "%s:%d: %s\n", displayPath, j+1, lines[j])
		}

		// Print matching line
		fmt.Fprintf(b, "%s:%d: %s\n", displayPath, i+1, line)

		// Print context after
		end := i + contextLines
		if end >= len(lines) {
			end = len(lines) - 1
		}
		for j := i + 1; j <= end; j++ {
			fmt.Fprintf(b, "%s:%d: %s\n", displayPath, j+1, lines[j])
		}

		if contextLines > 0 && matchCount < maxMatches {
			fmt.Fprintln(b, "--")
		}
	}

	return matchCount
}

func isBinaryFilename(name string) bool {
	binaryExts := map[string]bool{
		".exe": true, ".bin": true, ".so": true, ".dylib": true, ".dll": true,
		".o": true, ".a": true, ".obj": true,
		".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".bmp": true, ".ico": true, ".webp": true,
		".zip": true, ".tar": true, ".gz": true, ".bz2": true, ".xz": true, ".7z": true, ".rar": true,
		".pdf": true, ".doc": true, ".docx": true,
		".wasm": true, ".pyc": true, ".class": true,
	}
	return binaryExts[strings.ToLower(filepath.Ext(name))]
}
