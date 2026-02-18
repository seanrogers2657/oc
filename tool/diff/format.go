package diff

import (
	"fmt"
	"strconv"
	"strings"
)

// StyledLine represents a rendered line with styling information
type StyledLine struct {
	Text       string
	Background string // color name or hex
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

// formatDiffLine formats a single diff line with appropriate styling
func formatDiffLine(diffLine DiffLine, maxWidth int) StyledLine {
	var prefix string
	var style StyledLine

	switch diffLine.Type {
	case LineContext:
		prefix = " "
		style = StyledLine{
			Foreground: "white",
		}
	case LineAdded:
		prefix = "+"
		style = StyledLine{
			Background: "dark_green",
			Foreground: "white",
		}
	case LineRemoved:
		prefix = "-"
		style = StyledLine{
			Background: "dark_red",
			Foreground: "white",
		}
	}

	// Format line with numbers and content
	var lineStr string
	if diffLine.Type == LineRemoved {
		lineStr = fmt.Sprintf("%s%s", prefix, diffLine.Content)
	} else if diffLine.Type == LineAdded {
		lineStr = fmt.Sprintf("%s%s", prefix, diffLine.Content)
	} else {
		// Context line
		lineStr = fmt.Sprintf("%s%s", prefix, diffLine.Content)
	}

	style.Text = truncateToWidth(lineStr, maxWidth)
	return style
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