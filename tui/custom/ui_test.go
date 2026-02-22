package custom

import "testing"

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
