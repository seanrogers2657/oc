package common

import "testing"

func TestScrollDown(t *testing.T) {
	s := ScrollState{Offset: 0, ViewportHeight: 10, ContentHeight: 30}
	s.ScrollDown(5)
	if s.Offset != 5 {
		t.Fatalf("Offset = %d, want 5", s.Offset)
	}
}

func TestScrollDownClamp(t *testing.T) {
	s := ScrollState{Offset: 15, ViewportHeight: 10, ContentHeight: 20}
	s.ScrollDown(100)
	if s.Offset != 10 { // max = 20 - 10
		t.Fatalf("Offset = %d, want 10", s.Offset)
	}
}

func TestScrollUp(t *testing.T) {
	s := ScrollState{Offset: 10, ViewportHeight: 10, ContentHeight: 30}
	s.ScrollUp(3)
	if s.Offset != 7 {
		t.Fatalf("Offset = %d, want 7", s.Offset)
	}
}

func TestScrollUpClamp(t *testing.T) {
	s := ScrollState{Offset: 2, ViewportHeight: 10, ContentHeight: 30}
	s.ScrollUp(10)
	if s.Offset != 0 {
		t.Fatalf("Offset = %d, want 0", s.Offset)
	}
}

func TestScrollToBottom(t *testing.T) {
	s := ScrollState{Offset: 0, ViewportHeight: 10, ContentHeight: 30}
	s.ScrollToBottom()
	if s.Offset != 20 {
		t.Fatalf("Offset = %d, want 20", s.Offset)
	}
}

func TestScrollToBottomSmallContent(t *testing.T) {
	s := ScrollState{Offset: 0, ViewportHeight: 20, ContentHeight: 5}
	s.ScrollToBottom()
	if s.Offset != 0 { // content fits, no scroll
		t.Fatalf("Offset = %d, want 0", s.Offset)
	}
}

func TestAtBottom(t *testing.T) {
	s := ScrollState{Offset: 20, ViewportHeight: 10, ContentHeight: 30}
	if !s.AtBottom() {
		t.Fatal("should be at bottom")
	}

	s.Offset = 10
	if s.AtBottom() {
		t.Fatal("should not be at bottom")
	}
}

func TestAtBottomContentFits(t *testing.T) {
	s := ScrollState{Offset: 0, ViewportHeight: 20, ContentHeight: 5}
	if !s.AtBottom() {
		t.Fatal("should be at bottom when content fits viewport")
	}
}
