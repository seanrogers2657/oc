package common

import (
	"strings"
	"testing"
)

func TestNewScreenBuffer(t *testing.T) {
	buf := NewScreenBuffer(80, 24)
	if buf.Width != 80 || buf.Height != 24 {
		t.Fatalf("expected 80x24, got %dx%d", buf.Width, buf.Height)
	}
	// All cells should be spaces with default style
	for y := 0; y < buf.Height; y++ {
		for x := 0; x < buf.Width; x++ {
			c := buf.Cells[y][x]
			if c.Rune != ' ' {
				t.Fatalf("cell (%d,%d) = %q, want ' '", x, y, c.Rune)
			}
			if c.Style != (Style{}) {
				t.Fatalf("cell (%d,%d) has non-zero style", x, y)
			}
		}
	}
}

func TestScreenBufferSet(t *testing.T) {
	buf := NewScreenBuffer(10, 5)
	s := Style{Bold: true}

	buf.Set(3, 2, 'X', s)
	c := buf.Cells[2][3]
	if c.Rune != 'X' || !c.Style.Bold {
		t.Fatalf("Set failed: got %+v", c)
	}

	// Out of bounds should not panic
	buf.Set(-1, 0, 'A', Style{})
	buf.Set(0, -1, 'A', Style{})
	buf.Set(10, 0, 'A', Style{})
	buf.Set(0, 5, 'A', Style{})
}

func TestScreenBufferWriteString(t *testing.T) {
	buf := NewScreenBuffer(10, 3)
	s := Style{Italic: true}

	n := buf.WriteString(2, 1, "hello", s)
	if n != 5 {
		t.Fatalf("WriteString returned %d, want 5", n)
	}

	expected := "hello"
	for i, r := range expected {
		c := buf.Cells[1][2+i]
		if c.Rune != r {
			t.Fatalf("pos %d: got %q, want %q", i, c.Rune, r)
		}
		if !c.Style.Italic {
			t.Fatalf("pos %d: missing italic", i)
		}
	}

	// Surrounding cells unchanged
	if buf.Cells[1][0].Rune != ' ' || buf.Cells[1][7].Rune != ' ' {
		t.Fatal("surrounding cells modified")
	}
}

func TestScreenBufferWriteStringClipping(t *testing.T) {
	buf := NewScreenBuffer(5, 1)

	// Write string that extends past right edge
	n := buf.WriteString(3, 0, "hello", Style{})
	if n != 2 {
		t.Fatalf("WriteString returned %d, want 2 (clipped)", n)
	}
	if buf.Cells[0][3].Rune != 'h' || buf.Cells[0][4].Rune != 'e' {
		t.Fatal("clipped write incorrect")
	}
}

func TestScreenBufferWriteStringOutOfBounds(t *testing.T) {
	buf := NewScreenBuffer(10, 3)

	// Write to row out of bounds
	n := buf.WriteString(0, 5, "hello", Style{})
	if n != 0 {
		t.Fatalf("out-of-bounds WriteString returned %d, want 0", n)
	}

	n = buf.WriteString(0, -1, "hello", Style{})
	if n != 0 {
		t.Fatalf("negative row WriteString returned %d, want 0", n)
	}
}

func TestScreenBufferFill(t *testing.T) {
	buf := NewScreenBuffer(10, 5)
	s := Style{FG: NewColor(255, 0, 0)}

	buf.Fill(Rect{X: 2, Y: 1, Width: 3, Height: 2}, '#', s)

	for y := 1; y <= 2; y++ {
		for x := 2; x <= 4; x++ {
			c := buf.Cells[y][x]
			if c.Rune != '#' {
				t.Fatalf("(%d,%d) = %q, want '#'", x, y, c.Rune)
			}
		}
	}

	// Outside fill region
	if buf.Cells[0][2].Rune != ' ' {
		t.Fatal("cell above fill region modified")
	}
	if buf.Cells[1][1].Rune != ' ' {
		t.Fatal("cell left of fill region modified")
	}
}

func TestScreenBufferClear(t *testing.T) {
	buf := NewScreenBuffer(5, 3)
	buf.Set(2, 1, 'X', Style{Bold: true})

	buf.Clear()

	c := buf.Cells[1][2]
	if c.Rune != ' ' || c.Style.Bold {
		t.Fatalf("Clear did not reset cell: %+v", c)
	}
}

func TestScreenBufferResize(t *testing.T) {
	buf := NewScreenBuffer(5, 3)
	buf.Set(1, 1, 'A', Style{Bold: true})

	// Grow
	buf.Resize(10, 6)
	if buf.Width != 10 || buf.Height != 6 {
		t.Fatalf("resize to 10x6 failed: got %dx%d", buf.Width, buf.Height)
	}
	if buf.Cells[1][1].Rune != 'A' {
		t.Fatal("preserved cell lost on grow")
	}
	if buf.Cells[5][9].Rune != ' ' {
		t.Fatal("new cell not initialized")
	}

	// Shrink
	buf.Resize(3, 2)
	if buf.Width != 3 || buf.Height != 2 {
		t.Fatalf("resize to 3x2 failed: got %dx%d", buf.Width, buf.Height)
	}
	if buf.Cells[1][1].Rune != 'A' {
		t.Fatal("preserved cell lost on shrink")
	}
}

func TestScreenBufferWriteWrapped(t *testing.T) {
	buf := NewScreenBuffer(10, 5)

	lines := buf.WriteWrapped(Rect{X: 0, Y: 0, Width: 5, Height: 3}, "hello world", Style{})
	if lines != 3 {
		t.Fatalf("WriteWrapped returned %d lines, want 3", lines)
	}

	// First line: "hello"
	for i, r := range "hello" {
		if buf.Cells[0][i].Rune != r {
			t.Fatalf("line 0 pos %d: got %q want %q", i, buf.Cells[0][i].Rune, r)
		}
	}
	// Second line: " worl" (space then 4 chars)
	if buf.Cells[1][0].Rune != ' ' {
		t.Fatalf("line 1 pos 0: got %q want ' '", buf.Cells[1][0].Rune)
	}
}

func TestScreenBufferWriteWrappedNewline(t *testing.T) {
	buf := NewScreenBuffer(20, 5)

	lines := buf.WriteWrapped(Rect{X: 0, Y: 0, Width: 20, Height: 5}, "line1\nline2\nline3", Style{})
	if lines != 3 {
		t.Fatalf("WriteWrapped returned %d lines, want 3", lines)
	}

	row0 := cellsToString(buf.Cells[0][:5])
	row1 := cellsToString(buf.Cells[1][:5])
	row2 := cellsToString(buf.Cells[2][:5])
	if row0 != "line1" {
		t.Fatalf("row 0: got %q want %q", row0, "line1")
	}
	if row1 != "line2" {
		t.Fatalf("row 1: got %q want %q", row1, "line2")
	}
	if row2 != "line3" {
		t.Fatalf("row 2: got %q want %q", row2, "line3")
	}
}

func TestScreenBufferDiffEmpty(t *testing.T) {
	buf := NewScreenBuffer(5, 3)
	prev := NewScreenBuffer(5, 3)

	diff := buf.Diff(prev)
	if diff != "" {
		t.Fatalf("identical buffers should produce empty diff, got %d bytes", len(diff))
	}
}

func TestScreenBufferDiffChanged(t *testing.T) {
	prev := NewScreenBuffer(10, 3)
	buf := NewScreenBuffer(10, 3)

	buf.Set(5, 1, 'X', Style{})

	diff := buf.Diff(prev)
	if diff == "" {
		t.Fatal("changed cell should produce non-empty diff")
	}
	if !strings.Contains(diff, "X") {
		t.Fatal("diff should contain 'X'")
	}
}

func TestScreenBufferDiffStyle(t *testing.T) {
	prev := NewScreenBuffer(10, 3)
	buf := NewScreenBuffer(10, 3)

	buf.Set(0, 0, 'A', Style{Bold: true})

	diff := buf.Diff(prev)
	// Should contain bold escape and the character
	if !strings.Contains(diff, "\x1b[1m") {
		t.Fatal("diff should contain bold escape")
	}
	if !strings.Contains(diff, "A") {
		t.Fatal("diff should contain 'A'")
	}
}

func TestScreenBufferFullRender(t *testing.T) {
	buf := NewScreenBuffer(3, 2)
	buf.Set(0, 0, 'A', Style{})
	buf.Set(1, 0, 'B', Style{})

	out := buf.FullRender()
	if !strings.Contains(out, "A") || !strings.Contains(out, "B") {
		t.Fatal("FullRender should contain written characters")
	}
}

func TestScreenBufferDiffNilPrev(t *testing.T) {
	buf := NewScreenBuffer(3, 2)
	buf.Set(0, 0, 'X', Style{})

	// Diff with nil prev should render everything (same as FullRender)
	diff := buf.Diff(nil)
	if !strings.Contains(diff, "X") {
		t.Fatal("Diff(nil) should render all non-empty cells")
	}
}

func TestScreenBufferDiffWritesFullRowFromChange(t *testing.T) {
	// When a cell changes on a row, the diff must write through the
	// end of that row so that stale characters after the change are erased.
	prev := NewScreenBuffer(10, 3)
	prev.WriteString(0, 1, "ABCDEFGHIJ", Style{Bold: true})

	buf := NewScreenBuffer(10, 3)
	buf.WriteString(0, 1, "XY", Style{})
	// Columns 2-9 are default spaces from NewScreenBuffer.

	diff := buf.Diff(prev)

	// The diff must contain "XY" AND enough spaces to overwrite "CDEFGHIJ".
	// Count characters written for row 1 by looking for the row's content.
	// The diff should rewrite from the first changed cell (col 0) through col 9.
	if !strings.Contains(diff, "XY") {
		t.Fatal("diff should contain new content 'XY'")
	}

	// Apply the diff by parsing: the row should now be "XY        " (XY + 8 spaces).
	// Simplest check: the diff must contain at least 10 rune-writes for row 1.
	// Since the entire row from col 0 is rewritten, we can verify by counting
	// non-escape printable characters on that row.
	//
	// Better approach: apply the diff to prev and check the resulting state
	// by simulating. Instead, just verify the trailing characters are present
	// by checking the diff contains spaces after XY.
	afterXY := strings.Index(diff, "XY")
	if afterXY < 0 {
		t.Fatal("XY not found in diff")
	}
	// After "XY" in the diff output, there should be space characters
	// (the rest of the row), possibly interspersed with style escapes.
	remainder := diff[afterXY+2:]
	spaceCount := strings.Count(remainder, " ")
	if spaceCount < 8 {
		t.Fatalf("expected at least 8 trailing spaces in diff to clear row, got %d", spaceCount)
	}
}

func TestScreenBufferDiffSkipsUnchangedRows(t *testing.T) {
	prev := NewScreenBuffer(10, 3)
	prev.WriteString(0, 0, "unchanged", Style{})
	prev.WriteString(0, 2, "also same", Style{})

	buf := NewScreenBuffer(10, 3)
	buf.WriteString(0, 0, "unchanged", Style{})
	buf.WriteString(0, 1, "new", Style{}) // only row 1 changed
	buf.WriteString(0, 2, "also same", Style{})

	diff := buf.Diff(prev)

	// Should contain the new content
	if !strings.Contains(diff, "new") {
		t.Fatal("diff should contain 'new'")
	}
	// Should NOT contain content from unchanged rows
	if strings.Contains(diff, "unchanged") {
		t.Fatal("diff should not rewrite unchanged rows")
	}
	if strings.Contains(diff, "also same") {
		t.Fatal("diff should not rewrite unchanged rows")
	}
}

func cellsToString(cells []Cell) string {
	var b strings.Builder
	for _, c := range cells {
		b.WriteRune(c.Rune)
	}
	return b.String()
}
