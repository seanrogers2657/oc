package tui

import (
	"fmt"
	"strings"

	"github.com/srogers/oc/session"
)

// renderToolCall renders a tool call part within the message list.
// Returns the number of screen rows consumed.
func renderToolCall(buf *ScreenBuffer, x, y, maxWidth int, tc session.ToolCallPart) int {
	if maxWidth <= 0 {
		return 0
	}

	row := 0

	// Status indicator + tool name
	var statusIcon string
	var statusStyle Style
	switch tc.Status {
	case session.ToolPending:
		statusIcon = "○"
		statusStyle = Style{FG: NewColor(120, 120, 120)}
	case session.ToolRunning:
		statusIcon = "◌"
		statusStyle = Style{FG: NewColor(255, 200, 50)}
	case session.ToolCompleted:
		statusIcon = "●"
		statusStyle = Style{FG: NewColor(100, 200, 100)}
	case session.ToolError:
		statusIcon = "✗"
		statusStyle = Style{FG: NewColor(255, 80, 80)}
	}

	toolStyle := Style{FG: NewColor(180, 140, 255), Bold: true}
	dimStyle := Style{FG: NewColor(100, 100, 100)}

	cx := x
	cx += buf.WriteString(cx, y+row, statusIcon+" ", statusStyle)
	cx += buf.WriteString(cx, y+row, tc.Tool, toolStyle)

	// Duration
	if !tc.End.IsZero() && !tc.Start.IsZero() {
		dur := tc.End.Sub(tc.Start)
		durStr := fmt.Sprintf(" (%s)", dur.Round(1e6)) // round to ms
		buf.WriteString(cx, y+row, durStr, dimStyle)
	}
	row++

	// Output (truncated)
	if tc.Output != "" {
		output := tc.Output
		const maxOutputLines = 10
		const maxOutputLen = 2000

		if len(output) > maxOutputLen {
			output = output[:maxOutputLen] + "..."
		}

		lines := strings.Split(output, "\n")
		if len(lines) > maxOutputLines {
			lines = lines[:maxOutputLines]
			lines = append(lines, fmt.Sprintf("... (%d more lines)", len(strings.Split(tc.Output, "\n"))-maxOutputLines))
		}

		outputStyle := Style{FG: NewColor(160, 160, 160)}
		for _, line := range lines {
			if row > maxOutputLines+2 {
				break
			}
			// Indent output
			indent := x + 2
			// Truncate line to fit
			if len(line) > maxWidth-2 {
				line = line[:maxWidth-5] + "..."
			}
			buf.WriteString(indent, y+row, line, outputStyle)
			row++
		}
	}

	// Error
	if tc.Error != "" {
		errStyle := Style{FG: NewColor(255, 80, 80)}
		errLine := "Error: " + tc.Error
		if len(errLine) > maxWidth-2 {
			errLine = errLine[:maxWidth-5] + "..."
		}
		buf.WriteString(x+2, y+row, errLine, errStyle)
		row++
	}

	return row
}
