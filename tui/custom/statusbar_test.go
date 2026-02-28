package custom

import (
	"testing"

	"github.com/seanrogers2657/oc/domain"
	"github.com/seanrogers2657/oc/tui/common"
)

func TestFormatTokens(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{0, "0"},
		{1, "1"},
		{999, "999"},
		{1000, "1.0K"},
		{1500, "1.5K"},
		{10000, "10.0K"},
		{123456, "123.5K"},
		{999999, "1000.0K"},
		{1000000, "1.0M"},
		{1500000, "1.5M"},
		{10000000, "10.0M"},
	}
	for _, tt := range tests {
		got := formatTokens(tt.n)
		if got != tt.want {
			t.Errorf("formatTokens(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestShortenHome(t *testing.T) {
	// We can't predict the home dir in CI, but we can test the logic
	// with paths that won't match home
	got := shortenHome("/some/random/path")
	if got != "/some/random/path" {
		t.Errorf("non-home path should be unchanged, got %q", got)
	}
}

func TestNewStatusBar(t *testing.T) {
	sb := NewStatusBar(nil)
	if sb == nil {
		t.Fatal("NewStatusBar returned nil")
	}
}

func TestStatusBarMinSize(t *testing.T) {
	sb := NewStatusBar(nil)
	w, h := sb.MinSize()
	if w != 20 || h != 1 {
		t.Errorf("MinSize() = (%d, %d), want (20, 1)", w, h)
	}
}

func TestStatusBarFocused(t *testing.T) {
	sb := NewStatusBar(nil)
	if sb.Focused() {
		t.Error("StatusBar should never be focused")
	}
	sb.SetFocused(true) // no-op
	if sb.Focused() {
		t.Error("StatusBar should never be focused even after SetFocused(true)")
	}
}

func TestStatusBarUpdateTick(t *testing.T) {
	sb := NewStatusBar(nil)
	initial := sb.frame

	dirty := sb.Update(common.TickEvent{})
	if !dirty {
		t.Error("tick should return dirty=true")
	}
	if sb.frame != initial+1 {
		t.Errorf("frame should increment on tick: got %d, want %d", sb.frame, initial+1)
	}
}

func TestStatusBarUpdateNonTick(t *testing.T) {
	sb := NewStatusBar(nil)
	dirty := sb.Update(domain.KeyEvent{Key: domain.KeyRune, Rune: 'a'})
	if dirty {
		t.Error("non-tick event should return dirty=false")
	}
}

func TestStatusBarRenderNilProvider(t *testing.T) {
	sb := NewStatusBar(nil)
	buf := common.NewScreenBuffer(40, 1)
	bounds := common.Rect{X: 0, Y: 0, Width: 40, Height: 1}

	sb.Render(buf, bounds)

	// Should render "oc" when no status provider
	if buf.Cells[0][1].Rune != 'o' || buf.Cells[0][2].Rune != 'c' {
		t.Error("expected 'oc' rendered with nil status provider")
	}
}

func TestStatusBarRenderZeroBounds(t *testing.T) {
	sb := NewStatusBar(nil)
	buf := common.NewScreenBuffer(40, 1)

	// Should not panic with zero bounds
	sb.Render(buf, common.Rect{X: 0, Y: 0, Width: 0, Height: 0})
	sb.Render(buf, common.Rect{X: 0, Y: 0, Width: 0, Height: 1})
	sb.Render(buf, common.Rect{X: 0, Y: 0, Width: 1, Height: 0})
}

func TestStatusBarRenderWithStatus(t *testing.T) {
	sb := NewStatusBar(func() StatusInfo {
		return StatusInfo{
			Model:    "gpt-4o",
			Provider: "openai",
			Status:   "idle",
			Tokens:   1500,
		}
	})

	buf := common.NewScreenBuffer(80, 1)
	bounds := common.Rect{X: 0, Y: 0, Width: 80, Height: 1}

	sb.Render(buf, bounds)

	// Check model name appears in rendered output
	rendered := cellRowToString(buf, 0)
	if !containsSubstring(rendered, "gpt-4o") {
		t.Errorf("expected 'gpt-4o' in rendered output: %q", rendered)
	}
	if !containsSubstring(rendered, "openai") {
		t.Errorf("expected 'openai' in rendered output: %q", rendered)
	}
}

func TestStatusBarRenderBusy(t *testing.T) {
	sb := NewStatusBar(func() StatusInfo {
		return StatusInfo{
			Model:  "test",
			Status: "busy",
		}
	})

	buf := common.NewScreenBuffer(40, 1)
	bounds := common.Rect{X: 0, Y: 0, Width: 40, Height: 1}

	// Render with different frames to exercise spinner
	for i := range 12 {
		sb.frame = i
		sb.Render(buf, bounds)
	}
	// Should not panic
}

func TestStatusBarRenderWithDetail(t *testing.T) {
	sb := NewStatusBar(func() StatusInfo {
		return StatusInfo{
			Model:    "claude-sonnet",
			Provider: "anthropic",
			Detail:   "max",
			Status:   "idle",
		}
	})

	buf := common.NewScreenBuffer(80, 1)
	bounds := common.Rect{X: 0, Y: 0, Width: 80, Height: 1}
	sb.Render(buf, bounds)

	rendered := cellRowToString(buf, 0)
	if !containsSubstring(rendered, "anthropic/max") {
		t.Errorf("expected 'anthropic/max' in rendered output: %q", rendered)
	}
}

func TestStatusBarRenderWithCost(t *testing.T) {
	sb := NewStatusBar(func() StatusInfo {
		return StatusInfo{
			Model:  "test",
			Status: "idle",
			Cost:   "$0.03",
		}
	})

	buf := common.NewScreenBuffer(80, 1)
	bounds := common.Rect{X: 0, Y: 0, Width: 80, Height: 1}
	sb.Render(buf, bounds)

	rendered := cellRowToString(buf, 0)
	if !containsSubstring(rendered, "$0.03") {
		t.Errorf("expected '$0.03' in rendered output: %q", rendered)
	}
}

func TestStatusBarRenderContextTokens(t *testing.T) {
	sb := NewStatusBar(func() StatusInfo {
		return StatusInfo{
			Model:         "test",
			Status:        "idle",
			Tokens:        5000,
			ContextTokens: 2000,
		}
	})

	buf := common.NewScreenBuffer(80, 1)
	bounds := common.Rect{X: 0, Y: 0, Width: 80, Height: 1}
	sb.Render(buf, bounds)

	rendered := cellRowToString(buf, 0)
	if !containsSubstring(rendered, "ctx") {
		t.Errorf("expected 'ctx' in rendered output: %q", rendered)
	}
	if !containsSubstring(rendered, "5.0K") {
		t.Errorf("expected '5.0K' tokens in rendered output: %q", rendered)
	}
}

// cellRowToString extracts the text content of a screen buffer row.
func cellRowToString(buf *common.ScreenBuffer, y int) string {
	var s []rune
	for x := 0; x < buf.Width; x++ {
		s = append(s, buf.Cells[y][x].Rune)
	}
	return string(s)
}

// containsSubstring checks if s contains sub.
func containsSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
