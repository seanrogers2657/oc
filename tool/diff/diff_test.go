package diff

import (
	"strings"
	"testing"
)

func TestGenerateDiff_EmptyFiles(t *testing.T) {
	tests := []struct {
		name        string
		oldContent  string
		newContent  string
		wantAdded   int
		wantRemoved int
	}{
		{
			name:        "both empty",
			oldContent:  "",
			newContent:  "",
			wantAdded:   0,
			wantRemoved: 0,
		},
		{
			name:        "empty to content",
			oldContent:  "",
			newContent:  "line 1\nline 2",
			wantAdded:   2,
			wantRemoved: 0,
		},
		{
			name:        "content to empty",
			oldContent:  "line 1\nline 2",
			newContent:  "",
			wantAdded:   0,
			wantRemoved: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateDiff(tt.oldContent, tt.newContent, 3)
			if result.Added != tt.wantAdded {
				t.Errorf("Added = %v, want %v", result.Added, tt.wantAdded)
			}
			if result.Removed != tt.wantRemoved {
				t.Errorf("Removed = %v, want %v", result.Removed, tt.wantRemoved)
			}
		})
	}
}

func TestGenerateDiff_SimpleChanges(t *testing.T) {
	tests := []struct {
		name        string
		oldContent  string
		newContent  string
		wantAdded   int
		wantRemoved int
		wantTypes   []LineType
	}{
		{
			name:        "single line addition",
			oldContent:  "line 1\nline 2",
			newContent:  "line 1\nline 2\nline 3",
			wantAdded:   1,
			wantRemoved: 0,
			wantTypes:   []LineType{LineContext, LineAdded}, // Only "line 2" context + "line 3" added
		},
		{
			name:        "single line removal",
			oldContent:  "line 1\nline 2\nline 3",
			newContent:  "line 1\nline 3",
			wantAdded:   0,
			wantRemoved: 1,
			wantTypes:   []LineType{LineContext, LineRemoved, LineContext},
		},
		{
			name:        "single line replacement",
			oldContent:  "line 1\nold line\nline 3",
			newContent:  "line 1\nnew line\nline 3",
			wantAdded:   1,
			wantRemoved: 1,
			wantTypes:   []LineType{LineContext, LineRemoved, LineAdded, LineContext},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateDiff(tt.oldContent, tt.newContent, 1)
			
			if result.Added != tt.wantAdded {
				t.Errorf("Added = %v, want %v", result.Added, tt.wantAdded)
			}
			if result.Removed != tt.wantRemoved {
				t.Errorf("Removed = %v, want %v", result.Removed, tt.wantRemoved)
			}
			
			if len(result.Lines) != len(tt.wantTypes) {
				t.Fatalf("Lines count = %v, want %v", len(result.Lines), len(tt.wantTypes))
			}
			
			for i, line := range result.Lines {
				if line.Type != tt.wantTypes[i] {
					t.Errorf("Line %d type = %v, want %v", i, line.Type, tt.wantTypes[i])
				}
			}
		})
	}
}

func TestGenerateDiff_ContextLines(t *testing.T) {
	oldContent := "line 1\nline 2\nline 3\nline 4\nline 5\nline 6\nline 7"
	newContent := "line 1\nline 2\nline 3\nNEW LINE\nline 5\nline 6\nline 7"

	tests := []struct {
		name         string
		contextLines int
		wantMinLines int
		wantMaxLines int
	}{
		{
			name:         "no context",
			contextLines: 0,
			wantMinLines: 2, // removed + added
			wantMaxLines: 2,
		},
		{
			name:         "1 context line",
			contextLines: 1,
			wantMinLines: 4, // 1 before + removed + added + 1 after
			wantMaxLines: 4,
		},
		{
			name:         "3 context lines",
			contextLines: 3,
			wantMinLines: 7, // all lines should be included
			wantMaxLines: 8, // could be 8 if replacement shown as remove+add
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateDiff(oldContent, newContent, tt.contextLines)
			
			if len(result.Lines) < tt.wantMinLines || len(result.Lines) > tt.wantMaxLines {
				t.Errorf("Lines count = %v, want between %v and %v", 
					len(result.Lines), tt.wantMinLines, tt.wantMaxLines)
			}
			
			if result.Added != 1 {
				t.Errorf("Added = %v, want 1", result.Added)
			}
			if result.Removed != 1 {
				t.Errorf("Removed = %v, want 1", result.Removed)
			}
		})
	}
}

func TestGenerateDiff_LineNumbers(t *testing.T) {
	oldContent := "line 1\nline 2\nline 3"
	newContent := "line 1\nnew line\nline 3"
	
	result := GenerateDiff(oldContent, newContent, 1)
	
	// Should have: context(line 1), removed(line 2), added(new line), context(line 3)
	expectedLines := []struct {
		Type      LineType
		OldNumber int
		NewNumber int
		Content   string
	}{
		{LineContext, 1, 1, "line 1"},
		{LineRemoved, 2, 0, "line 2"},
		{LineAdded, 0, 2, "new line"},
		{LineContext, 3, 3, "line 3"},
	}
	
	if len(result.Lines) != len(expectedLines) {
		t.Fatalf("Lines count = %v, want %v", len(result.Lines), len(expectedLines))
	}
	
	for i, want := range expectedLines {
		got := result.Lines[i]
		if got.Type != want.Type {
			t.Errorf("Line %d type = %v, want %v", i, got.Type, want.Type)
		}
		if got.OldNumber != want.OldNumber {
			t.Errorf("Line %d old number = %v, want %v", i, got.OldNumber, want.OldNumber)
		}
		if got.NewNumber != want.NewNumber {
			t.Errorf("Line %d new number = %v, want %v", i, got.NewNumber, want.NewNumber)
		}
		if got.Content != want.Content {
			t.Errorf("Line %d content = %q, want %q", i, got.Content, want.Content)
		}
	}
}

func TestGenerateDiff_MultipleChanges(t *testing.T) {
	oldContent := "line 1\nline 2\nline 3\nline 4\nline 5"
	newContent := "line 1\nmodified 2\nline 3\nnew line\nline 5"
	
	result := GenerateDiff(oldContent, newContent, 1)
	
	if result.Added != 2 {
		t.Errorf("Added = %v, want 2", result.Added)
	}
	if result.Removed != 2 {
		t.Errorf("Removed = %v, want 2", result.Removed)
	}
	
	// Count each type
	var contextCount, addedCount, removedCount int
	for _, line := range result.Lines {
		switch line.Type {
		case LineContext:
			contextCount++
		case LineAdded:
			addedCount++
		case LineRemoved:
			removedCount++
		}
	}
	
	if addedCount != 2 {
		t.Errorf("Added line count in diff = %v, want 2", addedCount)
	}
	if removedCount != 2 {
		t.Errorf("Removed line count in diff = %v, want 2", removedCount)
	}
}

func TestSplitLines(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected []string
	}{
		{
			name:     "empty",
			content:  "",
			expected: nil,
		},
		{
			name:     "single line",
			content:  "line 1",
			expected: []string{"line 1"},
		},
		{
			name:     "multiple lines with \\n",
			content:  "line 1\nline 2\nline 3",
			expected: []string{"line 1", "line 2", "line 3"},
		},
		{
			name:     "trailing newline",
			content:  "line 1\nline 2\n",
			expected: []string{"line 1", "line 2"},
		},
		{
			name:     "windows line endings",
			content:  "line 1\r\nline 2\r\n",
			expected: []string{"line 1", "line 2"},
		},
		{
			name:     "mac line endings",
			content:  "line 1\rline 2\r",
			expected: []string{"line 1", "line 2"},
		},
		{
			name:     "mixed line endings",
			content:  "line 1\nline 2\r\nline 3\r",
			expected: []string{"line 1", "line 2", "line 3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitLines(tt.content)
			
			if len(result) != len(tt.expected) {
				t.Fatalf("Length = %v, want %v", len(result), len(tt.expected))
			}
			
			for i, line := range result {
				if line != tt.expected[i] {
					t.Errorf("Line %d = %q, want %q", i, line, tt.expected[i])
				}
			}
		})
	}
}

func TestGenerateDiff_RealWorldExample(t *testing.T) {
	// Simulate editing a Go function
	oldContent := `package main

import "fmt"

func hello(name string) {
	fmt.Println("Hello, " + name)
}

func main() {
	hello("World")
}`

	newContent := `package main

import "fmt"

func hello(name string) {
	fmt.Printf("Hello, %s!\n", name)
}

func greet(name string) {
	fmt.Printf("Greetings, %s\n", name)
}

func main() {
	hello("World")
	greet("Universe")
}`

	result := GenerateDiff(oldContent, newContent, 2)
	
	// Should detect:
	// - 1 removed line (old fmt.Println)
	// - 1 added line (new fmt.Printf)
	// - 3 added lines (new greet function)
	// - 1 added line (greet call in main)
	
	if result.Added < 5 || result.Added > 6 {
		t.Errorf("Added = %v, want between 5 and 6", result.Added)
	}
	if result.Removed != 1 {
		t.Errorf("Removed = %v, want 1", result.Removed)
	}
	
	// Should have a reasonable number of lines (not too many, not too few)
	if len(result.Lines) < 5 || len(result.Lines) > 20 {
		t.Errorf("Total lines = %v, expected between 5 and 20", len(result.Lines))
	}
	
	// Verify some key changes are detected
	hasOldPrintln := false
	hasNewPrintf := false
	hasGreetFunction := false
	
	for _, line := range result.Lines {
		if line.Type == LineRemoved && strings.Contains(line.Content, `fmt.Println("Hello, " + name)`) {
			hasOldPrintln = true
		}
		if line.Type == LineAdded && strings.Contains(line.Content, `fmt.Printf("Hello, %s!\n", name)`) {
			hasNewPrintf = true
		}
		if line.Type == LineAdded && strings.Contains(line.Content, "func greet") {
			hasGreetFunction = true
		}
	}
	
	if !hasOldPrintln {
		t.Error("Should detect removal of old fmt.Println line")
	}
	if !hasNewPrintf {
		t.Error("Should detect addition of new fmt.Printf line")
	}
	if !hasGreetFunction {
		t.Error("Should detect addition of greet function")
	}
}

// Benchmark for performance testing
func BenchmarkGenerateDiff(b *testing.B) {
	oldContent := strings.Repeat("line content here\n", 1000)
	newContent := strings.Replace(oldContent, "line content here", "modified content here", 10)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GenerateDiff(oldContent, newContent, 3)
	}
}