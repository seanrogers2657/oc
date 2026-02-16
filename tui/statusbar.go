package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// StatusBar displays model info, token count, and session status.
type StatusBar struct {
	getStatus StatusProvider
	frame     int // animation frame for spinner
}

// NewStatusBar creates a new status bar.
func NewStatusBar(getStatus StatusProvider) *StatusBar {
	return &StatusBar{getStatus: getStatus}
}

func (sb *StatusBar) Focused() bool     { return false }
func (sb *StatusBar) SetFocused(f bool) {}
func (sb *StatusBar) MinSize() (int, int) { return 20, 1 }

// Update handles tick events for spinner animation.
func (sb *StatusBar) Update(ev Event) bool {
	if _, ok := ev.(TickEvent); ok {
		sb.frame++
		return true
	}
	return false
}

// Render draws the status bar.
func (sb *StatusBar) Render(buf *ScreenBuffer, bounds Rect) {
	if bounds.Width < 1 || bounds.Height < 1 {
		return
	}

	bgStyle := Style{BG: NewColor(30, 30, 30), FG: NewColor(180, 180, 180)}

	// Fill background
	buf.Fill(bounds, ' ', bgStyle)

	x := bounds.X + 1

	if sb.getStatus == nil {
		buf.WriteString(x, bounds.Y, "oc", Style{BG: NewColor(30, 30, 30), FG: NewColor(100, 200, 100), Bold: true})
		return
	}

	info := sb.getStatus()

	// Status indicator
	var statusIcon string
	var statusStyle Style
	statusStyle.BG = NewColor(30, 30, 30)

	switch info.Status {
	case "busy":
		frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		statusIcon = frames[sb.frame%len(frames)]
		statusStyle.FG = NewColor(255, 200, 50)
	default:
		statusIcon = "●"
		statusStyle.FG = NewColor(100, 200, 100)
	}

	x += buf.WriteString(x, bounds.Y, statusIcon, statusStyle)
	x += buf.WriteString(x, bounds.Y, " ", bgStyle)

	// Model name
	if info.Model != "" {
		modelStyle := Style{BG: NewColor(30, 30, 30), FG: NewColor(140, 170, 255), Bold: true}
		x += buf.WriteString(x, bounds.Y, info.Model, modelStyle)
		x += buf.WriteString(x, bounds.Y, " ", bgStyle)
	}

	// Provider
	if info.Provider != "" {
		provStyle := Style{BG: NewColor(30, 30, 30), FG: NewColor(120, 120, 120)}
		x += buf.WriteString(x, bounds.Y, "("+info.Provider+")", provStyle)
		x += buf.WriteString(x, bounds.Y, " ", bgStyle)
	}

	// Working directory
	if info.WorkingDir != "" {
		dir := shortenHome(info.WorkingDir)
		dirStyle := Style{BG: NewColor(30, 30, 30), FG: NewColor(150, 150, 100)}
		x += buf.WriteString(x, bounds.Y, dir, dirStyle)
		x += buf.WriteString(x, bounds.Y, " ", bgStyle)
	}

	// Right-aligned: tokens and cost
	right := ""
	if info.Tokens > 0 {
		right = fmt.Sprintf("%d tokens", info.Tokens)
	}
	if info.Cost != "" {
		if right != "" {
			right += " | "
		}
		right += info.Cost
	}

	if right != "" {
		rx := bounds.X + bounds.Width - utf8.RuneCountInString(right) - 1
		if rx > x {
			buf.WriteString(rx, bounds.Y, right, bgStyle)
		}
	}
}

// shortenHome replaces the user's home directory prefix with "~".
func shortenHome(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Base(path)
	}
	if path == home {
		return "~"
	}
	if strings.HasPrefix(path, home+string(filepath.Separator)) {
		return "~" + path[len(home):]
	}
	return path
}
