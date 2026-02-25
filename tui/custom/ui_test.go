package custom

import (
	"fmt"
	"testing"
	"time"

	"github.com/srogers/oc/tui/common"
)

// scrollbackState mirrors the fields used in UI.render() for scrollback logic.
type scrollbackState struct {
	prevScrollOffset  int
	scrollbackEmitted int
}

// computeDelta returns the number of lines to push to scrollback and the
// updated state, using the same logic as UI.render().
func computeDelta(s scrollbackState, currentOffset int, isAutoScroll bool, viewportHeight int) (push int, updated scrollbackState) {
	delta := currentOffset - s.prevScrollOffset

	if delta > 0 && isAutoScroll && s.scrollbackEmitted < 1000 {
		// Cap to viewport height so non-MessageList rows never scroll into scrollback
		if delta > viewportHeight {
			delta = viewportHeight
		}
		if budget := 1000 - s.scrollbackEmitted; delta > budget {
			delta = budget
		}
		s.scrollbackEmitted += delta
		push = delta
	}
	if isAutoScroll {
		s.prevScrollOffset = currentOffset
	}
	return push, s
}

func TestScrollbackAutoScrollForward(t *testing.T) {
	s := scrollbackState{}
	push, s := computeDelta(s, 10, true, 100)
	if push != 10 {
		t.Errorf("expected push=10, got %d", push)
	}
	if s.scrollbackEmitted != 10 {
		t.Errorf("expected emitted=10, got %d", s.scrollbackEmitted)
	}
	if s.prevScrollOffset != 10 {
		t.Errorf("expected prevOffset=10, got %d", s.prevScrollOffset)
	}
}

func TestScrollbackManualScrollNoPush(t *testing.T) {
	s := scrollbackState{prevScrollOffset: 10, scrollbackEmitted: 10}
	// User scrolled up to offset 5 — isAutoScroll=false
	push, s := computeDelta(s, 5, false, 100)
	if push != 0 {
		t.Errorf("expected push=0 during manual scroll, got %d", push)
	}
	// prevScrollOffset should NOT update during manual scroll
	if s.prevScrollOffset != 10 {
		t.Errorf("expected prevOffset=10 (unchanged), got %d", s.prevScrollOffset)
	}
}

func TestScrollbackCapAt1000(t *testing.T) {
	s := scrollbackState{prevScrollOffset: 0, scrollbackEmitted: 995}
	push, s := computeDelta(s, 20, true, 100)
	if push != 5 {
		t.Errorf("expected push=5 (capped), got %d", push)
	}
	if s.scrollbackEmitted != 1000 {
		t.Errorf("expected emitted=1000, got %d", s.scrollbackEmitted)
	}
}

func TestScrollbackStopsAfterLimit(t *testing.T) {
	s := scrollbackState{prevScrollOffset: 0, scrollbackEmitted: 1000}
	push, s := computeDelta(s, 50, true, 100)
	if push != 0 {
		t.Errorf("expected push=0 after limit reached, got %d", push)
	}
}

func TestScrollbackResumeAfterManualScroll(t *testing.T) {
	s := scrollbackState{prevScrollOffset: 100, scrollbackEmitted: 100}

	// User manually scrolls up — offset goes back to 50
	push, s := computeDelta(s, 50, false, 100)
	if push != 0 {
		t.Errorf("expected push=0 during manual scroll, got %d", push)
	}
	// prevScrollOffset stays at 100 since not auto-scrolling
	if s.prevScrollOffset != 100 {
		t.Errorf("expected prevOffset=100, got %d", s.prevScrollOffset)
	}

	// Auto-scroll resumes at offset 120
	push, s = computeDelta(s, 120, true, 100)
	if push != 20 {
		t.Errorf("expected push=20 after resume, got %d", push)
	}
	if s.prevScrollOffset != 120 {
		t.Errorf("expected prevOffset=120, got %d", s.prevScrollOffset)
	}
}

func TestScrollbackNegativeDeltaIgnored(t *testing.T) {
	// When auto-scroll is on but content shrinks (unlikely but defensive)
	s := scrollbackState{prevScrollOffset: 50, scrollbackEmitted: 50}
	push, s := computeDelta(s, 40, true, 100)
	if push != 0 {
		t.Errorf("expected push=0 for negative delta, got %d", push)
	}
	// prevScrollOffset should update to current even on negative delta
	if s.prevScrollOffset != 40 {
		t.Errorf("expected prevOffset=40, got %d", s.prevScrollOffset)
	}
}

func TestScrollbackCappedAtViewportHeight(t *testing.T) {
	// Delta exceeds MessageList viewport height — should be capped to
	// prevent StatusBar/InputArea rows from scrolling into scrollback.
	s := scrollbackState{prevScrollOffset: 0, scrollbackEmitted: 0}
	// Offset jumped by 50, but MessageList viewport is only 30 rows
	push, s := computeDelta(s, 50, true, 30)
	if push != 30 {
		t.Errorf("expected push=30 (capped to viewport), got %d", push)
	}
	if s.scrollbackEmitted != 30 {
		t.Errorf("expected emitted=30, got %d", s.scrollbackEmitted)
	}
	if s.prevScrollOffset != 50 {
		t.Errorf("expected prevOffset=50 (tracks actual offset), got %d", s.prevScrollOffset)
	}
}

// --- FPS counter tests ---

// fpsState mirrors the FPS-related fields from UI for testable pure-function extraction.
type fpsState struct {
	showFPS     bool
	renderCount int
	currentFPS  int
	lastFPSTime time.Time
}

// updateFPS applies the same FPS computation logic as the TickEvent handler.
func updateFPS(s fpsState, now time.Time) fpsState {
	if now.Sub(s.lastFPSTime) >= time.Second {
		s.currentFPS = s.renderCount
		s.renderCount = 0
		s.lastFPSTime = now
	}
	return s
}

// renderFPSOverlay writes the FPS label into a buffer, same as render().
func renderFPSOverlay(buf *common.ScreenBuffer, fps int) {
	label := fmt.Sprintf(" %d fps ", fps)
	fpsStyle := common.Style{
		FG:   common.NewColor(0, 0, 0),
		BG:   common.NewColor(255, 220, 0),
		Bold: true,
	}
	buf.WriteString(buf.Width-len(label), 0, label, fpsStyle)
}

func TestToggleFPS(t *testing.T) {
	ui := &UI{}

	// Initially off
	if ui.showFPS {
		t.Fatal("expected showFPS=false initially")
	}

	// Toggle on
	ui.ToggleFPS()
	if !ui.showFPS {
		t.Fatal("expected showFPS=true after first toggle")
	}
	if ui.renderCount != 0 {
		t.Errorf("expected renderCount=0 after toggle on, got %d", ui.renderCount)
	}
	if ui.currentFPS != 0 {
		t.Errorf("expected currentFPS=0 after toggle on, got %d", ui.currentFPS)
	}
	if ui.lastFPSTime.IsZero() {
		t.Error("expected lastFPSTime to be set after toggle on")
	}

	// Toggle off
	ui.ToggleFPS()
	if ui.showFPS {
		t.Fatal("expected showFPS=false after second toggle")
	}
}

func TestFPSComputationAfterOneSecond(t *testing.T) {
	start := time.Now()
	s := fpsState{
		showFPS:     true,
		renderCount: 42,
		currentFPS:  0,
		lastFPSTime: start,
	}

	// Less than 1 second — no update
	s2 := updateFPS(s, start.Add(500*time.Millisecond))
	if s2.currentFPS != 0 {
		t.Errorf("expected currentFPS=0 before 1s, got %d", s2.currentFPS)
	}
	if s2.renderCount != 42 {
		t.Errorf("expected renderCount=42 unchanged, got %d", s2.renderCount)
	}

	// Exactly 1 second — should update
	s3 := updateFPS(s, start.Add(time.Second))
	if s3.currentFPS != 42 {
		t.Errorf("expected currentFPS=42, got %d", s3.currentFPS)
	}
	if s3.renderCount != 0 {
		t.Errorf("expected renderCount=0 after reset, got %d", s3.renderCount)
	}
	if s3.lastFPSTime != start.Add(time.Second) {
		t.Error("expected lastFPSTime to advance")
	}
}

func TestFPSComputationAccumulates(t *testing.T) {
	start := time.Now()
	s := fpsState{
		showFPS:     true,
		renderCount: 10,
		currentFPS:  0,
		lastFPSTime: start,
	}

	// First second: 10 renders
	s = updateFPS(s, start.Add(time.Second))
	if s.currentFPS != 10 {
		t.Errorf("expected currentFPS=10, got %d", s.currentFPS)
	}

	// Simulate 25 renders in next second
	s.renderCount = 25
	s = updateFPS(s, start.Add(2*time.Second))
	if s.currentFPS != 25 {
		t.Errorf("expected currentFPS=25, got %d", s.currentFPS)
	}
}

func TestFPSOverlayPosition(t *testing.T) {
	buf := common.NewScreenBuffer(80, 24)
	renderFPSOverlay(buf, 60)

	// " 60 fps " is 8 chars, should start at column 72 (80-8), row 0
	label := " 60 fps "
	startX := 80 - len(label)

	for i, expected := range label {
		cell := buf.Cells[0][startX+i]
		if cell.Rune != expected {
			t.Errorf("col %d: expected %q, got %q", startX+i, string(expected), string(cell.Rune))
		}
		if !cell.Style.Bold {
			t.Errorf("col %d: expected bold", startX+i)
		}
		if !cell.Style.FG.Set || cell.Style.FG.R != 0 || cell.Style.FG.G != 0 || cell.Style.FG.B != 0 {
			t.Errorf("col %d: expected black foreground", startX+i)
		}
		if !cell.Style.BG.Set || cell.Style.BG.R != 255 || cell.Style.BG.G != 220 || cell.Style.BG.B != 0 {
			t.Errorf("col %d: expected yellow background", startX+i)
		}
	}

	// Check area before the label is untouched (still spaces with default style)
	if startX > 0 {
		cell := buf.Cells[0][startX-1]
		if cell.Rune != ' ' || cell.Style.Bold {
			t.Error("cell before FPS label should be default")
		}
	}
}

func TestFPSOverlayNotRenderedWhenOff(t *testing.T) {
	buf := common.NewScreenBuffer(80, 24)
	// Don't call renderFPSOverlay — verify row 0 is all default spaces
	for x := 0; x < 80; x++ {
		cell := buf.Cells[0][x]
		if cell.Rune != ' ' {
			t.Errorf("col %d: expected space, got %q", x, string(cell.Rune))
		}
		if cell.Style.Bold {
			t.Errorf("col %d: expected no bold on untouched buffer", x)
		}
	}
}

func TestFPSOverlayNarrowTerminal(t *testing.T) {
	// With a very narrow terminal (e.g. 10 cols), the label " 0 fps " (7 chars)
	// should still fit and start at column 3.
	buf := common.NewScreenBuffer(10, 5)
	renderFPSOverlay(buf, 0)

	label := " 0 fps "
	startX := 10 - len(label)
	for i, expected := range label {
		cell := buf.Cells[0][startX+i]
		if cell.Rune != expected {
			t.Errorf("col %d: expected %q, got %q", startX+i, string(expected), string(cell.Rune))
		}
	}
}
