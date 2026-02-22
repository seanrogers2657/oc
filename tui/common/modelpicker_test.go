package common

import "testing"

func newTestModelPicker() *ModelPicker {
	models := []string{"gpt-4o", "gpt-4o-mini", "claude-sonnet", "llama3", "llama3:70b"}
	fetch := func() ([]string, error) { return models, nil }
	return NewModelPicker(fetch, func(string) {})
}

func TestModelPickerOpenClose(t *testing.T) {
	p := newTestModelPicker()

	if p.Active() {
		t.Fatal("picker should start inactive")
	}

	p.Open("gpt-4o")
	if !p.Active() {
		t.Fatal("picker should be active after Open()")
	}

	p.Close()
	if p.Active() {
		t.Fatal("picker should be inactive after Close()")
	}
}

func TestModelPickerEscapeCloses(t *testing.T) {
	p := newTestModelPicker()
	p.Open("gpt-4o")

	dirty, consumed := p.Update(KeyEvent{Key: KeyEscape})
	if !dirty || !consumed {
		t.Error("escape should be dirty and consumed")
	}
	if p.Active() {
		t.Error("picker should be closed after escape")
	}
}

func TestModelPickerEnterSelects(t *testing.T) {
	var selected string
	fetch := func() ([]string, error) { return []string{"model-a", "model-b"}, nil }
	p := NewModelPicker(fetch, func(m string) { selected = m })

	p.Open("")
	// Wait for async model fetch to complete by manually setting results
	p.models = []string{"model-a", "model-b"}
	p.loading = false
	p.refilter()

	dirty, consumed := p.Update(KeyEvent{Key: KeyEnter})
	if !dirty || !consumed {
		t.Error("enter should be dirty and consumed")
	}
	if selected != "model-a" {
		t.Errorf("expected model-a selected, got %q", selected)
	}
	if p.Active() {
		t.Error("picker should close after selecting")
	}
}

func TestModelPickerArrowNavigation(t *testing.T) {
	p := newTestModelPicker()
	p.Open("gpt-4o")
	// Manually set models to avoid async
	p.models = []string{"a", "b", "c"}
	p.loading = false
	p.refilter()

	if p.selected != 0 {
		t.Fatalf("selected should start at 0, got %d", p.selected)
	}

	p.Update(KeyEvent{Key: KeyDown})
	if p.selected != 1 {
		t.Errorf("selected should be 1 after down, got %d", p.selected)
	}

	p.Update(KeyEvent{Key: KeyUp})
	if p.selected != 0 {
		t.Errorf("selected should be 0 after up, got %d", p.selected)
	}

	// Ctrl+n/p navigation
	p.Update(KeyEvent{Key: KeyRune, Rune: 'n', Ctrl: true})
	if p.selected != 1 {
		t.Errorf("selected should be 1 after ctrl+n, got %d", p.selected)
	}

	p.Update(KeyEvent{Key: KeyRune, Rune: 'p', Ctrl: true})
	if p.selected != 0 {
		t.Errorf("selected should be 0 after ctrl+p, got %d", p.selected)
	}

	// Ctrl+j/k navigation
	p.Update(KeyEvent{Key: KeyRune, Rune: 'j', Ctrl: true})
	if p.selected != 1 {
		t.Errorf("selected should be 1 after ctrl+j, got %d", p.selected)
	}

	p.Update(KeyEvent{Key: KeyRune, Rune: 'k', Ctrl: true})
	if p.selected != 0 {
		t.Errorf("selected should be 0 after ctrl+k, got %d", p.selected)
	}

	// Can't go above 0
	p.Update(KeyEvent{Key: KeyUp})
	if p.selected != 0 {
		t.Errorf("selected should stay 0 at top, got %d", p.selected)
	}
}

func TestModelPickerFiltering(t *testing.T) {
	p := newTestModelPicker()
	p.Open("gpt-4o")
	p.models = []string{"gpt-4o", "gpt-4o-mini", "claude-sonnet", "llama3"}
	p.loading = false
	p.refilter()

	// Type "gpt" - should match gpt models
	p.Update(KeyEvent{Key: KeyRune, Rune: 'g'})
	p.Update(KeyEvent{Key: KeyRune, Rune: 'p'})
	p.Update(KeyEvent{Key: KeyRune, Rune: 't'})

	if len(p.filtered) != 2 {
		t.Fatalf("expected 2 matches for 'gpt', got %d", len(p.filtered))
	}
}

func TestModelPickerBackspaceOnEmptyCloses(t *testing.T) {
	p := newTestModelPicker()
	p.Open("gpt-4o")

	dirty, consumed := p.Update(KeyEvent{Key: KeyBackspace})
	if !dirty || !consumed {
		t.Error("backspace on empty should be dirty and consumed")
	}
	if p.Active() {
		t.Error("picker should close on backspace with empty input")
	}
}

func TestModelPickerBackspaceDeletesChar(t *testing.T) {
	p := newTestModelPicker()
	p.Open("gpt-4o")
	p.models = []string{"gpt-4o"}
	p.loading = false
	p.refilter()

	p.Update(KeyEvent{Key: KeyRune, Rune: 'g'})
	p.Update(KeyEvent{Key: KeyRune, Rune: 'p'})

	if string(p.input) != "gp" {
		t.Fatalf("input should be 'gp', got %q", string(p.input))
	}

	p.Update(KeyEvent{Key: KeyBackspace})
	if string(p.input) != "g" {
		t.Errorf("input should be 'g' after backspace, got %q", string(p.input))
	}
}

func TestModelPickerRender(t *testing.T) {
	p := newTestModelPicker()
	p.Open("gpt-4o")
	p.models = []string{"gpt-4o", "claude-sonnet"}
	p.loading = false
	p.refilter()

	buf := NewScreenBuffer(80, 24)
	p.Render(buf, 80, 24)

	// Check that "model:" prompt is rendered
	found := false
	for x := 0; x < buf.Width; x++ {
		if buf.Cells[2][x].Rune == 'm' {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'model:' prompt to be rendered")
	}
}

func TestModelPickerRenderCurrentModelMarker(t *testing.T) {
	p := newTestModelPicker()
	p.Open("gpt-4o")
	p.models = []string{"gpt-4o", "claude-sonnet"}
	p.loading = false
	p.refilter()

	buf := NewScreenBuffer(80, 24)
	p.Render(buf, 80, 24)

	// The current model should have a '*' marker
	found := false
	for y := 0; y < buf.Height; y++ {
		for x := 0; x < buf.Width; x++ {
			if buf.Cells[y][x].Rune == '*' {
				found = true
				break
			}
		}
	}
	if !found {
		t.Error("expected '*' marker for current model")
	}
}

func TestModelPickerRenderLoading(t *testing.T) {
	p := newTestModelPicker()
	p.active = true
	p.loading = true

	buf := NewScreenBuffer(80, 24)
	p.Render(buf, 80, 24)

	// Should contain "loading" text
	found := false
	for y := 0; y < buf.Height; y++ {
		for x := 0; x < buf.Width-6; x++ {
			if buf.Cells[y][x].Rune == 'l' &&
				buf.Cells[y][x+1].Rune == 'o' &&
				buf.Cells[y][x+2].Rune == 'a' {
				found = true
				break
			}
		}
	}
	if !found {
		t.Error("expected 'loading' text in loading state")
	}
}

func TestModelPickerRenderNoResults(t *testing.T) {
	p := newTestModelPicker()
	p.Open("")
	p.models = []string{"gpt-4o"}
	p.loading = false
	p.refilter()

	// Type something that won't match
	p.Update(KeyEvent{Key: KeyRune, Rune: 'z'})
	p.Update(KeyEvent{Key: KeyRune, Rune: 'z'})
	p.Update(KeyEvent{Key: KeyRune, Rune: 'z'})

	if len(p.filtered) != 0 {
		t.Fatalf("expected 0 matches, got %d", len(p.filtered))
	}

	buf := NewScreenBuffer(80, 24)
	p.Render(buf, 80, 24)

	// Should render without panic
}

func TestModelPickerSelectedClamped(t *testing.T) {
	p := newTestModelPicker()
	p.Open("")
	p.models = []string{"a", "b", "c"}
	p.loading = false
	p.refilter()

	// Select last item
	p.Update(KeyEvent{Key: KeyDown})
	p.Update(KeyEvent{Key: KeyDown})
	if p.selected != 2 {
		t.Fatalf("selected should be 2, got %d", p.selected)
	}

	// Can't go past last
	p.Update(KeyEvent{Key: KeyDown})
	if p.selected != 2 {
		t.Errorf("selected should stay at 2, got %d", p.selected)
	}

	// Filter to fewer items — selected should be clamped
	p.Update(KeyEvent{Key: KeyRune, Rune: 'a'})
	if p.selected >= len(p.filtered) && len(p.filtered) > 0 {
		t.Errorf("selected %d should be < filtered count %d", p.selected, len(p.filtered))
	}
}

func TestModelPickerUnconsumedKeys(t *testing.T) {
	p := newTestModelPicker()
	p.Open("")

	// Keys that shouldn't be consumed
	dirty, consumed := p.Update(KeyEvent{Key: KeyRune, Rune: 'x', Ctrl: true})
	if consumed {
		t.Error("ctrl+x should not be consumed")
	}
	if dirty {
		t.Error("ctrl+x should not be dirty")
	}
}

func TestModelPickerRenderInactive(t *testing.T) {
	p := newTestModelPicker()
	buf := NewScreenBuffer(80, 24)

	// Should not render when inactive
	p.Render(buf, 80, 24)

	// Buffer should be all spaces
	for y := 0; y < buf.Height; y++ {
		for x := 0; x < buf.Width; x++ {
			if buf.Cells[y][x].Rune != ' ' {
				t.Fatalf("inactive picker should not render, found %q at (%d,%d)", buf.Cells[y][x].Rune, x, y)
			}
		}
	}
}
