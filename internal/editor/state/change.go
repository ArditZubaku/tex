package state

import "github.com/ArditZubaku/tex/internal/buffer"

// An EditKind is what happened to a line, for anything holding positions in the
// file that now have to move.
type EditKind int

const (
	// LineChanged is the runes of one line, which may have moved anywhere
	// along it.
	LineChanged EditKind = iota
	// LineInserted is a line put in at that row, so everything from it down is
	// one row further on.
	LineInserted
	// LineDeleted is a line taken out at that row.
	LineDeleted
	// FileRewritten is too much to follow: an undo, a redo, or a formatter
	// having been over the whole of it.
	FileRewritten
)

// OnEdit is told what happened to a line, and is nil until something wants to
// know. The editor's own loop wires it, since nothing below this package may
// reach back up to what does.
var OnEdit func(path string, row int, what EditKind)

// Edited is the report itself, exported for the one change that does not go
// through a line at a time: a formatter having rewritten the whole file.
func Edited(path string, row int, what EditKind) {
	if OnEdit != nil {
		OnEdit(path, row, what)
	}
}

func (e *Editor) BeginChange() { e.Hist.Begin(e.Row, e.Col) }

// EndChange leaves an Edit-mode session open: an insert runs from the key that
// entered it to the Esc that leaves it, and VIM undoes all of it in one go.
func (e *Editor) EndChange() { e.Hist.End(e.Mode == EditMode) }

// Every way text changes outside the buffer itself goes through one of these
// three first, which is what makes them the seam something tracking positions
// in the file can be told about.
func (e *Editor) TouchLine(row int) {
	e.Hist.TouchLine(e.Buf, row)
	Edited(e.SourceFile, row, LineChanged)
}

func (e *Editor) TouchInsertLine(row int) {
	e.Hist.TouchInsert(row)
	Edited(e.SourceFile, row, LineInserted)
}

func (e *Editor) TouchDeleteLine(row int) {
	e.Hist.TouchDelete(e.Buf, row)
	Edited(e.SourceFile, row, LineDeleted)
}

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
	Edited(e.SourceFile, row, FileRewritten)
}
