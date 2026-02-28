package common

import (
	"sort"

	"github.com/seanrogers2657/oc/domain"
)

// ModelPicker is an overlay component for searching and selecting AI models.
type ModelPicker struct {
	active       bool
	loading      bool
	models       []string
	input        []rune
	filtered     []scoredModel
	selected     int
	currentModel string
	err          error

	fetchModels func() ([]string, error)
	onSelect    func(model string)
}

type scoredModel struct {
	name  string
	score int
}

const modelPickerMaxVisible = 12

// NewModelPicker creates a model picker with the given fetch and select callbacks.
func NewModelPicker(fetchModels func() ([]string, error), onSelect func(string)) *ModelPicker {
	return &ModelPicker{
		fetchModels: fetchModels,
		onSelect:    onSelect,
	}
}

// Active returns whether the picker is currently open.
func (p *ModelPicker) Active() bool {
	return p.active
}

// Open activates the picker and starts fetching models.
func (p *ModelPicker) Open(currentModel string) {
	p.active = true
	p.loading = true
	p.input = nil
	p.selected = 0
	p.filtered = nil
	p.models = nil
	p.err = nil
	p.currentModel = currentModel

	go func() {
		models, err := p.fetchModels()
		// Store results; next tick/key event will pick them up.
		p.models = models
		p.err = err
		p.loading = false
		p.refilter()
	}()
}

// Close deactivates the picker.
func (p *ModelPicker) Close() {
	p.active = false
	p.input = nil
	p.filtered = nil
	p.models = nil
	p.selected = 0
	p.err = nil
}

// Update handles a key event. Returns (dirty, consumed).
func (p *ModelPicker) Update(e domain.KeyEvent) (bool, bool) {
	if action, ok := resolveOverlayAction(e); ok {
		return p.HandleAction(action)
	}
	if e.Key == domain.KeyRune && e.Mod == 0 {
		return p.InsertRune(e.Rune)
	}
	return false, false
}

// HandleAction responds to a resolved action. Returns (needsRedraw, consumedEvent).
func (p *ModelPicker) HandleAction(action domain.Action) (bool, bool) {
	switch action {
	case domain.ActionOverlayClose:
		p.Close()
		return true, true
	case domain.ActionOverlayConfirm:
		if p.selected < len(p.filtered) {
			model := p.filtered[p.selected].name
			p.Close()
			if p.onSelect != nil {
				p.onSelect(model)
			}
		} else {
			p.Close()
		}
		return true, true
	case domain.ActionOverlayPrev:
		if p.selected > 0 {
			p.selected--
		}
		return true, true
	case domain.ActionOverlayNext:
		if p.selected < len(p.filtered)-1 {
			p.selected++
		}
		return true, true
	case domain.ActionOverlayBackspace:
		if len(p.input) > 0 {
			p.input = p.input[:len(p.input)-1]
			p.refilter()
			return true, true
		}
		p.Close()
		return true, true
	}
	return false, false
}

// InsertRune handles a typed character in the picker filter. Returns (needsRedraw, consumedEvent).
func (p *ModelPicker) InsertRune(r rune) (bool, bool) {
	p.input = append(p.input, r)
	p.refilter()
	return true, true
}

// refilter matches the current input against all models and sorts by score.
func (p *ModelPicker) refilter() {
	query := string(p.input)
	p.filtered = nil

	for _, name := range p.models {
		if query == "" {
			p.filtered = append(p.filtered, scoredModel{name: name, score: 1})
			continue
		}
		if score, ok := FuzzyMatch(query, name); ok {
			p.filtered = append(p.filtered, scoredModel{name: name, score: score})
		}
	}

	sort.Slice(p.filtered, func(i, j int) bool {
		return p.filtered[i].score > p.filtered[j].score
	})

	if p.selected >= len(p.filtered) {
		p.selected = 0
	}
}

// Render draws the model picker overlay onto the screen buffer.
func (p *ModelPicker) Render(buf *ScreenBuffer, screenWidth, screenHeight int) {
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
	if visibleCount > modelPickerMaxVisible {
		visibleCount = modelPickerMaxVisible
	}

	// Box height: 1 (input line) + 1 (separator) + body rows
	bodyRows := visibleCount
	if p.loading {
		bodyRows = 1
	} else if p.err != nil {
		bodyRows = 1
	} else if len(p.filtered) == 0 {
		bodyRows = 1
	}
	boxHeight := 2 + bodyRows
	if boxHeight < 3 {
		boxHeight = 3
	}

	boxX := (screenWidth - boxWidth) / 2
	boxY := 2
	if boxY+boxHeight > screenHeight-2 {
		boxY = 1
	}

	// Styles (matching command palette)
	bgStyle := Style{BG: NewColor(30, 30, 30)}
	promptStyle := Style{FG: NewColor(200, 200, 100), BG: NewColor(30, 30, 30), Bold: true}
	inputStyle := Style{FG: NewColor(220, 220, 220), BG: NewColor(30, 30, 30)}
	cursorStyle := Style{FG: NewColor(0, 0, 0), BG: NewColor(220, 220, 220)}
	sepStyle := Style{FG: NewColor(60, 60, 60), BG: NewColor(30, 30, 30)}
	nameStyle := Style{FG: NewColor(120, 170, 255), BG: NewColor(30, 30, 30)}
	currentStyle := Style{FG: NewColor(100, 200, 100), BG: NewColor(30, 30, 30)}
	selBGStyle := Style{BG: NewColor(50, 50, 70)}
	selNameStyle := Style{FG: NewColor(120, 170, 255), BG: NewColor(50, 50, 70), Bold: true}
	selCurrentStyle := Style{FG: NewColor(100, 200, 100), BG: NewColor(50, 50, 70), Bold: true}
	dimStyle := Style{FG: NewColor(100, 100, 100), BG: NewColor(30, 30, 30), Italic: true}

	// Fill background
	buf.Fill(Rect{X: boxX, Y: boxY, Width: boxWidth, Height: boxHeight}, ' ', bgStyle)

	// Draw input line: "model: <input><cursor>"
	y := boxY
	prompt := "model: "
	x := boxX
	for _, r := range prompt {
		if x < boxX+boxWidth {
			buf.Set(x, y, r, promptStyle)
			x++
		}
	}
	for _, r := range p.input {
		if x < boxX+boxWidth {
			buf.Set(x, y, r, inputStyle)
			x++
		}
	}
	if x < boxX+boxWidth {
		buf.Set(x, y, ' ', cursorStyle)
	}

	// Draw separator
	y++
	for sx := boxX; sx < boxX+boxWidth; sx++ {
		buf.Set(sx, y, '─', sepStyle)
	}

	// Loading state
	if p.loading {
		y++
		buf.WriteString(boxX+1, y, "loading models...", dimStyle)
		return
	}

	// Error state
	if p.err != nil {
		y++
		errStyle := Style{FG: NewColor(255, 100, 100), BG: NewColor(30, 30, 30)}
		msg := p.err.Error()
		if len(msg) > boxWidth-2 {
			msg = msg[:boxWidth-2]
		}
		buf.WriteString(boxX+1, y, msg, errStyle)
		return
	}

	// Draw filtered models
	for i := 0; i < visibleCount; i++ {
		y++
		model := p.filtered[i]
		isCurrent := model.name == p.currentModel

		ns := nameStyle
		cs := currentStyle
		if i == p.selected {
			for sx := boxX; sx < boxX+boxWidth; sx++ {
				buf.Set(sx, y, ' ', selBGStyle)
			}
			ns = selNameStyle
			cs = selCurrentStyle
		}

		x = boxX + 1

		// Show checkmark for current model
		if isCurrent {
			buf.Set(x, y, '*', cs)
			x += 2
		} else {
			x += 2 // keep alignment
		}

		for _, r := range model.name {
			if x < boxX+boxWidth-1 {
				if isCurrent {
					buf.Set(x, y, r, cs)
				} else {
					buf.Set(x, y, r, ns)
				}
				x++
			}
		}
	}

	// No results
	if len(p.filtered) == 0 && len(p.input) > 0 {
		y++
		if y < boxY+boxHeight {
			buf.WriteString(boxX+1, y, "no matching models", dimStyle)
		}
	}
}
