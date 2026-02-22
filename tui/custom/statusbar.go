package custom

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/srogers/oc/tui/common"
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

func (sb *StatusBar) Focused() bool       { return false }
func (sb *StatusBar) SetFocused(f bool)   {}
func (sb *StatusBar) MinSize() (int, int) { return 20, 1 }

// Update handles tick events for spinner animation.
func (sb *StatusBar) Update(ev common.Event) bool {
	if _, ok := ev.(common.TickEvent); ok {
		sb.frame++
		return true
	}
	return false
}

// Render draws the status bar.
func (sb *StatusBar) Render(buf *common.ScreenBuffer, bounds common.Rect) {
	if bounds.Width < 1 || bounds.Height < 1 {
		return
	}

	bgStyle := common.Style{BG: common.NewColor(30, 30, 30), FG: common.NewColor(180, 180, 180)}

	// Fill background
	buf.Fill(bounds, ' ', bgStyle)

	x := bounds.X + 1

	if sb.getStatus == nil {
		buf.WriteString(x, bounds.Y, "oc", common.Style{BG: common.NewColor(30, 30, 30), FG: common.NewColor(100, 200, 100), Bold: true})
		return
	}

	info := sb.getStatus()

	// Status indicator
	var statusIcon string
	var statusStyle common.Style
	statusStyle.BG = common.NewColor(30, 30, 30)

	switch info.Status {
	case "busy":
		frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		statusIcon = frames[sb.frame%len(frames)]
		statusStyle.FG = common.NewColor(255, 200, 50)
	default:
		statusIcon = "●"
		statusStyle.FG = common.NewColor(100, 200, 100)
	}

	x += buf.WriteString(x, bounds.Y, statusIcon, statusStyle)
	x += buf.WriteString(x, bounds.Y, " ", bgStyle)

	// Model name
	if info.Model != "" {
		modelStyle := common.Style{BG: common.NewColor(30, 30, 30), FG: common.NewColor(140, 170, 255), Bold: true}
		x += buf.WriteString(x, bounds.Y, info.Model, modelStyle)
		x += buf.WriteString(x, bounds.Y, " ", bgStyle)
	}

	// Provider (with optional detail)
	if info.Provider != "" {
		provStyle := common.Style{BG: common.NewColor(30, 30, 30), FG: common.NewColor(120, 120, 120)}
		label := info.Provider
		if info.Detail != "" {
			label += "/" + info.Detail
		}
		x += buf.WriteString(x, bounds.Y, "("+label+")", provStyle)
		x += buf.WriteString(x, bounds.Y, " ", bgStyle)
	}

	// Working directory
	if info.WorkingDir != "" {
		dir := shortenHome(info.WorkingDir)
		dirStyle := common.Style{BG: common.NewColor(30, 30, 30), FG: common.NewColor(150, 150, 100)}
		x += buf.WriteString(x, bounds.Y, dir, dirStyle)
		x += buf.WriteString(x, bounds.Y, " ", bgStyle)
	}

	// Right-aligned: context, tokens, and cost
	right := ""
	if info.ContextTokens > 0 {
		right = fmt.Sprintf("ctx %s", formatTokens(info.ContextTokens))
	}
	if info.Tokens > 0 {
		if right != "" {
			right += " | "
		}
		right += fmt.Sprintf("%s tokens", formatTokens(info.Tokens))
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

// formatTokens formats a token count with K/M suffixes for readability.
func formatTokens(n int) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
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
