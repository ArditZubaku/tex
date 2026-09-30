package edit

import (
	"slices"
	"strings"

	"github.com/ArditZubaku/tex/internal/editor/state"
)

// InsertText is a paste landing in Edit mode: spliced into the buffer at the
// cursor exactly as it arrived, cursor left after it so typing carries on
// from there. Typing an opening bracket closes it and typing Enter copies
// the line above's indent because that's almost always what a person meant;
// neither is true of text that already carries its own brackets and
// indentation, so this goes straight to the buffer rather than through
// InsertRune and Enter.
func InsertText(e *state.Editor, text string) {
	if text == "" {
		return
	}

	lines := strings.Split(text, "\n")
	line := slices.Clone(e.Buf.Line(e.Row))
	col := min(e.Col, len(line))

	e.TouchLine(e.Row)

	if len(lines) == 1 {
		e.Buf.SetLine(e.Row, slices.Insert(line, col, []rune(lines[0])...))
		e.Col = col + len(lines[0])
		e.Modified = true
		return
	}

	tail := slices.Clone(line[col:])
	e.Buf.SetLine(e.Row, append(line[:col], []rune(lines[0])...))

	row := e.Row
	for _, l := range lines[1 : len(lines)-1] {
		row++
		e.TouchInsertLine(row)
		e.Buf.InsertLine(row)
		e.Buf.SetLine(row, []rune(l))
	}

	last := []rune(lines[len(lines)-1])
	row++
	e.TouchInsertLine(row)
	e.Buf.InsertLine(row)
	e.Buf.SetLine(row, append(last, tail...))

	e.Row, e.Col = row, len(last)
	e.Modified = true
}
