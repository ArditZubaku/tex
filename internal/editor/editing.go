package editor

import (
	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/ArditZubaku/tex/internal/motion"
	"github.com/nsf/termbox-go"
)

func insertRune(event termbox.Event) {
	ch := event.Ch
	switch event.Key {
	case termbox.KeySpace, termbox.KeyTab:
		ch = ' '
	}

	ed.TouchLine(ed.Row)
	ed.Buf.InsertRune(ed.Row, ed.Col, ch)
	ed.Col++
	ed.Modified = true
}

// deleteRune - drop the character under the cursor.
func deleteRune() {
	line := ed.Buf.Line(ed.Row)
	to := min(ed.Col+ed.Count(), len(line))
	if ed.Col >= to {
		return
	}

	yankChars(ed.Row, ed.Col, to)
	ed.TouchLine(ed.Row)
	ed.Buf.DeleteRunes(ed.Row, ed.Col, to)
	ed.Modified = true
}

// deleteWord is 'dw' and deleteToWordEnd is 'de'. Both stop at the end of the
// line even when the motion itself would carry on to the next one, which is
// what VIM does: an operator never eats the line break.
func deleteWord() {
	row, col := motion.NextWordFrom(ed.Buf, ed.Row, ed.Col)
	deleteTo(row, col)
}

func deleteToWordEnd() {
	row, col := motion.EndOfWordFrom(ed.Buf, ed.Row, ed.Col)
	deleteTo(row, col+1)
}

// deleteToPrevWord is 'db': unlike dw/de it deletes behind the cursor, so the
// cursor follows the text back.
func deleteToPrevWord() {
	row, col := motion.PrevWordFrom(ed.Buf, ed.Row, ed.Col)
	if row != ed.Row {
		col = 0
	}
	if col >= ed.Col {
		return
	}

	yankChars(ed.Row, col, ed.Col)
	ed.TouchLine(ed.Row)
	ed.Buf.DeleteRunes(ed.Row, col, ed.Col)
	ed.Col = col
	ed.Modified = true
}

func deleteTo(row, col int) {
	if row != ed.Row {
		col = ed.Buf.RuneLen(ed.Row)
	}
	if col <= ed.Col {
		return
	}

	yankChars(ed.Row, ed.Col, col)
	ed.TouchLine(ed.Row)
	ed.Buf.DeleteRunes(ed.Row, ed.Col, col)
	ed.Modified = true
}

// openLineBelow is 'o' and openLineAbove is 'O': both add an empty line and
// start typing on it.
func openLineBelow() {
	ed.TouchInsertLine(ed.Row + 1)
	ed.Buf.InsertLine(ed.Row + 1)
	ed.Row++
	startInsert()
}

func openLineAbove() {
	ed.TouchInsertLine(ed.Row)
	ed.Buf.InsertLine(ed.Row)
	startInsert()
}

func startInsert() {
	ed.Col = 0
	ed.Modified = true
	ed.EnterEditMode()
}

// enter splits the line at the cursor in Edit mode, the inverse of what
// backspace does at column 0. In Read mode it is VIM's move to the line below.
func enter() {
	if ed.Mode != state.EditMode {
		ed.Down()
		return
	}

	ed.TouchLine(ed.Row)
	ed.TouchInsertLine(ed.Row + 1)
	ed.Buf.SplitLine(ed.Row, ed.Col)
	ed.Row++
	ed.Col = 0
	ed.Modified = true
}

// backspace deletes behind the cursor in Edit mode, joining onto the line
// above when there is nothing left to delete on this one. In Read mode it is
// VIM's plain leftwards move.
func backspace() {
	if ed.Mode != state.EditMode {
		ed.Left()
		return
	}

	switch {
	case ed.Col > 0:
		ed.TouchLine(ed.Row)
		ed.Col--
		ed.Buf.DeleteRunes(ed.Row, ed.Col, ed.Col+1)
	case ed.Row > 0:
		ed.TouchLine(ed.Row - 1)
		ed.TouchDeleteLine(ed.Row)
		ed.Row--
		ed.Col = ed.Buf.RuneLen(ed.Row)
		ed.Buf.JoinLine(ed.Row)
	default:
		return
	}

	ed.Modified = true
}

// saveFile is Ctrl-S, in either mode: ':w' without the prompt.
func saveFile() {
	writeFile("")
}

func deleteLine() {
	deleteLines(ed.Count())
}

func deleteLines(n int) {
	n = min(n, ed.Buf.LineCount()-ed.Row)
	yankLines(ed.Row, n)

	for range n {
		// the last line of a buffer is emptied rather than dropped
		if ed.Buf.LineCount() == 1 {
			ed.TouchLine(ed.Row)
		} else {
			ed.TouchDeleteLine(ed.Row)
		}
		ed.Buf.DeleteLine(ed.Row)
	}

	if ed.Row >= ed.Buf.LineCount() {
		ed.Row = ed.Buf.LineCount() - 1
	}
	ed.Modified = true
}
