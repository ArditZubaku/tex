package editor

import (
	"github.com/ArditZubaku/tex/internal/buffer"
	"github.com/ArditZubaku/tex/internal/editor/history"
)

// hist is the history of the buffer being edited; the others keep theirs on
// their own entry until they are taken out again.
var hist history.History

func beginChange() { hist.Begin(currentRow, currentCol) }
func endChange()   { hist.End(mode == EditMode) }

func touchLine(row int)       { hist.TouchLine(buf, row) }
func touchInsertLine(row int) { hist.TouchInsert(row) }
func touchDeleteLine(row int) { hist.TouchDelete(buf, row) }

func undo() { stepHistory(hist.Undo) }
func redo() { stepHistory(hist.Redo) }

// A step is only taken from Read mode: an insert session has a change open,
// which undoing halfway through would leave in the history.
func stepHistory(step func(*buffer.Buffer, int, int) (int, int, bool)) {
	if mode != ReadMode {
		return
	}

	row, col, ok := step(buf, currentRow, currentCol)
	if !ok {
		return
	}

	currentRow, currentCol = row, col
	clampCol()
	modified = true
}
