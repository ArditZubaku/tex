package editor

import "slices"

// A change is what it takes to undo one command: a list of actions that are
// replayed in reverse order. Nothing snapshots the buffer as a whole — an
// action carries at most the one line it has to put back, so remembering a
// command costs the lines that command touched and nothing more.
type actionKind int

const (
	restoreLine actionKind = iota // put line back as it was
	dropLine                      // undo an inserted line
	addLine                       // undo a deleted line
)

type undoAction struct {
	kind actionKind
	row  int
	line []rune
}

type change struct {
	actions  []undoAction
	row, col int // where the cursor was before the command ran
}

// undoDepth bounds both stacks, so a long session cannot grow them without end.
const undoDepth = 500

var (
	undoStack, redoStack []change
	pendingChange        *change
	touchedRows          map[int]bool
)

func beginChange() {
	if pendingChange != nil {
		return
	}
	pendingChange = &change{row: currentRow, col: currentCol}
	touchedRows = nil // most commands never touch a line, so allocate on demand
}

// endChange closes the open change, except in Edit mode: an insert session runs
// from the key that entered it to the Esc that leaves it, and VIM undoes all of
// it in one go.
func endChange() {
	if pendingChange == nil || mode == EditMode {
		return
	}
	if len(pendingChange.actions) > 0 {
		undoStack = pushChange(undoStack, *pendingChange)
		redoStack = redoStack[:0]
	}
	pendingChange = nil
}

func pushChange(stack []change, c change) []change {
	stack = append(stack, c)
	if len(stack) > undoDepth {
		stack = stack[len(stack)-undoDepth:]
	}

	return stack
}

// touchLine records the content of a line about to be edited. The first
// snapshot of a line wins: later edits to it are already covered by putting the
// original back. A structural change invalidates that, since it renumbers the
// rows the snapshots are keyed by, so both recorders below clear the set.
func touchLine(row int) {
	if pendingChange == nil || touchedRows[row] {
		return
	}
	if touchedRows == nil {
		touchedRows = make(map[int]bool)
	}
	touchedRows[row] = true
	pendingChange.actions = append(pendingChange.actions, undoAction{
		kind: restoreLine,
		row:  row,
		line: slices.Clone(buf.Line(row)),
	})
}

// touchInsertLine and touchDeleteLine are called before the insert or delete
// they describe, while the row still holds what has to be remembered.
func touchInsertLine(row int) {
	if pendingChange == nil {
		return
	}
	pendingChange.actions = append(pendingChange.actions, undoAction{kind: dropLine, row: row})
	clear(touchedRows)
}

func touchDeleteLine(row int) {
	if pendingChange == nil {
		return
	}
	pendingChange.actions = append(pendingChange.actions, undoAction{
		kind: addLine,
		row:  row,
		line: slices.Clone(buf.Line(row)),
	})
	clear(touchedRows)
}

func undo() {
	if mode != ReadMode || len(undoStack) == 0 {
		return
	}
	c := undoStack[len(undoStack)-1]
	undoStack = undoStack[:len(undoStack)-1]
	redoStack = pushChange(redoStack, applyChange(c))
	modified = true
}

func redo() {
	if mode != ReadMode || len(redoStack) == 0 {
		return
	}
	c := redoStack[len(redoStack)-1]
	redoStack = redoStack[:len(redoStack)-1]
	undoStack = pushChange(undoStack, applyChange(c))
	modified = true
}

// applyChange replays a change and returns the change that reverses it, which
// is how redo works: undoing an undo is just another change.
func applyChange(c change) change {
	reverse := change{row: currentRow, col: currentCol}

	for _, action := range slices.Backward(c.actions) {
		switch action.kind {
		case restoreLine:
			reverse.actions = append(reverse.actions, undoAction{
				kind: restoreLine,
				row:  action.row,
				line: slices.Clone(buf.Line(action.row)),
			})
			buf.SetLine(action.row, action.line)
		case dropLine:
			reverse.actions = append(reverse.actions, undoAction{
				kind: addLine,
				row:  action.row,
				line: slices.Clone(buf.Line(action.row)),
			})
			buf.DeleteLine(action.row)
		case addLine:
			reverse.actions = append(reverse.actions, undoAction{kind: dropLine, row: action.row})
			buf.InsertLine(action.row)
			buf.SetLine(action.row, action.line)
		}
	}

	currentRow = min(c.row, buf.LineCount()-1)
	currentCol = c.col
	clampCol()

	return reverse
}
