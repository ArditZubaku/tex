package state_test

import (
	"testing"

	"github.com/ArditZubaku/tex/internal/editor/edit"
	"github.com/ArditZubaku/tex/internal/editor/edtest"
	"github.com/ArditZubaku/tex/internal/editor/state"
)

func TestUndoRedoDeleteLine(t *testing.T) {
	e := state.New()

	b := edtest.InReadMode(t, e, "foo\nbar\n", 0, 0)

	edtest.Press(t, e, "dd")
	edtest.WantLines(t, b, "bar")

	edtest.Press(t, e, "u")
	edtest.WantLines(t, b, "foo", "bar")
	if e.Row != 0 || e.Col != 0 {
		t.Errorf("cursor at %d,%d, want 0,0", e.Row, e.Col)
	}

	e.Redo()
	edtest.WantLines(t, b, "bar")
}

func TestUndoCountedDeleteLineInOneStep(t *testing.T) {
	e := state.New()

	b := edtest.InReadMode(t, e, "a\nb\nc\n", 0, 0)

	edtest.Press(t, e, "2dd")
	edtest.WantLines(t, b, "c")

	edtest.Press(t, e, "u")
	edtest.WantLines(t, b, "a", "b", "c")
}

func TestUndoInsertSessionAsOneChange(t *testing.T) {
	e := state.New()

	b := edtest.InReadMode(t, e, "foo\n", 0, 0)

	edtest.Press(t, e, "i")
	edtest.TypeIn(t, e, "abc")
	edtest.Esc(t, e)
	edtest.WantLines(t, b, "abcfoo")

	edtest.Press(t, e, "u")
	edtest.WantLines(t, b, "foo")
	if e.Hist.CanUndo() {
		t.Error("the insert session left more than one change behind")
	}

	e.Redo()
	edtest.WantLines(t, b, "abcfoo")
}

func TestUndoOpenedLine(t *testing.T) {
	e := state.New()

	b := edtest.InReadMode(t, e, "foo\n", 0, 0)

	edtest.Press(t, e, "o")
	edtest.TypeIn(t, e, "bar")
	edtest.Esc(t, e)
	edtest.WantLines(t, b, "foo", " bar")

	edtest.Press(t, e, "u")
	edtest.WantLines(t, b, "foo")
}

func TestUndoSplit(t *testing.T) {
	e := state.New()

	b := edtest.InReadMode(t, e, "foobar\n", 0, 3)

	edtest.Press(t, e, "i")
	edit.Enter(e)
	edtest.Esc(t, e)
	edtest.WantLines(t, b, "foo", " bar")

	edtest.Press(t, e, "u")
	edtest.WantLines(t, b, "foobar")

	e.Redo()
	edtest.WantLines(t, b, "foo", " bar")
}

func TestUndoJoinRestoresBothLines(t *testing.T) {
	e := state.New()

	b := edtest.InReadMode(t, e, "foo\nbar\n", 1, 0)

	edtest.Press(t, e, "i")
	edit.Backspace(e)
	edtest.Esc(t, e)
	edtest.WantLines(t, b, "foobar")

	edtest.Press(t, e, "u")
	edtest.WantLines(t, b, "foo", "bar")
}

func TestUndoPaste(t *testing.T) {
	e := state.New()

	b := edtest.InReadMode(t, e, "foo\n", 0, 0)

	edtest.Press(t, e, "yy3p")
	edtest.WantLines(t, b, "foo", "foo", "foo", "foo")

	edtest.Press(t, e, "u")
	edtest.WantLines(t, b, "foo")
}

func TestNewEditClearsTheRedoStack(t *testing.T) {
	e := state.New()

	b := edtest.InReadMode(t, e, "a\nb\n", 0, 0)

	edtest.Press(t, e, "ddu")
	if !e.Hist.CanRedo() {
		t.Fatal("the undone delete left nothing to redo")
	}

	edtest.Press(t, e, "x")
	if e.Hist.CanRedo() {
		t.Error("the new edit left the redo stack standing")
	}
	edtest.WantLines(t, b, "", "b")
}

func TestUndoEmptyingTheLastLine(t *testing.T) {
	e := state.New()

	b := edtest.InReadMode(t, e, "solo\n", 0, 0)

	edtest.Press(t, e, "dd")
	edtest.WantLines(t, b, "")

	edtest.Press(t, e, "u")
	edtest.WantLines(t, b, "solo")
}

func TestUndoDeleteWord(t *testing.T) {
	e := state.New()

	b := edtest.InReadMode(t, e, "foo bar baz\n", 0, 0)

	edtest.Press(t, e, "dw")
	edtest.WantLines(t, b, "bar baz")

	edtest.Press(t, e, "u")
	edtest.WantLines(t, b, "foo bar baz")

	e.Redo()
	edtest.WantLines(t, b, "bar baz")
}

func TestUndoWithNothingToUndo(t *testing.T) {
	e := state.New()

	b := edtest.InReadMode(t, e, "foo\n", 0, 0)

	edtest.Press(t, e, "u")

	edtest.WantLines(t, b, "foo")
}
