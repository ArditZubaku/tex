package diag_test

import (
	"strings"
	"testing"

	"github.com/ArditZubaku/tex/internal/editor/diag"
	"github.com/ArditZubaku/tex/internal/editor/edtest"
	"github.com/ArditZubaku/tex/internal/editor/state"
)

// pinned is the editor with the hook wired the way the loop wires it, over a
// file with something said about three of its lines.
func pinned(t *testing.T, row, col int) *state.Editor {
	t.Helper()

	e := state.New()
	edtest.InReadMode(t, e, strings.Repeat("some line of code\n", 20), row, col)
	edtest.SingleWindow(e, 10, 80)

	state.OnEdit = diag.Edited
	t.Cleanup(func() {
		state.OnEdit = nil
		diag.Reset()
	})

	diag.Set(e.SourceFile, diag.File{
		{Severity: diag.Error, Message: "on 2", Row: 2, Col: 0, EndRow: 2, EndCol: 4},
		{Severity: diag.Error, Message: "on 7", Row: 7, Col: 0, EndRow: 7, EndCol: 4},
		{Severity: diag.Error, Message: "on 12", Row: 12, Col: 0, EndRow: 12, EndCol: 4},
	})

	return e
}

func rowsOf(e *state.Editor) []int {
	notes := diag.Of(e.SourceFile)
	rows := make([]int, 0, len(notes))
	for _, note := range notes {
		rows = append(rows, note.Row)
	}

	return rows
}

func wantRows(t *testing.T, e *state.Editor, want ...int) {
	t.Helper()

	got := rowsOf(e)
	if len(got) != len(want) {
		t.Fatalf("diagnostics on rows %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("diagnostics on rows %v, want %v", got, want)
		}
	}
}

func TestALinePutInMovesEverythingBelowItDown(t *testing.T) {
	e := pinned(t, 5, 0)

	edtest.Press(t, e, "O")
	edtest.Esc(t, e)

	wantRows(t, e, 2, 8, 13)
}

func TestALineTakenOutMovesEverythingBelowItUpAndTakesItsOwn(t *testing.T) {
	e := pinned(t, 7, 0)

	edtest.Press(t, e, "dd")

	wantRows(t, e, 2, 11)
}

func TestTypingOnALineDropsWhatWasSaidAboutThatLineAlone(t *testing.T) {
	e := pinned(t, 7, 2)

	edtest.Press(t, e, "x")

	wantRows(t, e, 2, 12)
}

// An undo, a redo or a formatter's rewrite moves too much to follow, and the
// server republishes a fraction of a second later anyway.
func TestAnUndoDropsTheWholeFile(t *testing.T) {
	e := pinned(t, 7, 2)

	edtest.Press(t, e, "x")
	edtest.Press(t, e, "u")

	wantRows(t, e)
}

func TestSomethingSaidAboutAnotherFileIsLeftAlone(t *testing.T) {
	e := pinned(t, 5, 0)
	other := edtest.WriteTemp(t, "package other\n")
	diag.Set(other, diag.File{{Severity: diag.Error, Message: "elsewhere", Row: 4}})

	edtest.Press(t, e, "dd")

	if got := diag.Of(other); len(got) != 1 || got[0].Row != 4 {
		t.Errorf("the other file's diagnostics are %v, want the one on row 4", got)
	}
}

// A range that starts above a line and reaches past it grows or shrinks by that
// line rather than moving with it.
func TestARangeAcrossTheEditedLineStretchesRatherThanMoving(t *testing.T) {
	e := state.New()
	edtest.InReadMode(t, e, strings.Repeat("some line of code\n", 20), 6, 0)
	edtest.SingleWindow(e, 10, 80)

	state.OnEdit = diag.Edited
	t.Cleanup(func() {
		state.OnEdit = nil
		diag.Reset()
	})

	diag.Set(e.SourceFile, diag.File{
		{Severity: diag.Error, Message: "over four lines", Row: 4, Col: 0, EndRow: 8, EndCol: 2},
	})

	edtest.Press(t, e, "O")
	edtest.Esc(t, e)

	note := diag.Of(e.SourceFile)[0]
	if note.Row != 4 || note.EndRow != 9 {
		t.Errorf("the range is rows %d to %d, want 4 to 9", note.Row, note.EndRow)
	}
}
