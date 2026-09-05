// Package history is what 'u' and Ctrl-R step through: the changes made to one
// buffer, each of them small enough to be remembered by the lines it touched.
package history

import (
	"slices"

	"github.com/ArditZubaku/tex/internal/buffer"
)

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

type action struct {
	kind actionKind
	row  int
	line []rune
}

type change struct {
	actions  []action
	row, col int // where the cursor was before the command ran
}

// depth bounds both stacks, so a long session cannot grow them without end.
const depth = 500

// A History belongs to one buffer, which is why it is a value: switching
// buffers puts one away and takes another out.
type History struct {
	undo, redo []change
	pending    *change
	touched    map[int]bool
}

func (h *History) CanUndo() bool { return len(h.undo) > 0 }
func (h *History) CanRedo() bool { return len(h.redo) > 0 }

func (h *History) Begin(row, col int) {
	if h.pending != nil {
		return
	}
	h.pending = &change{row: row, col: col}
	h.touched = nil // most commands never touch a line, so allocate on demand
}

// End closes the open change, unless it is being held: an insert session runs
// from the key that entered it to the Esc that leaves it, and VIM undoes all of
// it in one go.
func (h *History) End(hold bool) {
	if h.pending == nil || hold {
		return
	}
	if len(h.pending.actions) > 0 {
		h.undo = push(h.undo, *h.pending)
		h.redo = h.redo[:0]
	}
	h.pending = nil
}

// Abandon drops the change being recorded without keeping it, which is what a
// buffer taken out again needs: whatever was open belonged to the last session
// in it.
func (h *History) Abandon() {
	h.pending, h.touched = nil, nil
}

func push(stack []change, c change) []change {
	stack = append(stack, c)
	if len(stack) > depth {
		stack = stack[len(stack)-depth:]
	}

	return stack
}

// TouchLine records the content of a line about to be edited. The first
// snapshot of a line wins: later edits to it are already covered by putting the
// original back. A structural change invalidates that, since it renumbers the
// rows the snapshots are keyed by, so both recorders below clear the set.
func (h *History) TouchLine(b *buffer.Buffer, row int) {
	if h.pending == nil || h.touched[row] {
		return
	}
	if h.touched == nil {
		h.touched = make(map[int]bool)
	}
	h.touched[row] = true
	h.pending.actions = append(h.pending.actions, action{
		kind: restoreLine,
		row:  row,
		line: slices.Clone(b.Line(row)),
	})
}

// TouchInsert and TouchDelete are called before the insert or delete they
// describe, while the row still holds what has to be remembered.
func (h *History) TouchInsert(row int) {
	if h.pending == nil {
		return
	}
	h.pending.actions = append(h.pending.actions, action{kind: dropLine, row: row})
	clear(h.touched)
}

func (h *History) TouchDelete(b *buffer.Buffer, row int) {
	if h.pending == nil {
		return
	}
	h.pending.actions = append(h.pending.actions, action{
		kind: addLine,
		row:  row,
		line: slices.Clone(b.Line(row)),
	})
	clear(h.touched)
}

// Undo puts the last change back and says where the cursor belongs afterwards.
func (h *History) Undo(b *buffer.Buffer, row, col int) (int, int, bool) {
	if len(h.undo) == 0 {
		return row, col, false
	}
	c := h.undo[len(h.undo)-1]
	h.undo = h.undo[:len(h.undo)-1]

	reverse, row, col := apply(b, c, row, col)
	h.redo = push(h.redo, reverse)

	return row, col, true
}

func (h *History) Redo(b *buffer.Buffer, row, col int) (int, int, bool) {
	if len(h.redo) == 0 {
		return row, col, false
	}
	c := h.redo[len(h.redo)-1]
	h.redo = h.redo[:len(h.redo)-1]

	reverse, row, col := apply(b, c, row, col)
	h.undo = push(h.undo, reverse)

	return row, col, true
}

// apply replays a change and returns the change that reverses it, which is how
// redo works: undoing an undo is just another change.
func apply(b *buffer.Buffer, c change, row, col int) (change, int, int) {
	reverse := change{row: row, col: col}

	for _, one := range slices.Backward(c.actions) {
		switch one.kind {
		case restoreLine:
			reverse.actions = append(reverse.actions, action{
				kind: restoreLine,
				row:  one.row,
				line: slices.Clone(b.Line(one.row)),
			})
			b.SetLine(one.row, one.line)
		case dropLine:
			reverse.actions = append(reverse.actions, action{
				kind: addLine,
				row:  one.row,
				line: slices.Clone(b.Line(one.row)),
			})
			b.DeleteLine(one.row)
		case addLine:
			reverse.actions = append(reverse.actions, action{kind: dropLine, row: one.row})
			b.InsertLine(one.row)
			b.SetLine(one.row, one.line)
		}
	}

	return reverse, min(c.row, b.LineCount()-1), c.col
}
