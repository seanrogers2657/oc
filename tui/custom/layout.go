package custom

import "github.com/seanrogers2657/oc/tui/common"

// Layout holds the computed bounds for the three panels.
type Layout struct {
	StatusBar   common.Rect
	MessageList common.Rect
	InputArea   common.Rect
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

	// Input area: content lines + 3 (top border + bottom border + hint line),
	// minimum 4 rows, maximum 1/3 of screen
	inputAreaH := inputLines + 3
	if inputAreaH < 4 {
		inputAreaH = 4
	}
	maxInputH := height / 3
	if maxInputH < 4 {
		maxInputH = 4
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
		MessageList: common.Rect{
			X: 0, Y: 0,
			Width: width, Height: messageListH,
		},
		StatusBar: common.Rect{
			X: 0, Y: messageListH,
			Width: width, Height: statusBarH,
		},
		InputArea: common.Rect{
			X: 0, Y: messageListH + statusBarH,
			Width: width, Height: inputAreaH,
		},
	}
}
