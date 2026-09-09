package edit

import (
	"slices"

	"github.com/ArditZubaku/tex/internal/chars"
	"github.com/ArditZubaku/tex/internal/editor/state"
)

// ToggleCommentLine is 'gcc': the current line, or with a count the lines
// below it too, gains the language's line comment or loses it.
func ToggleCommentLine(e *state.Editor) {
	ToggleCommentLines(e, e.Row, e.Count())
}

// ToggleCommentSelection is Visual mode's 'gc': every line the selection
// touches is commented or uncommented together, whether the selection itself
// runs by line or by character.
func ToggleCommentSelection(e *state.Editor) {
	s := Selection(e)
	if !s.Active {
		return
	}
	ExitVisual(e)
	e.Row, e.Col = s.startRow, 0

	ToggleCommentLines(e, s.startRow, s.endRow-s.startRow+1)
	e.ClampCol()
}

// ToggleCommentLines treats rows [row, row+n) as one block: only once every
// non-blank line already carries the leader does the block read as commented,
// in which case they all lose it; otherwise every line missing it gains one,
// and a line that already had it is left alone. A file with no known comment
// syntax, and a block that is entirely blank, are both no-ops.
func ToggleCommentLines(e *state.Editor, row, n int) {
	leader := e.Lang.LineComment()
	if leader == "" {
		return
	}
	n = min(n, e.Buf.LineCount()-row)
	if n <= 0 {
		return
	}

	toggleOff := allCommented(e, row, n, leader)
	for r := row; r < row+n; r++ {
		if toggleOff {
			uncommentLine(e, r, leader)
		} else {
			commentLine(e, r, leader)
		}
	}
	e.Modified = true
}

// allCommented requires at least one non-blank line in range, so a block that
// is entirely blank reads as needing comments rather than losing them.
func allCommented(e *state.Editor, row, n int, leader string) bool {
	found := false
	for r := row; r < row+n; r++ {
		line := e.Buf.Line(r)
		at := indentEnd(line)
		if at == len(line) {
			continue
		}
		if !hasRunesAt(line, at, leader) {
			return false
		}
		found = true
	}

	return found
}

func commentLine(e *state.Editor, row int, leader string) {
	line := e.Buf.Line(row)
	at := indentEnd(line)
	if at == len(line) || hasRunesAt(line, at, leader) {
		return
	}

	e.TouchLine(row)
	e.Buf.SetLine(row, slices.Insert(slices.Clone(line), at, append([]rune(leader), ' ')...))
}

func uncommentLine(e *state.Editor, row int, leader string) {
	line := e.Buf.Line(row)
	at := indentEnd(line)
	if at == len(line) || !hasRunesAt(line, at, leader) {
		return
	}

	to := at + len([]rune(leader))
	if to < len(line) && line[to] == ' ' {
		to++
	}

	e.TouchLine(row)
	e.Buf.DeleteRunes(row, at, to)
}

func indentEnd(line []rune) int {
	i := 0
	for i < len(line) && chars.IsSpace(line[i]) {
		i++
	}

	return i
}

func hasRunesAt(line []rune, at int, s string) bool {
	want := []rune(s)
	if at+len(want) > len(line) {
		return false
	}

	return slices.Equal(line[at:at+len(want)], want)
}
