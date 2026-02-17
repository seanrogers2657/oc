package tui

import "testing"

func TestCursorTo(t *testing.T) {
	tests := []struct {
		x, y int
		want string
	}{
		{0, 0, "\x1b[1;1H"},
		{5, 3, "\x1b[4;6H"},
		{79, 23, "\x1b[24;80H"},
	}
	for _, tt := range tests {
		got := CursorTo(tt.x, tt.y)
		if got != tt.want {
			t.Errorf("CursorTo(%d, %d) = %q, want %q", tt.x, tt.y, got, tt.want)
		}
	}
}

func TestFGRGB(t *testing.T) {
	got := FGRGB(255, 128, 0)
	want := "\x1b[38;2;255;128;0m"
	if got != want {
		t.Errorf("FGRGB(255,128,0) = %q, want %q", got, want)
	}
}

func TestBGRGB(t *testing.T) {
	got := BGRGB(0, 64, 128)
	want := "\x1b[48;2;0;64;128m"
	if got != want {
		t.Errorf("BGRGB(0,64,128) = %q, want %q", got, want)
	}
}

func TestFG256(t *testing.T) {
	got := FG256(196)
	want := "\x1b[38;5;196m"
	if got != want {
		t.Errorf("FG256(196) = %q, want %q", got, want)
	}
}

func TestBG256(t *testing.T) {
	got := BG256(21)
	want := "\x1b[48;5;21m"
	if got != want {
		t.Errorf("BG256(21) = %q, want %q", got, want)
	}
}

func TestStyleToANSIEmpty(t *testing.T) {
	got := StyleToANSI(Style{})
	if got != "" {
		t.Errorf("empty style should produce empty string, got %q", got)
	}
}

func TestStyleToANSIBold(t *testing.T) {
	got := StyleToANSI(Style{Bold: true})
	if got != "\x1b[1m" {
		t.Errorf("bold style = %q, want %q", got, "\x1b[1m")
	}
}

func TestStyleToANSIFull(t *testing.T) {
	s := Style{
		FG:            NewColor(255, 0, 0),
		BG:            NewColor(0, 0, 255),
		Bold:          true,
		Dim:           true,
		Italic:        true,
		Underline:     true,
		Strikethrough: true,
	}
	got := StyleToANSI(s)

	// Should contain all attribute codes
	expects := []string{
		"\x1b[1m",            // bold
		"\x1b[2m",            // dim
		"\x1b[3m",            // italic
		"\x1b[4m",            // underline
		"\x1b[9m",            // strikethrough
		"\x1b[38;2;255;0;0m", // FG red
		"\x1b[48;2;0;0;255m", // BG blue
	}
	for _, e := range expects {
		if !contains(got, e) {
			t.Errorf("StyleToANSI missing %q in %q", e, got)
		}
	}
}

func TestStylesEqual(t *testing.T) {
	a := Style{Bold: true, FG: NewColor(1, 2, 3)}
	b := Style{Bold: true, FG: NewColor(1, 2, 3)}
	c := Style{Bold: true, FG: NewColor(1, 2, 4)}

	if !StylesEqual(a, b) {
		t.Error("identical styles should be equal")
	}
	if StylesEqual(a, c) {
		t.Error("different styles should not be equal")
	}
}

func TestConstants(t *testing.T) {
	// Verify key constants are correct escape sequences
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"AltScreenEnter", AltScreenEnter, "\x1b[?1049h"},
		{"AltScreenExit", AltScreenExit, "\x1b[?1049l"},
		{"CursorHide", CursorHide, "\x1b[?25l"},
		{"CursorShow", CursorShow, "\x1b[?25h"},
		{"ClearScreen", ClearScreen, "\x1b[2J"},
		{"ClearLine", ClearLine, "\x1b[2K"},
		{"ResetStyle", ResetStyle, "\x1b[0m"},
		{"Bold", Bold, "\x1b[1m"},
		{"Dim", Dim, "\x1b[2m"},
		{"Italic", Italic, "\x1b[3m"},
		{"Underline", Underline, "\x1b[4m"},
		{"Strikethrough", Strikethrough, "\x1b[9m"},
{"BracketedPasteEnable", BracketedPasteEnable, "\x1b[?2004h"},
		{"BracketedPasteDisable", BracketedPasteDisable, "\x1b[?2004l"},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s = %q, want %q", tt.name, tt.got, tt.want)
		}
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle || len(needle) == 0 ||
		(len(haystack) > 0 && containsStr(haystack, needle)))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
