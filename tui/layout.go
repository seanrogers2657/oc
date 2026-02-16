package tui

// Layout holds the computed bounds for the three panels.
type Layout struct {
	StatusBar   Rect
	MessageList Rect
	InputArea   Rect
}

// ComputeLayout computes the 3-panel layout for the given terminal dimensions.
// inputLines is the number of text lines in the input editor (minimum 1).
//
// Layout:
//
//	MessageList (fills top)
//	StatusBar   (1 row, above input)
//	InputArea   (3-N rows, bottom)
func ComputeLayout(width, height, inputLines int) Layout {
	if width < 1 || height < 1 {
		return Layout{}
	}

	statusBarH := 1

	// Input area: content lines + 2 (border/prompt + hint line),
	// minimum 3 rows, maximum 1/3 of screen
	inputAreaH := inputLines + 2
	if inputAreaH < 3 {
		inputAreaH = 3
	}
	maxInputH := height / 3
	if maxInputH < 3 {
		maxInputH = 3
	}
	if inputAreaH > maxInputH {
		inputAreaH = maxInputH
	}

	// Message list fills what's left
	messageListH := height - statusBarH - inputAreaH
	if messageListH < 0 {
		messageListH = 0
	}

	return Layout{
		MessageList: Rect{
			X: 0, Y: 0,
			Width: width, Height: messageListH,
		},
		StatusBar: Rect{
			X: 0, Y: messageListH,
			Width: width, Height: statusBarH,
		},
		InputArea: Rect{
			X: 0, Y: messageListH + statusBarH,
			Width: width, Height: inputAreaH,
		},
	}
}
