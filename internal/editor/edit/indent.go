package edit

import (
	"slices"

	"github.com/ArditZubaku/tex/internal/chars"
	"github.com/ArditZubaku/tex/internal/editor/state"
)

// indentNewLine lays the indent on the line the cursor has just landed on,
// read off the line ref it came from: the whitespace ref starts with, one unit
// deeper when ref leaves a bracket open and nest allows it.
func indentNewLine(e *state.Editor, ref int, nest bool) {
	indent := indentOf(e, ref)
	if nest && opensBlock(e, ref) {
		indent = append(indent, unitOf(indent))
	}

	insertIndent(e, e.Row, indent)
	e.Col = len(indent)
}

func insertIndent(e *state.Editor, row int, indent []rune) {
	for col, ch := range indent {
		e.Buf.InsertRune(row, col, ch)
	}
}

// indentOf copies the leading whitespace of row, verbatim so that a file
// indented with real tabs stays that way.
func indentOf(e *state.Editor, row int) []rune {
	line := e.Buf.Line(row)
	end := 0
	for end < len(line) && chars.IsSpace(line[end]) {
		end++
	}

	return slices.Clone(line[:end])
}

// opensBlock says whether row ends on a bracket it left open, the one thing
// that makes the line below it a level deeper.
func opensBlock(e *state.Editor, row int) bool {
	line := e.Buf.Line(row)
	for i := len(line) - 1; i >= 0; i-- {
		if chars.IsSpace(line[i]) {
			continue
		}
		return isOpener(line[i])
	}

	return false
}

// isOpener is closerFor without the quote: a string is not a block, so a line
// ending in one does not indent the line below it.
func isOpener(ch rune) bool {
	_, ok := closerFor[ch]
	return ok && ch != '"'
}

// unitOf is the one rune a level adds, taken from the indent it extends so
// that tabs beget a tab and everything else the space Tab itself types.
func unitOf(indent []rune) rune {
	if slices.Contains(indent, '\t') {
		return '\t'
	}

	return ' '
}
