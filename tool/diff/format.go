package diff

import (
	"fmt"
	"strconv"
	"strings"
)

// StyledLine represents a rendered line with styling information.
// Prefix (e.g. "+", "-", " ") is separate so the TUI can style it differently.
type StyledLine struct {
	Prefix     string // "+", "-", " ", or "" for header lines
	Text       string // the content (without prefix)
	Background string // color name or hex (applies to Text only)
	Foreground string // color name or hex
	Bold       bool
}

// FormatDiffForTUI converts a diff result into styled lines for display in the TUI
func FormatDiffForTUI(diffResult *DiffResult, maxWidth int) []StyledLine {
	if diffResult == nil || len(diffResult.Lines) == 0 {
		return nil
	}

	var lines []StyledLine

	// Header with file names and stats
	if diffResult.OldName != "" || diffResult.NewName != "" {
		oldName := diffResult.OldName
		if oldName == "" {
			oldName = "/dev/null"
		}
		newName := diffResult.NewName
		if newName == "" {
			newName = "/dev/null"
		}
		
		header := fmt.Sprintf("--- %s", oldName)
		lines = append(lines, StyledLine{
			Text:       truncateToWidth(header, maxWidth),
			Foreground: "bright_black",
		})
		
		header = fmt.Sprintf("+++ %s", newName)
		lines = append(lines, StyledLine{
			Text:       truncateToWidth(header, maxWidth),
			Foreground: "bright_black",
		})
	}

	// Process diff lines
	for _, diffLine := range diffResult.Lines {
		line := formatDiffLine(diffLine, maxWidth)
		lines = append(lines, line)
	}

	return lines
}

// formatDiffLine formats a single diff line with appropriate styling.
// Tabs in content are expanded to spaces so the screen buffer renders correctly.
func formatDiffLine(diffLine DiffLine, maxWidth int) StyledLine {
	var style StyledLine

	switch diffLine.Type {
	case LineContext:
		style = StyledLine{
			Prefix:     " ",
			Foreground: "white",
		}
	case LineAdded:
		style = StyledLine{
			Prefix:     "+",
			Background: "dark_green",
			Foreground: "white",
		}
	case LineRemoved:
		style = StyledLine{
			Prefix:     "-",
			Background: "dark_red",
			Foreground: "white",
		}
	}

	content := expandTabs(diffLine.Content, tabWidth)

	// Reserve 1 column for the prefix when truncating
	if maxWidth > 1 {
		style.Text = truncateToWidth(content, maxWidth-1)
	} else {
		style.Text = content
	}
	return style
}

const tabWidth = 4

// expandTabs replaces tab characters with spaces, aligning to tab stops.
func expandTabs(s string, tabSize int) string {
	if !strings.Contains(s, "\t") {
		return s
	}
	var b strings.Builder
	col := 0
	for _, r := range s {
		switch r {
		case '\t':
			spaces := tabSize - (col % tabSize)
			for range spaces {
				b.WriteByte(' ')
			}
			col += spaces
		case '\n':
			b.WriteByte('\n')
			col = 0
		default:
			b.WriteRune(r)
			col++
		}
	}
	return b.String()
}

// FormatDiffHeader creates a summary header for a diff
func FormatDiffHeader(diffResult *DiffResult) string {
	if diffResult == nil {
		return ""
	}
	
	if diffResult.Added == 0 && diffResult.Removed == 0 {
		return "No changes"
	}
	
	return fmt.Sprintf("%s (+%d -%d)", 
		getFileName(diffResult.NewName, diffResult.OldName), 
		diffResult.Added, 
		diffResult.Removed)
}

// getFileName extracts the filename from a path, preferring the new name
func getFileName(newName, oldName string) string {
	if newName != "" && newName != "/dev/null" {
		parts := strings.Split(newName, "/")
		return parts[len(parts)-1]
	}
	if oldName != "" && oldName != "/dev/null" {
		parts := strings.Split(oldName, "/")
		return parts[len(parts)-1]
	}
	return "file"
}

// truncateToWidth truncates text to fit within the specified width
func truncateToWidth(text string, maxWidth int) string {
	if maxWidth <= 0 {
		return text
	}
	
	// Count runes for proper Unicode handling
	runes := []rune(text)
	if len(runes) <= maxWidth {
		return text
	}
	
	if maxWidth <= 3 {
		return string(runes[:maxWidth])
	}
	
	return string(runes[:maxWidth-3]) + "..."
}

// ParseNumber safely parses a string to int, returning 0 on error
func ParseNumber(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}