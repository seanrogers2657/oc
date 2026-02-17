package tui

import "testing"

func newTestRegistry() *CommandRegistry {
	reg := NewCommandRegistry()
	reg.Register(Command{Name: "clear", Description: "Clear conversation history", Aliases: []string{"cls", "reset"}})
	reg.Register(Command{Name: "exit", Description: "Exit the application", Aliases: []string{"quit", "q"}})
	reg.Register(Command{Name: "help", Description: "Show available commands", Aliases: []string{"?", "commands"}})
	return reg
}

func TestCommandPaletteOpenClose(t *testing.T) {
	p := NewCommandPalette(newTestRegistry())

	if p.Active() {
		t.Fatal("palette should start inactive")
	}

	p.Open()
	if !p.Active() {
		t.Fatal("palette should be active after Open()")
	}
	if len(p.filtered) != 3 {
		t.Errorf("expected 3 commands on open, got %d", len(p.filtered))
	}

	p.Close()
	if p.Active() {
		t.Fatal("palette should be inactive after Close()")
	}
}

func TestCommandPaletteEscapeCloses(t *testing.T) {
	p := NewCommandPalette(newTestRegistry())
	p.Open()

	dirty, consumed := p.Update(KeyEvent{Key: KeyEscape})
	if !dirty || !consumed {
		t.Error("escape should be dirty and consumed")
	}
	if p.Active() {
		t.Error("palette should be closed after escape")
	}
}

func TestCommandPaletteEnterExecutes(t *testing.T) {
	reg := NewCommandRegistry()
	executed := false
	reg.Register(Command{
		Name:   "test",
		Action: func() { executed = true },
	})

	p := NewCommandPalette(reg)
	p.Open()

	dirty, consumed := p.Update(KeyEvent{Key: KeyEnter})
	if !dirty || !consumed {
		t.Error("enter should be dirty and consumed")
	}
	if !executed {
		t.Error("command action should have been executed")
	}
	if p.Active() {
		t.Error("palette should close after executing")
	}
}

func TestCommandPaletteArrowNavigation(t *testing.T) {
	p := NewCommandPalette(newTestRegistry())
	p.Open()

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

func TestCommandPaletteFiltering(t *testing.T) {
	p := NewCommandPalette(newTestRegistry())
	p.Open()

	// Type "cl" - should match "clear" and possibly others
	p.Update(KeyEvent{Key: KeyRune, Rune: 'c'})
	p.Update(KeyEvent{Key: KeyRune, Rune: 'l'})

	if len(p.filtered) == 0 {
		t.Fatal("expected at least one match for 'cl'")
	}
	if p.filtered[0].Name != "clear" {
		t.Errorf("expected 'clear' as top result, got %q", p.filtered[0].Name)
	}
}

func TestCommandPaletteBackspaceOnEmptyCloses(t *testing.T) {
	p := NewCommandPalette(newTestRegistry())
	p.Open()

	dirty, consumed := p.Update(KeyEvent{Key: KeyBackspace})
	if !dirty || !consumed {
		t.Error("backspace on empty should be dirty and consumed")
	}
	if p.Active() {
		t.Error("palette should close on backspace with empty input")
	}
}

func TestCommandPaletteBackspaceDeletesChar(t *testing.T) {
	p := NewCommandPalette(newTestRegistry())
	p.Open()

	p.Update(KeyEvent{Key: KeyRune, Rune: 'c'})
	p.Update(KeyEvent{Key: KeyRune, Rune: 'l'})

	if string(p.input) != "cl" {
		t.Fatalf("input should be 'cl', got %q", string(p.input))
	}

	p.Update(KeyEvent{Key: KeyBackspace})
	if string(p.input) != "c" {
		t.Errorf("input should be 'c' after backspace, got %q", string(p.input))
	}
}

func TestCommandPaletteRender(t *testing.T) {
	p := NewCommandPalette(newTestRegistry())
	p.Open()

	buf := NewScreenBuffer(80, 24)
	p.Render(buf, 80, 24)

	// Check that the ':' prompt is rendered somewhere in the buffer
	found := false
	for x := 0; x < buf.Width; x++ {
		if buf.Cells[2][x].Rune == ':' {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected ':' prompt to be rendered on the palette")
	}
}

func TestCommandPaletteAliasMatch(t *testing.T) {
	p := NewCommandPalette(newTestRegistry())
	p.Open()

	// Type "q" - should match "exit" via its "q" alias
	p.Update(KeyEvent{Key: KeyRune, Rune: 'q'})

	found := false
	for _, cmd := range p.filtered {
		if cmd.Name == "exit" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'exit' to match via 'q' alias")
	}
}
