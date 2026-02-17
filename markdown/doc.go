// Package markdown is a line-oriented markdown-to-styled-text parser for
// terminal rendering. It produces []Line, where each Line contains styled
// Spans with a SpanKind indicating the formatting.
//
// Supported syntax:
//
//   - Inline: **bold**, *italic*, ***bold+italic***, `code`, [links](url)
//   - Block: # headings (1-6), > blockquotes, fenced ```code blocks```,
//     unordered lists (-, *, +), ordered lists (1.), horizontal rules (---)
//
// The parser is single-pass and does not build an AST. Each input line maps
// to one or more output Lines. The TUI's MessageList handles word-wrapping
// and final rendering to the screen buffer.
//
// This package has no dependencies beyond stdlib.
package markdown
