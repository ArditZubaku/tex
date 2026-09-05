package state

import "github.com/ArditZubaku/tex/internal/buffer"

func (e *Editor) BeginChange() { e.Hist.Begin(e.Row, e.Col) }

// EndChange leaves an Edit-mode session open: an insert runs from the key that
// entered it to the Esc that leaves it, and VIM undoes all of it in one go.
func (e *Editor) EndChange() { e.Hist.End(e.Mode == EditMode) }

func (e *Editor) TouchLine(row int)       { e.Hist.TouchLine(e.Buf, row) }
func (e *Editor) TouchInsertLine(row int) { e.Hist.TouchInsert(row) }
func (e *Editor) TouchDeleteLine(row int) { e.Hist.TouchDelete(e.Buf, row) }

func (e *Editor) Undo() { e.stepHistory(e.Hist.Undo) }
func (e *Editor) Redo() { e.stepHistory(e.Hist.Redo) }

// A step is only taken from Read mode: an insert session has a change open,
// which undoing halfway through would leave in the history.
func (e *Editor) stepHistory(step func(*buffer.Buffer, int, int) (int, int, bool)) {
	if e.Mode != ReadMode {
		return
	}

	row, col, ok := step(e.Buf, e.Row, e.Col)
	if !ok {
		return
	}

	e.Row, e.Col = row, col
	e.ClampCol()
	e.Modified = true
}
