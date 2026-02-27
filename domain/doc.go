// Package domain defines foundational types shared across the application.
//
// Key types for keyboard input:
//
//   - Key       — special key enum (Enter, Tab, arrows, etc.)
//   - Mod       — modifier bitmask (Ctrl, Alt, Shift)
//   - KeyEvent  — a single keystroke (key + rune + modifiers)
//
// Action mapping system:
//
//   - Action    — named operation ("exit", "input.submit", etc.)
//   - KeyScope  — binding visibility (global, overlay, input)
//   - KeyMap    — registry that resolves key events to actions by scope
//
// Configuration:
//
//   - ParseKeyPattern      — parses "ctrl+k", "alt+enter" into KeyEvent patterns
//   - LoadKeyMapBytes/File — populates a KeyMap from JSON binding definitions
//
// This package has no dependencies beyond stdlib.
package domain
