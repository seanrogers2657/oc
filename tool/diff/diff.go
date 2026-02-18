// Package diff provides unified diff generation for displaying file changes
// in a git-style format with proper context lines and change detection.
package diff

import (
	"strings"
)

// LineType represents the type of a diff line
type LineType int

const (
	LineContext LineType = iota // Unchanged context line
	LineAdded                   // Added line (green)
	LineRemoved                 // Removed line (red)
)

// DiffLine represents a single line in a unified diff
type DiffLine struct {
	Type       LineType // Type of change
	OldNumber  int      // Line number in original file (0 for added lines)
	NewNumber  int      // Line number in new file (0 for removed lines)
	Content    string   // Line content without newline
}

// DiffResult contains the complete diff information
type DiffResult struct {
	Lines    []DiffLine // All diff lines
	Added    int        // Count of added lines
	Removed  int        // Count of removed lines
	OldName  string     // Original file name/path
	NewName  string     // New file name/path
}

// GenerateDiff creates a unified diff between old and new content
func GenerateDiff(oldContent, newContent string, contextLines int) DiffResult {
	if contextLines < 0 {
		contextLines = 3 // default
	}

	oldLines := splitLines(oldContent)
	newLines := splitLines(newContent)

	// Use a simpler but more reliable diff algorithm
	diffLines := generateUnifiedDiff(oldLines, newLines, contextLines)
	
	// Count additions and deletions
	added, removed := countChanges(diffLines)
	
	return DiffResult{
		Lines:   diffLines,
		Added:   added,
		Removed: removed,
	}
}

// generateUnifiedDiff creates a unified diff using a simple algorithm
func generateUnifiedDiff(oldLines, newLines []string, contextLines int) []DiffLine {
	var result []DiffLine
	
	// Use simple longest common subsequence approach
	lcs := computeLCS(oldLines, newLines)
	
	oldIndex := 0
	newIndex := 0
	oldLineNum := 1
	newLineNum := 1
	
	for _, match := range lcs {
		// Add removed lines (lines in old that don't match)
		for oldIndex < match.OldIndex {
			result = append(result, DiffLine{
				Type:      LineRemoved,
				OldNumber: oldLineNum,
				NewNumber: 0,
				Content:   oldLines[oldIndex],
			})
			oldIndex++
			oldLineNum++
		}
		
		// Add added lines (lines in new that don't match)
		for newIndex < match.NewIndex {
			result = append(result, DiffLine{
				Type:      LineAdded,
				OldNumber: 0,
				NewNumber: newLineNum,
				Content:   newLines[newIndex],
			})
			newIndex++
			newLineNum++
		}
		
		// Add the matching line
		result = append(result, DiffLine{
			Type:      LineContext,
			OldNumber: oldLineNum,
			NewNumber: newLineNum,
			Content:   oldLines[oldIndex],
		})
		oldIndex++
		newIndex++
		oldLineNum++
		newLineNum++
	}
	
	// Handle remaining lines
	for oldIndex < len(oldLines) {
		result = append(result, DiffLine{
			Type:      LineRemoved,
			OldNumber: oldLineNum,
			NewNumber: 0,
			Content:   oldLines[oldIndex],
		})
		oldIndex++
		oldLineNum++
	}
	
	for newIndex < len(newLines) {
		result = append(result, DiffLine{
			Type:      LineAdded,
			OldNumber: 0,
			NewNumber: newLineNum,
			Content:   newLines[newIndex],
		})
		newIndex++
		newLineNum++
	}
	
	// Apply context filtering if needed
	return filterContext(result, contextLines)
}

// lcsPair represents a matching line pair
type lcsPair struct {
	OldIndex int
	NewIndex int
}

// computeLCS computes the longest common subsequence of lines
func computeLCS(oldLines, newLines []string) []lcsPair {
	m, n := len(oldLines), len(newLines)
	if m == 0 || n == 0 {
		return nil
	}
	
	// Create LCS table
	lcs := make([][]int, m+1)
	for i := range lcs {
		lcs[i] = make([]int, n+1)
	}
	
	// Fill LCS table
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if oldLines[i-1] == newLines[j-1] {
				lcs[i][j] = lcs[i-1][j-1] + 1
			} else {
				lcs[i][j] = max(lcs[i-1][j], lcs[i][j-1])
			}
		}
	}
	
	// Backtrack to find actual matches
	var matches []lcsPair
	i, j := m, n
	for i > 0 && j > 0 {
		if oldLines[i-1] == newLines[j-1] {
			matches = append([]lcsPair{{i-1, j-1}}, matches...)
			i--
			j--
		} else if lcs[i-1][j] > lcs[i][j-1] {
			i--
		} else {
			j--
		}
	}
	
	return matches
}

// filterContext reduces diff to only show changes with specified context
func filterContext(allLines []DiffLine, contextLines int) []DiffLine {
	if contextLines < 0 || len(allLines) == 0 {
		return allLines
	}
	
	// Find all change lines
	var changeIndices []int
	for i, line := range allLines {
		if line.Type != LineContext {
			changeIndices = append(changeIndices, i)
		}
	}
	
	if len(changeIndices) == 0 {
		// No changes, return empty diff
		return nil
	}
	
	// Calculate ranges to include (changes + context)
	var ranges []struct{ start, end int }
	for _, changeIdx := range changeIndices {
		start := max(0, changeIdx-contextLines)
		end := min(len(allLines)-1, changeIdx+contextLines)
		
		// Merge overlapping ranges
		if len(ranges) > 0 && start <= ranges[len(ranges)-1].end+1 {
			if end > ranges[len(ranges)-1].end {
				ranges[len(ranges)-1].end = end
			}
		} else {
			ranges = append(ranges, struct{ start, end int }{start, end})
		}
	}
	
	// Extract lines from ranges
	var result []DiffLine
	for _, r := range ranges {
		for i := r.start; i <= r.end; i++ {
			result = append(result, allLines[i])
		}
	}
	
	return result
}

// countChanges counts additions and deletions in diff lines
func countChanges(lines []DiffLine) (added, removed int) {
	for _, line := range lines {
		switch line.Type {
		case LineAdded:
			added++
		case LineRemoved:
			removed++
		}
	}
	return
}

// splitLines splits content into lines, handling different line endings
func splitLines(content string) []string {
	if content == "" {
		return nil
	}
	
	// Normalize line endings
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")
	
	lines := strings.Split(content, "\n")
	
	// Remove the last empty line if the content ended with a newline
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	
	return lines
}

// Helper functions
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}