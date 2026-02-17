package tui

import "sort"

// CommandPalette is an overlay component for finding and executing commands.
type CommandPalette struct {
	active   bool
	input    []rune
	filtered []scoredCommand
	selected int
	registry *CommandRegistry
}

type scoredCommand struct {
	Command
	score int
}

const maxVisible = 10

// NewCommandPalette creates a command palette backed by the given registry.
func NewCommandPalette(reg *CommandRegistry) *CommandPalette {
	return &CommandPalette{registry: reg}
}

// Active returns whether the palette is currently open.
func (p *CommandPalette) Active() bool {
	return p.active
}

// Open activates the palette and shows all commands.
func (p *CommandPalette) Open() {
	p.active = true
	p.input = nil
	p.selected = 0
	p.refilter()
}

// Close deactivates the palette.
func (p *CommandPalette) Close() {
	p.active = false
	p.input = nil
	p.filtered = nil
	p.selected = 0
}

// Update handles a key event. Returns (dirty, consumed).
func (p *CommandPalette) Update(e KeyEvent) (bool, bool) {
	switch {
	case e.Key == KeyEscape:
		p.Close()
		return true, true

	case e.Key == KeyEnter:
		if p.selected < len(p.filtered) {
			cmd := p.filtered[p.selected]
			p.Close()
			if cmd.Action != nil {
				cmd.Action()
			}
		} else {
			p.Close()
		}
		return true, true

	case e.Key == KeyUp || (e.Ctrl && (e.Rune == 'p' || e.Rune == 'k')):
		if p.selected > 0 {
			p.selected--
		}
		return true, true

	case e.Key == KeyDown || (e.Ctrl && (e.Rune == 'n' || e.Rune == 'j')):
		if p.selected < len(p.filtered)-1 {
			p.selected++
		}
		return true, true

	case e.Key == KeyBackspace:
		if len(p.input) > 0 {
			p.input = p.input[:len(p.input)-1]
			p.refilter()
			return true, true
		}
		// Backspace on empty input closes the palette
		p.Close()
		return true, true

	case e.Key == KeyRune && !e.Ctrl && !e.Alt:
		p.input = append(p.input, e.Rune)
		p.refilter()
		return true, true
	}

	return false, false
}

// refilter matches the current input against all commands and sorts by score.
func (p *CommandPalette) refilter() {
	query := string(p.input)
	p.filtered = nil

	for _, cmd := range p.registry.All() {
		bestScore := 0
		matched := false

		// Match against name
		if score, ok := FuzzyMatch(query, cmd.Name); ok && score > bestScore {
			bestScore = score
			matched = true
		}

		// Match against aliases
		for _, alias := range cmd.Aliases {
			if score, ok := FuzzyMatch(query, alias); ok && score > bestScore {
				bestScore = score
				matched = true
			}
		}

		// Match against description
		if score, ok := FuzzyMatch(query, cmd.Description); ok {
			// Description matches get a lower weight so name/alias matches rank higher
			descScore := score / 2
			if descScore > bestScore {
				bestScore = descScore
				matched = true
			}
		}

		if matched {
			p.filtered = append(p.filtered, scoredCommand{Command: cmd, score: bestScore})
		}
	}

	sort.Slice(p.filtered, func(i, j int) bool {
		return p.filtered[i].score > p.filtered[j].score
	})

	// Reset selection
	p.selected = 0
}

// Render draws the command palette overlay onto the screen buffer.
func (p *CommandPalette) Render(buf *ScreenBuffer, screenWidth, screenHeight int) {
	if !p.active {
		return
	}

	// Dimensions: 2/3 screen width, positioned near top
	boxWidth := screenWidth * 2 / 3
	if boxWidth < 30 {
		boxWidth = 30
	}
	if boxWidth > screenWidth-2 {
		boxWidth = screenWidth - 2
	}

	visibleCount := len(p.filtered)
	if visibleCount > maxVisible {
		visibleCount = maxVisible
	}

	// Box height: 1 (input line) + 1 (separator) + visibleCount (results)
	boxHeight := 2 + visibleCount
	if boxHeight < 3 {
		boxHeight = 3
	}

	boxX := (screenWidth - boxWidth) / 2
	boxY := 2 // near top of screen
	if boxY+boxHeight > screenHeight-2 {
		boxY = 1
	}

	// Styles
	bgStyle := Style{BG: NewColor(30, 30, 30)}
	promptStyle := Style{FG: NewColor(200, 200, 100), BG: NewColor(30, 30, 30), Bold: true}
	inputStyle := Style{FG: NewColor(220, 220, 220), BG: NewColor(30, 30, 30)}
	cursorStyle := Style{FG: NewColor(0, 0, 0), BG: NewColor(220, 220, 220)}
	sepStyle := Style{FG: NewColor(60, 60, 60), BG: NewColor(30, 30, 30)}
	nameStyle := Style{FG: NewColor(120, 170, 255), BG: NewColor(30, 30, 30), Bold: true}
	descStyle := Style{FG: NewColor(130, 130, 130), BG: NewColor(30, 30, 30)}
	selBGStyle := Style{BG: NewColor(50, 50, 70)}
	selNameStyle := Style{FG: NewColor(120, 170, 255), BG: NewColor(50, 50, 70), Bold: true}
	selDescStyle := Style{FG: NewColor(180, 180, 180), BG: NewColor(50, 50, 70)}

	// Fill background
	buf.Fill(Rect{X: boxX, Y: boxY, Width: boxWidth, Height: boxHeight}, ' ', bgStyle)

	// Draw input line: ": <input><cursor>"
	y := boxY
	buf.Set(boxX, y, ':', promptStyle)
	x := boxX + 1
	for _, r := range p.input {
		if x < boxX+boxWidth {
			buf.Set(x, y, r, inputStyle)
			x++
		}
	}
	// Block cursor
	if x < boxX+boxWidth {
		buf.Set(x, y, ' ', cursorStyle)
	}

	// Draw separator
	y++
	for sx := boxX; sx < boxX+boxWidth; sx++ {
		buf.Set(sx, y, '─', sepStyle)
	}

	// Draw filtered commands
	for i := 0; i < visibleCount; i++ {
		y++
		cmd := p.filtered[i]

		ns := nameStyle
		ds := descStyle
		if i == p.selected {
			// Fill selected row background
			for sx := boxX; sx < boxX+boxWidth; sx++ {
				buf.Set(sx, y, ' ', selBGStyle)
			}
			ns = selNameStyle
			ds = selDescStyle
		}

		// Name
		x = boxX + 1
		for _, r := range cmd.Name {
			if x < boxX+boxWidth-1 {
				buf.Set(x, y, r, ns)
				x++
			}
		}

		// Description (with spacing)
		if cmd.Description != "" {
			x += 2
			for _, r := range cmd.Description {
				if x < boxX+boxWidth-1 {
					buf.Set(x, y, r, ds)
					x++
				}
			}
		}
	}

	// If no results, show a "no matches" message
	if len(p.filtered) == 0 && len(p.input) > 0 {
		y++
		if y < boxY+boxHeight {
			noMatchStyle := Style{FG: NewColor(100, 100, 100), BG: NewColor(30, 30, 30), Italic: true}
			buf.WriteString(boxX+1, y, "no matching commands", noMatchStyle)
		}
	}
}
