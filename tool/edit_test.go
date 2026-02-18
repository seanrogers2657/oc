package tool

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEditSingleReplacement(t *testing.T) {
	ctx, dir := makeTestCtx(t)
	writeTestFile(t, dir, "e.txt", "hello world\n")

	tool := NewEdit()
	r := tool.Execute(ctx, `{"file_path":"e.txt","old_string":"hello","new_string":"goodbye"}`)
	if r.Error != nil {
		t.Fatal(r.Error)
	}

	data, err := os.ReadFile(filepath.Join(dir, "e.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "goodbye world\n" {
		t.Errorf("expected 'goodbye world\\n', got %q", string(data))
	}
	if !strings.Contains(r.Title, "Edit e.txt") {
		t.Errorf("unexpected title: %s", r.Title)
	}
	if !strings.Contains(r.Output, "1 replacement") {
		t.Errorf("output should mention replacement count, got:\n%s", r.Output)
	}
	
	// Check that diff information is present
	if r.Diff == nil {
		t.Error("expected diff information")
	} else {
		if r.Diff.Added != 1 || r.Diff.Removed != 1 {
			t.Errorf("expected 1 added and 1 removed line, got +%d -%d", r.Diff.Added, r.Diff.Removed)
		}
	}
}

func TestEditReplaceAll(t *testing.T) {
	ctx, dir := makeTestCtx(t)
	writeTestFile(t, dir, "ra.txt", "aaa bbb aaa\n")

	tool := NewEdit()
	r := tool.Execute(ctx, `{"file_path":"ra.txt","old_string":"aaa","new_string":"ccc","replace_all":true}`)
	if r.Error != nil {
		t.Fatal(r.Error)
	}

	data, err := os.ReadFile(filepath.Join(dir, "ra.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "ccc bbb ccc\n" {
		t.Errorf("expected 'ccc bbb ccc\\n', got %q", string(data))
	}
	if !strings.Contains(r.Title, "Edit ra.txt") {
		t.Errorf("unexpected title: %s", r.Title)
	}
	if !strings.Contains(r.Output, "2 replacement") {
		t.Errorf("output should mention replacement count, got:\n%s", r.Output)
	}
	
	// Check that diff information is present
	if r.Diff == nil {
		t.Error("expected diff information")
	}
}

func TestEditMultipleWithoutReplaceAllFails(t *testing.T) {
	ctx, dir := makeTestCtx(t)
	writeTestFile(t, dir, "m.txt", "foo bar foo\n")

	tool := NewEdit()
	r := tool.Execute(ctx, `{"file_path":"m.txt","old_string":"foo","new_string":"baz"}`)
	if r.Error == nil {
		t.Fatal("expected error when old_string matches multiple times without replace_all")
	}
}

func TestEditNotFound(t *testing.T) {
	ctx, dir := makeTestCtx(t)
	writeTestFile(t, dir, "nf.txt", "hello\n")

	tool := NewEdit()
	r := tool.Execute(ctx, `{"file_path":"nf.txt","old_string":"missing","new_string":"x"}`)
	if r.Error == nil {
		t.Fatal("expected error when old_string not found")
	}
}

func TestEditIdenticalStrings(t *testing.T) {
	ctx, dir := makeTestCtx(t)
	writeTestFile(t, dir, "id.txt", "same\n")

	tool := NewEdit()
	r := tool.Execute(ctx, `{"file_path":"id.txt","old_string":"same","new_string":"same"}`)
	if r.Error == nil {
		t.Fatal("expected error when old_string and new_string are identical")
	}
}

func TestEditMissingFile(t *testing.T) {
	ctx, _ := makeTestCtx(t)
	tool := NewEdit()
	r := tool.Execute(ctx, `{"file_path":"nope.txt","old_string":"a","new_string":"b"}`)
	if r.Error == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestEditMultilineReplacement(t *testing.T) {
	ctx, dir := makeTestCtx(t)
	writeTestFile(t, dir, "ml.txt", "line1\nline2\nline3\n")

	tool := NewEdit()
	r := tool.Execute(ctx, `{"file_path":"ml.txt","old_string":"line1\nline2","new_string":"replaced"}`)
	if r.Error != nil {
		t.Fatal(r.Error)
	}

	data, err := os.ReadFile(filepath.Join(dir, "ml.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "replaced\nline3\n" {
		t.Errorf("expected 'replaced\\nline3\\n', got %q", string(data))
	}
	if !strings.Contains(r.Output, "replacement") {
		t.Errorf("output should mention replacement, got:\n%s", r.Output)
	}
}

func TestEditSnippet(t *testing.T) {
	tests := []struct {
		name       string
		content    string
		target     string
		wantLines  []string // substrings that must appear
		wantAbsent []string // substrings that must NOT appear
	}{
		{
			name:    "mid-file with context",
			content: "L1\nL2\nL3\nL4\nL5\nL6\nL7\nL8\nL9\nL10",
			target:  "L5",
			// target on line 5 (idx 4), context=3 → lines 2..8
			wantLines:  []string{"L2", "L3", "L4", "L5", "L6", "L7", "L8"},
			wantAbsent: []string{"  L1\n", "  L9\n", "  L10\n"},
		},
		{
			name:    "near start clamps to line 1",
			content: "A\nB\nC\nD\nE\nF\nG\nH",
			target:  "B",
			wantLines:  []string{"A", "B", "C", "D", "E"},
			wantAbsent: []string{"  F\n", "  G\n"},
		},
		{
			name:    "near end clamps to last line",
			content: "A\nB\nC\nD\nE\nF\nG\nH",
			target:  "G",
			wantLines:  []string{"D", "E", "F", "G", "H"},
			wantAbsent: []string{"  A\n", "  B\n", "  C\n"},
		},
		{
			name:    "multiline target shows all changed lines",
			content: "A\nB\nC\nD\nE\nF\nG\nH\nI\nJ",
			target:  "D\nE\nF",
			// starts line 4, spans 2 newlines → endLine=5, window [1,9)
			wantLines: []string{"A", "B", "C", "D", "E", "F", "G", "H", "I"},
		},
		{
			name:      "single line file",
			content:   "only line",
			target:    "only line",
			wantLines: []string{"only line"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := editSnippet(tt.content, tt.target)
			if got == "" {
				t.Fatal("editSnippet returned empty string")
			}

			// Every non-separator line should have line-number prefix (6 digits + 2 spaces).
			for _, ol := range strings.Split(strings.TrimRight(got, "\n"), "\n") {
				if ol == "   ..." {
					continue
				}
				if len(ol) < 8 || ol[6:8] != "  " {
					t.Errorf("line missing line-number prefix: %q", ol)
				}
			}

			for _, want := range tt.wantLines {
				if !strings.Contains(got, want) {
					t.Errorf("expected output to contain %q, got:\n%s", want, got)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(got, absent) {
					t.Errorf("expected output NOT to contain %q, got:\n%s", absent, got)
				}
			}
		})
	}
}

func TestEditSnippetMultipleOccurrences(t *testing.T) {
	// Two occurrences far apart → disjoint windows separated by "..."
	// 20 lines: target on lines 3 and 18 (indices 2 and 17)
	var lines []string
	for i := 1; i <= 20; i++ {
		if i == 3 || i == 18 {
			lines = append(lines, "TARGET")
		} else {
			lines = append(lines, fmt.Sprintf("line%d", i))
		}
	}
	content := strings.Join(lines, "\n")

	got := editSnippet(content, "TARGET")

	// Should contain both TARGET occurrences
	if strings.Count(got, "TARGET") != 2 {
		t.Errorf("expected 2 occurrences of TARGET, got %d:\n%s",
			strings.Count(got, "TARGET"), got)
	}

	// Should have a "..." separator between disjoint windows
	if !strings.Contains(got, "   ...") {
		t.Errorf("expected '   ...' separator between disjoint windows, got:\n%s", got)
	}

	// Context from first window (line 3 ± 3 → lines 1-6)
	if !strings.Contains(got, "line1") {
		t.Errorf("should contain line1 (context for first occurrence), got:\n%s", got)
	}
	if !strings.Contains(got, "line6") {
		t.Errorf("should contain line6 (context for first occurrence), got:\n%s", got)
	}

	// Context from second window (line 18 ± 3 → lines 15-20)
	if !strings.Contains(got, "line15") {
		t.Errorf("should contain line15 (context for second occurrence), got:\n%s", got)
	}
	if !strings.Contains(got, "line20") {
		t.Errorf("should contain line20 (context for second occurrence), got:\n%s", got)
	}

	// Lines between the windows should be absent
	if strings.Contains(got, "line10") {
		t.Errorf("should NOT contain line10 (between windows), got:\n%s", got)
	}
}

func TestEditSnippetOverlappingWindowsMerge(t *testing.T) {
	// Two occurrences close together → windows merge, no "..." separator
	// target on lines 5 and 8 (indices 4 and 7); windows overlap
	var lines []string
	for i := 1; i <= 15; i++ {
		if i == 5 || i == 8 {
			lines = append(lines, "HIT")
		} else {
			lines = append(lines, fmt.Sprintf("L%d", i))
		}
	}
	content := strings.Join(lines, "\n")

	got := editSnippet(content, "HIT")

	if strings.Count(got, "HIT") != 2 {
		t.Errorf("expected 2 HIT occurrences, got %d:\n%s",
			strings.Count(got, "HIT"), got)
	}

	// Windows overlap → no separator
	if strings.Contains(got, "...") {
		t.Errorf("overlapping windows should merge without separator, got:\n%s", got)
	}
}

func TestEditSnippetLineNumbers(t *testing.T) {
	content := "A\nB\nC\nD\nE"
	got := editSnippet(content, "C")

	// C is on line 3, context=3 → shows all 5 lines
	outputLines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(outputLines) != 5 {
		t.Fatalf("expected 5 lines, got %d:\n%s", len(outputLines), got)
	}

	for i, line := range outputLines {
		wantPrefix := fmt.Sprintf("%6d  ", i+1)
		if !strings.HasPrefix(line, wantPrefix) {
			t.Errorf("line %d: expected prefix %q, got %q", i, wantPrefix, line)
		}
	}
}
