package tui

// ScrollState tracks viewport position within scrollable content.
type ScrollState struct {
	Offset         int // first visible line (0 = top)
	ViewportHeight int // number of visible lines
	ContentHeight  int // total lines of content
}

// ScrollUp moves the viewport up by n lines, clamped to 0.
func (s *ScrollState) ScrollUp(n int) {
	s.Offset -= n
	if s.Offset < 0 {
		s.Offset = 0
	}
}

// ScrollDown moves the viewport down by n lines, clamped to max scroll.
func (s *ScrollState) ScrollDown(n int) {
	s.Offset += n
	max := s.maxOffset()
	if s.Offset > max {
		s.Offset = max
	}
}

// ScrollToBottom moves the viewport to show the last lines of content.
func (s *ScrollState) ScrollToBottom() {
	s.Offset = s.maxOffset()
}

// AtBottom returns true if the viewport is showing the last line of content.
func (s *ScrollState) AtBottom() bool {
	return s.Offset >= s.maxOffset()
}

// maxOffset returns the maximum valid offset.
func (s *ScrollState) maxOffset() int {
	max := s.ContentHeight - s.ViewportHeight
	if max < 0 {
		return 0
	}
	return max
}
