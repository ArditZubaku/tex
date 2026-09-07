package lsp

import (
	"unicode/utf8"

	"github.com/ArditZubaku/tex/internal/buffer"
)

// An Encoding is how a server counts along a line. The protocol's own default
// is UTF-16 code units, which is what a JavaScript editor's strings are made
// of; a server may agree to UTF-8 bytes instead, which is what this editor's
// lines decode from. Neither is the rune index the cursor is, which is why
// everything crossing the wire comes through here.
type Encoding int

const (
	UTF16 Encoding = iota // what a server naming no encoding at all means
	UTF8
)

func (enc Encoding) String() string {
	if enc == UTF8 {
		return "utf-8"
	}

	return "utf-16"
}

// Encodings are what the editor offers, in the order it prefers them: bytes
// are what a line is held as, so counting them is a walk of the line the
// decoder has already made.
var Encodings = []string{"utf-8", "utf-16"}

// EncodingNamed is a server's answer read back. A server naming nothing means
// the protocol's own default; one naming something never offered is refused,
// since counting a line the wrong way would put every answer a column out and
// look like a rendering bug rather than a handshake that went wrong.
func EncodingNamed(name string) (Encoding, bool) {
	switch name {
	case "utf-8":
		return UTF8, true
	case "utf-16", "":
		return UTF16, true
	}

	return UTF16, false
}

type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// A Range is half-open: End is one past the last unit covered, so a range with
// End equal to Start covers nothing at all — which is what a server sends for
// something missing rather than something wrong.
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Pos is where a cursor is as the server counts it. The line has to be fetched
// to answer at all: how far along it a rune index falls depends on every rune
// before it. Reading a buffer belongs to the loop's goroutine, so this does too.
func (enc Encoding) Pos(b *buffer.Buffer, row, col int) Position {
	return Position{Line: row, Character: enc.Character(b.Line(row), col)}
}

// RowCol is Pos the other way round, clamped onto the buffer: a server naming a
// line the buffer no longer has is one working from text it was told about
// before the last change landed.
func (enc Encoding) RowCol(b *buffer.Buffer, at Position) (int, int) {
	row := min(max(at.Line, 0), b.LineCount()-1)

	return row, enc.Column(b.Line(row), at.Character)
}

// Character is where col — a rune index, which is what the cursor is — falls in
// the units the server counts.
func (enc Encoding) Character(line []rune, col int) int {
	col = min(max(col, 0), len(line))

	units := 0
	for _, ch := range line[:col] {
		units += enc.width(ch)
	}

	return units
}

// Column is Character back: the rune a count of units lands on. One landing
// inside a rune names that rune, since a column between two of them is no
// column the cursor can be at, and one past the end of the line names its end,
// which is what a server sends for a range that runs to it.
func (enc Encoding) Column(line []rune, units int) int {
	if units <= 0 {
		return 0
	}

	at := 0
	for col, ch := range line {
		if at >= units {
			return col
		}
		if next := at + enc.width(ch); next > units {
			return col // the count landed inside this rune, which names it
		}
		at += enc.width(ch)
	}

	return len(line)
}

// width is what one rune costs. A byte the decoder could not make sense of
// became one replacement rune, and is counted as the single byte it stood for,
// since that is what the document holds in its place.
func (enc Encoding) width(ch rune) int {
	if enc == UTF8 {
		if n := utf8.RuneLen(ch); n > 0 {
			return n
		}

		return 1
	}
	if ch > 0xFFFF {
		return 2 // UTF-16 spells it as a surrogate pair
	}

	return 1
}
