// Package diff provides unified diff generation for displaying file changes
// in a git-style format with proper context lines and change detection.
//
// GenerateDiff computes the line-level differences between two strings.
// FormatDiffForTUI converts a DiffResult into styled lines suitable for
// rendering in the terminal UI.
//
// This package has no dependencies beyond stdlib.
package diff
