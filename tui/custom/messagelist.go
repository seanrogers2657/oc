package custom

import (
	"strings"
	"unicode/utf8"

	"github.com/srogers/oc/event"
	"github.com/srogers/oc/markdown"
	"github.com/srogers/oc/provider"
	"github.com/srogers/oc/session"
	"github.com/srogers/oc/tool/diff"
	"github.com/srogers/oc/tui/common"
)

// MessageList is a scrollable chat history component.
type MessageList struct {
	getMessages MessageListProvider
	scroll      common.ScrollState
	autoScroll  bool // auto-scroll to bottom on new content
	dirty       bool // needs re-render
}

// NewMessageList creates a new message list.
func NewMessageList(getMessages MessageListProvider) *MessageList {
	return &MessageList{
		getMessages: getMessages,
		autoScroll:  true,
	}
}

// ScrollInfo returns the current scroll offset and whether auto-scroll is active.
func (ml *MessageList) ScrollInfo() (offset int, autoScroll bool) {
	return ml.scroll.Offset, ml.autoScroll
}

func (ml *MessageList) Focused() bool       { return false }
func (ml *MessageList) SetFocused(f bool)   {}
func (ml *MessageList) MinSize() (int, int) { return 20, 3 }

// ScrollUp scrolls the message list up by n lines.
func (ml *MessageList) ScrollUp(n int) {
	ml.scroll.ScrollUp(n)
	ml.autoScroll = false
}

// ScrollDown scrolls the message list down by n lines.
func (ml *MessageList) ScrollDown(n int) {
	ml.scroll.ScrollDown(n)
	if ml.scroll.AtBottom() {
		ml.autoScroll = true
	}
}

// Update handles events. Returns true if redraw needed.
func (ml *MessageList) Update(ev common.Event) bool {
	switch e := ev.(type) {
	case common.KeyEvent:
		// Scroll with PgUp/PgDown
		switch e.Key {
		case common.KeyPgUp:
			ml.scroll.ScrollUp(ml.scroll.ViewportHeight / 2)
			ml.autoScroll = false
			return true
		case common.KeyPgDown:
			ml.scroll.ScrollDown(ml.scroll.ViewportHeight / 2)
			if ml.scroll.AtBottom() {
				ml.autoScroll = true
			}
			return true
		}

	case common.CustomEvent:
		switch e.Topic {
		case string(event.TopicPartDelta), string(event.TopicMsgDone), string(event.TopicToolStart), string(event.TopicToolDone), string(event.TopicError):
			ml.dirty = true
			return true
		}

	case common.TickEvent:
		// Refresh during streaming
		if ml.dirty {
			ml.dirty = false
			return true
		}
	}

	return false
}

// Render draws the message list into the buffer.
func (ml *MessageList) Render(buf *common.ScreenBuffer, bounds common.Rect) {
	if bounds.Width < 1 || bounds.Height < 1 || ml.getMessages == nil {
		return
	}

	// Pre-render all messages into lines
	msgs := ml.getMessages()
	var allLines []renderedLine
	width := bounds.Width - 2 // margin

	for _, msg := range msgs {
		lines := ml.renderMessage(msg, width)
		allLines = append(allLines, lines...)
		// Blank line between messages
		allLines = append(allLines, renderedLine{})
	}

	// Update scroll state
	ml.scroll.ContentHeight = len(allLines)
	ml.scroll.ViewportHeight = bounds.Height
	if ml.autoScroll {
		ml.scroll.ScrollToBottom()
	}

	// Draw visible lines
	startLine := ml.scroll.Offset
	for i := 0; i < bounds.Height; i++ {
		lineIdx := startLine + i
		if lineIdx >= len(allLines) {
			break
		}
		rl := allLines[lineIdx]
		y := bounds.Y + i
		x := bounds.X + 1 // left margin

		for _, span := range rl.spans {
			x += buf.WriteString(x, y, span.text, span.style)
		}
	}
}

// renderedLine is one pre-rendered screen line with styled spans.
type renderedLine struct {
	spans []styledSpan
}

type styledSpan struct {
	text  string
	style common.Style
}

// renderMessage converts a session.Message into renderedLines.
func (ml *MessageList) renderMessage(msg session.Message, maxWidth int) []renderedLine {
	var lines []renderedLine

	isUser := msg.Role == provider.RoleUser
	isTool := msg.Role == provider.RoleTool

	// Prefixing constants
	const userPrefix = "> "
	const assistantIndent = "  "

	// Determine content width (account for prefix/indent)
	contentWidth := maxWidth
	if isUser {
		contentWidth = maxWidth - len(userPrefix)
	} else if !isTool {
		contentWidth = maxWidth - len(assistantIndent)
	}
	if contentWidth < 10 {
		contentWidth = 10
	}

	// Render each part
	for _, part := range msg.Parts {
		switch p := part.(type) {
		case session.TextPart:
			if p.Text == "" {
				continue
			}
			mdLines := markdown.Render(p.Text)
			for _, mdLine := range mdLines {
				lines = append(lines, mdLineToRendered(mdLine, contentWidth)...)
			}

		case session.ReasoningPart:
			if p.Text == "" {
				continue
			}
			thinkStyle := common.Style{FG: common.NewColor(120, 120, 120), Italic: true}
			lines = append(lines, renderedLine{spans: []styledSpan{
				{text: "thinking: ", style: common.Style{FG: common.NewColor(100, 100, 100), Italic: true, Bold: true}},
			}})
			wrapped := common.WrapText(p.Text, contentWidth)
			for _, wl := range wrapped {
				lines = append(lines, renderedLine{spans: []styledSpan{
					{text: wl, style: thinkStyle},
				}})
			}

		case session.ToolCallPart:
			if msg.Role == provider.RoleTool {
				continue
			}
			lines = append(lines, renderToolCallLines(p, contentWidth)...)
		}
	}

	// Render error as agent-style content
	if msg.Error != nil {
		errStyle := common.Style{FG: common.NewColor(255, 80, 80)}
		errLabel := common.Style{FG: common.NewColor(255, 80, 80), Bold: true}
		wrapped := common.WrapText(msg.Error.Error(), contentWidth-len("Error: "))
		for i, wl := range wrapped {
			if i == 0 {
				lines = append(lines, renderedLine{spans: []styledSpan{
					{text: "Error: ", style: errLabel},
					{text: wl, style: errStyle},
				}})
			} else {
				lines = append(lines, renderedLine{spans: []styledSpan{
					{text: wl, style: errStyle},
				}})
			}
		}
	}

	// Apply prefix/indent to all content lines
	if isUser {
		userStyle := common.Style{FG: common.NewColor(180, 180, 180)}
		for i, line := range lines {
			prefixed := []styledSpan{{text: userPrefix, style: userStyle}}
			// Apply light gray to all spans
			for _, s := range line.spans {
				s.style.FG = common.NewColor(180, 180, 180)
				prefixed = append(prefixed, s)
			}
			lines[i] = renderedLine{spans: prefixed}
		}
	} else if !isTool {
		for i, line := range lines {
			if len(line.spans) == 0 {
				continue
			}
			indented := []styledSpan{{text: assistantIndent, style: common.Style{}}}
			indented = append(indented, line.spans...)
			lines[i] = renderedLine{spans: indented}
		}
	}

	return lines
}

// mdLineToRendered converts a markdown.Line to renderedLines, wrapping if needed.
func mdLineToRendered(line markdown.Line, maxWidth int) []renderedLine {
	if len(line.Spans) == 0 {
		return []renderedLine{{}}
	}

	indent := line.Indent
	if indent >= maxWidth {
		indent = maxWidth - 1
	}
	if indent < 0 {
		indent = 0
	}

	makeIndentSpan := func() styledSpan {
		return styledSpan{text: strings.Repeat(" ", indent), style: common.Style{}}
	}

	var result []renderedLine
	var currentRow []styledSpan
	cx := 0 // cursor position in current row (runes)

	// Add indent for first line
	if indent > 0 {
		currentRow = append(currentRow, makeIndentSpan())
		cx = indent
	}

	flushRow := func() {
		result = append(result, renderedLine{spans: currentRow})
		currentRow = nil
		cx = 0
		// Add indent for continuation line
		if indent > 0 {
			currentRow = append(currentRow, makeIndentSpan())
			cx = indent
		}
	}

	for _, span := range line.Spans {
		style := mdStyle(span.Kind)

		// HRule: produce a full-width line
		if span.Kind == markdown.KindHRule {
			hrText := strings.Repeat("─", maxWidth)
			return []renderedLine{{spans: []styledSpan{{text: hrText, style: style}}}}
		}

		remaining := span.Text
		for len(remaining) > 0 {
			remainLen := utf8.RuneCountInString(remaining)
			avail := maxWidth - cx

			// Text fits on current line
			if remainLen <= avail {
				currentRow = append(currentRow, styledSpan{text: remaining, style: style})
				cx += remainLen
				remaining = ""
				break
			}

			// Text doesn't fit — try word-boundary break
			chunk := findBreakPoint(remaining, avail)
			if chunk != "" {
				currentRow = append(currentRow, styledSpan{text: chunk, style: style})
				remaining = remaining[len(chunk):]
				// Trim leading spaces from remainder after a space break
				remaining = strings.TrimLeft(remaining, " ")
				flushRow()
				continue
			}

			// No word break found
			if cx > indent {
				// Mid-line: flush current row and retry with full line width
				flushRow()
				continue
			}

			// At start of line: force character break
			chunk = truncateRunes(remaining, avail)
			currentRow = append(currentRow, styledSpan{text: chunk, style: style})
			remaining = remaining[len(chunk):]
			flushRow()
		}
	}

	// Flush final row
	if len(currentRow) > 0 {
		result = append(result, renderedLine{spans: currentRow})
	}

	if len(result) == 0 {
		return []renderedLine{{}}
	}
	return result
}

// renderToolCallLines creates rendered lines for a tool call part.
func renderToolCallLines(tc session.ToolCallPart, maxWidth int) []renderedLine {
	var lines []renderedLine

	// Status + tool name
	var statusIcon string
	var statusStyle common.Style
	switch tc.Status {
	case session.ToolPending:
		statusIcon = "○"
		statusStyle = common.Style{FG: common.NewColor(120, 120, 120)}
	case session.ToolRunning:
		statusIcon = "◌"
		statusStyle = common.Style{FG: common.NewColor(255, 200, 50)}
	case session.ToolCompleted:
		statusIcon = "●"
		statusStyle = common.Style{FG: common.NewColor(100, 200, 100)}
	case session.ToolError:
		statusIcon = "✗"
		statusStyle = common.Style{FG: common.NewColor(255, 80, 80)}
	}

	toolStyle := common.Style{FG: common.NewColor(180, 140, 255), Bold: true}
	label := tc.Tool
	if tc.Title != "" {
		label = tc.Title
	}
	headerSpans := []styledSpan{
		{text: statusIcon + " ", style: statusStyle},
		{text: label, style: toolStyle},
	}

	if !tc.End.IsZero() && !tc.Start.IsZero() {
		dur := tc.End.Sub(tc.Start)
		durStr := " (" + dur.Round(1e6).String() + ")"
		headerSpans = append(headerSpans, styledSpan{text: durStr, style: common.Style{FG: common.NewColor(100, 100, 100)}})
	}
	lines = append(lines, renderedLine{spans: headerSpans})

	// Show diff if available (for file-modifying tools)
	if tc.Diff != nil && (tc.Diff.Added > 0 || tc.Diff.Removed > 0) {
		diffLines := diff.FormatDiffForTUI(tc.Diff, maxWidth-2)
		const maxDiffLines = 15
		for i, dl := range diffLines {
			if i >= maxDiffLines {
				remaining := len(diffLines) - maxDiffLines
				lines = append(lines, renderedLine{spans: []styledSpan{
					{text: "  ...", style: common.Style{FG: common.NewColor(100, 100, 100)}},
					{text: " (" + itoa(remaining) + " more lines)", style: common.Style{FG: common.NewColor(100, 100, 100)}},
				}})
				break
			}

			// Convert diff styling to TUI styling.
			contentStyle := diffLineToStyle(dl)
			prefixStyle := common.Style{FG: contentStyle.FG}
			var spans []styledSpan
			spans = append(spans, styledSpan{text: "  " + dl.Prefix, style: prefixStyle})
			spans = append(spans, styledSpan{text: dl.Text, style: contentStyle})
			lines = append(lines, renderedLine{spans: spans})
		}
	} else if tc.Output != "" {
		// Fall back to showing regular output if no diff available
		outputStyle := common.Style{FG: common.NewColor(160, 160, 160)}
		outputLines := common.WrapText(tc.Output, maxWidth-2)
		const maxOutputLines = 10
		for i, ol := range outputLines {
			if i >= maxOutputLines {
				remaining := len(outputLines) - maxOutputLines
				lines = append(lines, renderedLine{spans: []styledSpan{
					{text: "  ...", style: common.Style{FG: common.NewColor(100, 100, 100)}},
					{text: " (" + itoa(remaining) + " more lines)", style: common.Style{FG: common.NewColor(100, 100, 100)}},
				}})
				break
			}
			lines = append(lines, renderedLine{spans: []styledSpan{
				{text: "  " + ol, style: outputStyle},
			}})
		}
	}

	// Error
	if tc.Error != "" {
		errStyle := common.Style{FG: common.NewColor(255, 80, 80)}
		lines = append(lines, renderedLine{spans: []styledSpan{
			{text: "  Error: " + tc.Error, style: errStyle},
		}})
	}

	return lines
}

// truncateRunes returns the first n runes of s.
func truncateRunes(s string, n int) string {
	i := 0
	for idx := range s {
		if i >= n {
			return s[:idx]
		}
		i++
	}
	return s
}

// findBreakPoint finds the best word-boundary break point in text at or before
// maxRunes. Returns the prefix up to the break (space excluded, hyphen included).
// Returns "" if no good break point exists.
func findBreakPoint(text string, maxRunes int) string {
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return text
	}
	for i := maxRunes; i > 0; i-- {
		if runes[i] == ' ' {
			return string(runes[:i])
		}
		if runes[i] == '-' {
			return string(runes[:i+1])
		}
	}
	return ""
}

// itoa converts int to string without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	digits := make([]byte, 0, 10)
	for n > 0 {
		digits = append(digits, byte('0'+n%10))
		n /= 10
	}
	if neg {
		digits = append(digits, '-')
	}
	// Reverse
	for i, j := 0, len(digits)-1; i < j; i, j = i+1, j-1 {
		digits[i], digits[j] = digits[j], digits[i]
	}
	return string(digits)
}

// mdStyle maps a markdown SpanKind to a TUI Style.
func mdStyle(kind markdown.SpanKind) common.Style {
	switch kind {
	case markdown.KindBold:
		return common.Style{FG: common.NewColor(220, 220, 220), Bold: true}
	case markdown.KindItalic:
		return common.Style{FG: common.NewColor(200, 200, 200), Italic: true}
	case markdown.KindBoldItalic:
		return common.Style{FG: common.NewColor(220, 220, 220), Bold: true, Italic: true}
	case markdown.KindCode:
		return common.Style{FG: common.NewColor(230, 180, 80), BG: common.NewColor(40, 40, 40)}
	case markdown.KindCodeBlock:
		return common.Style{FG: common.NewColor(200, 200, 200), BG: common.NewColor(30, 30, 30)}
	case markdown.KindCodeLang:
		return common.Style{FG: common.NewColor(120, 120, 120), BG: common.NewColor(30, 30, 30), Italic: true}
	case markdown.KindHeader:
		return common.Style{FG: common.NewColor(140, 170, 255), Bold: true}
	case markdown.KindLink:
		return common.Style{FG: common.NewColor(100, 180, 255), Underline: true}
	case markdown.KindLinkURL:
		return common.Style{FG: common.NewColor(100, 100, 100)}
	case markdown.KindBlockquote:
		return common.Style{FG: common.NewColor(160, 160, 160), Italic: true}
	case markdown.KindListBullet:
		return common.Style{FG: common.NewColor(100, 200, 100)}
	case markdown.KindHRule:
		return common.Style{FG: common.NewColor(80, 80, 80)}
	default:
		return common.Style{FG: common.NewColor(220, 220, 220)}
	}
}

// diffLineToStyle converts a diff.StyledLine to a TUI Style
func diffLineToStyle(dl diff.StyledLine) common.Style {
	style := common.Style{}

	// Set foreground color
	switch dl.Foreground {
	case "white":
		style.FG = common.NewColor(255, 255, 255)
	case "bright_black":
		style.FG = common.NewColor(120, 120, 120)
	default:
		style.FG = common.NewColor(160, 160, 160) // default gray
	}

	// Set background color
	switch dl.Background {
	case "dark_green":
		style.BG = common.NewColor(0, 80, 0)
	case "dark_red":
		style.BG = common.NewColor(120, 0, 0)
	}

	// Set bold if needed
	if dl.Bold {
		style.Bold = true
	}

	return style
}
