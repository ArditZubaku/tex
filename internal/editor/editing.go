package editor

import (
	"github.com/ArditZubaku/tex/internal/motion"
	"github.com/nsf/termbox-go"
)

func insertRune(event termbox.Event) {
	ch := event.Ch
	switch event.Key {
	case termbox.KeySpace, termbox.KeyTab:
		ch = ' '
	}

	touchLine(currentRow)
	buf.InsertRune(currentRow, currentCol, ch)
	currentCol++
	modified = true
}

// deleteRune - drop the character under the cursor.
func deleteRune() {
	line := buf.Line(currentRow)
	to := min(currentCol+count(), len(line))
	if currentCol >= to {
		return
	}

	yankChars(currentRow, currentCol, to)
	touchLine(currentRow)
	buf.DeleteRunes(currentRow, currentCol, to)
	modified = true
}

// deleteWord is 'dw' and deleteToWordEnd is 'de'. Both stop at the end of the
// line even when the motion itself would carry on to the next one, which is
// what VIM does: an operator never eats the line break.
func deleteWord() {
	row, col := motion.NextWordFrom(buf, currentRow, currentCol)
	deleteTo(row, col)
}

func deleteToWordEnd() {
	row, col := motion.EndOfWordFrom(buf, currentRow, currentCol)
	deleteTo(row, col+1)
}

// deleteToPrevWord is 'db': unlike dw/de it deletes behind the cursor, so the
// cursor follows the text back.
func deleteToPrevWord() {
	row, col := motion.PrevWordFrom(buf, currentRow, currentCol)
	if row != currentRow {
		col = 0
	}
	if col >= currentCol {
		return
	}

	yankChars(currentRow, col, currentCol)
	touchLine(currentRow)
	buf.DeleteRunes(currentRow, col, currentCol)
	currentCol = col
	modified = true
}

func deleteTo(row, col int) {
	if row != currentRow {
		col = buf.RuneLen(currentRow)
	}
	if col <= currentCol {
		return
	}

	yankChars(currentRow, currentCol, col)
	touchLine(currentRow)
	buf.DeleteRunes(currentRow, currentCol, col)
	modified = true
}

// openLineBelow is 'o' and openLineAbove is 'O': both add an empty line and
// start typing on it.
func openLineBelow() {
	touchInsertLine(currentRow + 1)
	buf.InsertLine(currentRow + 1)
	currentRow++
	startInsert()
}

func openLineAbove() {
	touchInsertLine(currentRow)
	buf.InsertLine(currentRow)
	startInsert()
}

func startInsert() {
	currentCol = 0
	modified = true
	enterEditMode()
}

// enter splits the line at the cursor in Edit mode, the inverse of what
// backspace does at column 0. In Read mode it is VIM's move to the line below.
func enter() {
	if mode != EditMode {
		down()
		return
	}

	touchLine(currentRow)
	touchInsertLine(currentRow + 1)
	buf.SplitLine(currentRow, currentCol)
	currentRow++
	currentCol = 0
	modified = true
}

// backspace deletes behind the cursor in Edit mode, joining onto the line
// above when there is nothing left to delete on this one. In Read mode it is
// VIM's plain leftwards move.
func backspace() {
	if mode != EditMode {
		left()
		return
	}

	switch {
	case currentCol > 0:
		touchLine(currentRow)
		currentCol--
		buf.DeleteRunes(currentRow, currentCol, currentCol+1)
	case currentRow > 0:
		touchLine(currentRow - 1)
		touchDeleteLine(currentRow)
		currentRow--
		currentCol = buf.RuneLen(currentRow)
		buf.JoinLine(currentRow)
	default:
		return
	}

	modified = true
}

// saveFile is Ctrl-S, in either mode: ':w' without the prompt.
func saveFile() {
	writeFile("")
}

func deleteLine() {
	deleteLines(count())
}

func deleteLines(n int) {
	n = min(n, buf.LineCount()-currentRow)
	yankLines(currentRow, n)

	for range n {
		// the last line of a buffer is emptied rather than dropped
		if buf.LineCount() == 1 {
			touchLine(currentRow)
		} else {
			touchDeleteLine(currentRow)
		}
		buf.DeleteLine(currentRow)
	}

	if currentRow >= buf.LineCount() {
		currentRow = buf.LineCount() - 1
	}
	modified = true
}
