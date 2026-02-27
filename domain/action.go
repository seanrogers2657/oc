package domain

// Action is a named operation the user can perform.
type Action string

// Global actions — always available.
const (
	ActionExit        Action = "exit"
	ActionScrollUp    Action = "scroll_up"
	ActionScrollDown  Action = "scroll_down"
	ActionPageUp      Action = "page_up"
	ActionPageDown    Action = "page_down"
	ActionOpenPalette Action = "open_palette"
)

// Input actions — active when the input area is focused.
const (
	ActionSubmit             Action = "input.submit"
	ActionNewline            Action = "input.newline"
	ActionBackspace          Action = "input.backspace"
	ActionDelete             Action = "input.delete"
	ActionDeleteWordBackward Action = "input.delete_word_backward"
	ActionCursorLeft         Action = "input.cursor_left"
	ActionCursorRight        Action = "input.cursor_right"
	ActionCursorUp           Action = "input.cursor_up"
	ActionCursorDown         Action = "input.cursor_down"
	ActionWordLeft           Action = "input.word_left"
	ActionWordRight          Action = "input.word_right"
	ActionHome               Action = "input.home"
	ActionEnd                Action = "input.end"
	ActionKillLineBefore     Action = "input.kill_line_before"
	ActionClearInput         Action = "input.clear"
	ActionInsertTab          Action = "input.tab"
)

// Overlay actions — active when a picker/palette is open.
const (
	ActionOverlayClose     Action = "overlay.close"
	ActionOverlayConfirm   Action = "overlay.confirm"
	ActionOverlayPrev      Action = "overlay.prev"
	ActionOverlayNext      Action = "overlay.next"
	ActionOverlayBackspace Action = "overlay.backspace"
)
