// Package edit is what changes the text: typing and deleting, the yank and
// paste that move it about, and the Visual selection the operators act on.
package edit

import (
	"github.com/nsf/termbox-go"

	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/ArditZubaku/tex/internal/motion"
)

func InsertRune(e *state.Editor, event termbox.Event) {
	ch := event.Ch
	switch event.Key {
	case termbox.KeySpace, termbox.KeyTab:
		ch = ' '
	}

	e.TouchLine(e.Row)
	e.Buf.InsertRune(e.Row, e.Col, ch)
	e.Col++
	e.Modified = true
}

func DeleteRune(e *state.Editor) {
	line := e.Buf.Line(e.Row)
	to := min(e.Col+e.Count(), len(line))
	if e.Col >= to {
		return
	}

	yankChars(e, e.Row, e.Col, to)
	e.TouchLine(e.Row)
	e.Buf.DeleteRunes(e.Row, e.Col, to)
	e.Modified = true
}

// DeleteWord is 'dw' and DeleteToWordEnd is 'de'. Both stop at the end of the
// line even when the motion itself would carry on to the next one, which is
// what VIM does: an operator never eats the line break.
func DeleteWord(e *state.Editor) {
	row, col := motion.NextWordFrom(e.Buf, e.Row, e.Col)
	deleteTo(e, row, col)
}

func DeleteToWordEnd(e *state.Editor) {
	row, col := motion.EndOfWordFrom(e.Buf, e.Row, e.Col)
	deleteTo(e, row, col+1)
}

// DeleteToPrevWord is 'db': unlike dw/de it deletes behind the cursor, so the
// cursor follows the text back.
func DeleteToPrevWord(e *state.Editor) {
	row, col := motion.PrevWordFrom(e.Buf, e.Row, e.Col)
	if row != e.Row {
		col = 0
	}
	if col >= e.Col {
		return
	}

	yankChars(e, e.Row, col, e.Col)
	e.TouchLine(e.Row)
	e.Buf.DeleteRunes(e.Row, col, e.Col)
	e.Col = col
	e.Modified = true
}

func deleteTo(e *state.Editor, row, col int) {
	if row != e.Row {
		col = e.Buf.RuneLen(e.Row)
	}
	if col <= e.Col {
		return
	}

	yankChars(e, e.Row, e.Col, col)
	e.TouchLine(e.Row)
	e.Buf.DeleteRunes(e.Row, e.Col, col)
	e.Modified = true
}

// OpenLineBelow is 'o' and OpenLineAbove is 'O': both add an empty line and
// start typing on it.
func OpenLineBelow(e *state.Editor) {
	e.TouchInsertLine(e.Row + 1)
	e.Buf.InsertLine(e.Row + 1)
	e.Row++
	startInsert(e)
}

func OpenLineAbove(e *state.Editor) {
	e.TouchInsertLine(e.Row)
	e.Buf.InsertLine(e.Row)
	startInsert(e)
}

func startInsert(e *state.Editor) {
	e.Col = 0
	e.Modified = true
	e.EnterEditMode()
}

// Enter splits the line at the cursor in Edit mode, the inverse of what
// Backspace does at column 0. In Read mode it is VIM's move to the line below.
func Enter(e *state.Editor) {
	if e.Mode != state.EditMode {
		e.Down()
		return
	}

	e.TouchLine(e.Row)
	e.TouchInsertLine(e.Row + 1)
	e.Buf.SplitLine(e.Row, e.Col)
	e.Row++
	e.Col = 0
	e.Modified = true
}

// Backspace deletes behind the cursor in Edit mode, joining onto the line
// above when there is nothing left to delete on this one. In Read mode it is
// VIM's plain leftwards move.
func Backspace(e *state.Editor) {
	if e.Mode != state.EditMode {
		e.Left()
		return
	}

	switch {
	case e.Col > 0:
		e.TouchLine(e.Row)
		e.Col--
		e.Buf.DeleteRunes(e.Row, e.Col, e.Col+1)
	case e.Row > 0:
		e.TouchLine(e.Row - 1)
		e.TouchDeleteLine(e.Row)
		e.Row--
		e.Col = e.Buf.RuneLen(e.Row)
		e.Buf.JoinLine(e.Row)
	default:
		return
	}

	e.Modified = true
}

func DeleteLine(e *state.Editor) {
	DeleteLines(e, e.Count())
}

func DeleteLines(e *state.Editor, n int) {
	n = min(n, e.Buf.LineCount()-e.Row)
	YankLines(e, e.Row, n)

	for range n {
		// the last line of a buffer is emptied rather than dropped
		if e.Buf.LineCount() == 1 {
			e.TouchLine(e.Row)
		} else {
			e.TouchDeleteLine(e.Row)
		}
		e.Buf.DeleteLine(e.Row)
	}

	if e.Row >= e.Buf.LineCount() {
		e.Row = e.Buf.LineCount() - 1
	}
	e.Modified = true
}
