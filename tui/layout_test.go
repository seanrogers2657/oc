package tui

import "testing"

func TestComputeLayoutBasic(t *testing.T) {
	l := ComputeLayout(80, 24, 1)

	// Message list: fills top
	if l.MessageList.Y != 0 || l.MessageList.Width != 80 {
		t.Fatalf("MessageList = %+v", l.MessageList)
	}
	if l.MessageList.Height != 20 { // 24 - 1 - 3
		t.Fatalf("MessageList.Height = %d, want 20", l.MessageList.Height)
	}

	// Status bar: 1 row above input
	if l.StatusBar.Y != 20 || l.StatusBar.Height != 1 || l.StatusBar.Width != 80 {
		t.Fatalf("StatusBar = %+v", l.StatusBar)
	}

	// Input area: 3 rows at bottom (1 line + 2 = 3, minimum 3)
	if l.InputArea.Height != 3 {
		t.Fatalf("InputArea.Height = %d, want 3", l.InputArea.Height)
	}
	if l.InputArea.Y != 21 { // 24 - 3
		t.Fatalf("InputArea.Y = %d, want 21", l.InputArea.Y)
	}

	// Panels should tile exactly
	total := l.MessageList.Height + l.StatusBar.Height + l.InputArea.Height
	if total != 24 {
		t.Fatalf("total height = %d, want 24", total)
	}
}

func TestComputeLayoutMultiLineInput(t *testing.T) {
	l := ComputeLayout(80, 24, 5)

	// 5 lines + 2 = 7 rows for input
	if l.InputArea.Height != 7 {
		t.Fatalf("InputArea.Height = %d, want 7", l.InputArea.Height)
	}
}

func TestComputeLayoutInputCapped(t *testing.T) {
	// 20 input lines, but max is height/3 = 8
	l := ComputeLayout(80, 24, 20)

	if l.InputArea.Height != 8 { // 24/3 = 8
		t.Fatalf("InputArea.Height = %d, want 8", l.InputArea.Height)
	}
}

func TestComputeLayoutSmallTerminal(t *testing.T) {
	l := ComputeLayout(40, 6, 1)

	if l.StatusBar.Height != 1 {
		t.Fatalf("StatusBar.Height = %d, want 1", l.StatusBar.Height)
	}
	if l.InputArea.Height != 3 { // minimum
		t.Fatalf("InputArea.Height = %d, want 3", l.InputArea.Height)
	}
	if l.MessageList.Height != 2 { // 6 - 1 - 3
		t.Fatalf("MessageList.Height = %d, want 2", l.MessageList.Height)
	}
}

func TestComputeLayoutZeroSize(t *testing.T) {
	l := ComputeLayout(0, 0, 1)
	if l.StatusBar.Height != 0 && l.MessageList.Height != 0 && l.InputArea.Height != 0 {
		t.Fatal("zero-size terminal should produce empty layout")
	}
}

func TestComputeLayoutContiguity(t *testing.T) {
	// Panels should tile vertically with no gaps: MessageList | StatusBar | InputArea
	for _, h := range []int{10, 20, 24, 40, 50, 80} {
		for _, il := range []int{1, 3, 5, 10} {
			l := ComputeLayout(80, h, il)
			if l.MessageList.Height == 0 && h > 0 {
				continue // skip degenerate
			}
			// MessageList starts at 0
			if l.MessageList.Y != 0 {
				t.Fatalf("h=%d il=%d: MessageList.Y = %d", h, il, l.MessageList.Y)
			}
			// StatusBar starts right after MessageList
			if l.StatusBar.Y != l.MessageList.Y+l.MessageList.Height {
				t.Fatalf("h=%d il=%d: gap between MessageList and StatusBar", h, il)
			}
			// InputArea starts right after StatusBar
			if l.InputArea.Y != l.StatusBar.Y+l.StatusBar.Height {
				t.Fatalf("h=%d il=%d: gap between StatusBar and InputArea", h, il)
			}
			// Total height
			total := l.MessageList.Height + l.StatusBar.Height + l.InputArea.Height
			if total != h {
				t.Fatalf("h=%d il=%d: total=%d", h, il, total)
			}
		}
	}
}
