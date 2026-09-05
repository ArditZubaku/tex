package editor

import (
	"testing"

	"github.com/ArditZubaku/tex/internal/editor/edit"
	"github.com/ArditZubaku/tex/internal/editor/edtest"
	"github.com/nsf/termbox-go"
)

func typeIn(t *testing.T, text string) {
	t.Helper()

	for _, ch := range text {
		handleCharKey(termbox.Event{Ch: ch})
	}
}

func TestUndoRedoDeleteLine(t *testing.T) {
	b := inReadMode(t, "foo\nbar\n", 0, 0)

	press(t, "dd")
	edtest.WantLines(t, b, "bar")

	press(t, "u")
	edtest.WantLines(t, b, "foo", "bar")
	if ed.Row != 0 || ed.Col != 0 {
		t.Errorf("cursor at %d,%d, want 0,0", ed.Row, ed.Col)
	}

	ed.Redo()
	edtest.WantLines(t, b, "bar")
}

func TestUndoCountedDeleteLineInOneStep(t *testing.T) {
	b := inReadMode(t, "a\nb\nc\n", 0, 0)

	press(t, "2dd")
	edtest.WantLines(t, b, "c")

	press(t, "u")
	edtest.WantLines(t, b, "a", "b", "c")
}

func TestUndoInsertSessionAsOneChange(t *testing.T) {
	b := inReadMode(t, "foo\n", 0, 0)

	press(t, "i")
	typeIn(t, "abc")
	esc()
	edtest.WantLines(t, b, "abcfoo")

	press(t, "u")
	edtest.WantLines(t, b, "foo")
	if ed.Hist.CanUndo() {
		t.Error("the insert session left more than one change behind")
	}

	ed.Redo()
	edtest.WantLines(t, b, "abcfoo")
}

func TestUndoOpenedLine(t *testing.T) {
	b := inReadMode(t, "foo\n", 0, 0)

	press(t, "o")
	typeIn(t, "bar")
	esc()
	edtest.WantLines(t, b, "foo", "bar")

	press(t, "u")
	edtest.WantLines(t, b, "foo")
}

func TestUndoSplit(t *testing.T) {
	b := inReadMode(t, "foobar\n", 0, 3)

	press(t, "i")
	edit.Enter(ed)
	esc()
	edtest.WantLines(t, b, "foo", "bar")

	press(t, "u")
	edtest.WantLines(t, b, "foobar")

	ed.Redo()
	edtest.WantLines(t, b, "foo", "bar")
}

func TestUndoJoinRestoresBothLines(t *testing.T) {
	b := inReadMode(t, "foo\nbar\n", 1, 0)

	press(t, "i")
	edit.Backspace(ed)
	esc()
	edtest.WantLines(t, b, "foobar")

	press(t, "u")
	edtest.WantLines(t, b, "foo", "bar")
}

func TestUndoPaste(t *testing.T) {
	b := inReadMode(t, "foo\n", 0, 0)

	press(t, "yy3p")
	edtest.WantLines(t, b, "foo", "foo", "foo", "foo")

	press(t, "u")
	edtest.WantLines(t, b, "foo")
}

func TestNewEditClearsTheRedoStack(t *testing.T) {
	b := inReadMode(t, "a\nb\n", 0, 0)

	press(t, "ddu")
	if !ed.Hist.CanRedo() {
		t.Fatal("the undone delete left nothing to redo")
	}

	press(t, "x")
	if ed.Hist.CanRedo() {
		t.Error("the new edit left the redo stack standing")
	}
	edtest.WantLines(t, b, "", "b")
}

func TestUndoEmptyingTheLastLine(t *testing.T) {
	b := inReadMode(t, "solo\n", 0, 0)

	press(t, "dd")
	edtest.WantLines(t, b, "")

	press(t, "u")
	edtest.WantLines(t, b, "solo")
}

func TestUndoDeleteWord(t *testing.T) {
	b := inReadMode(t, "foo bar baz\n", 0, 0)

	press(t, "dw")
	edtest.WantLines(t, b, "bar baz")

	press(t, "u")
	edtest.WantLines(t, b, "foo bar baz")

	ed.Redo()
	edtest.WantLines(t, b, "bar baz")
}

func TestUndoWithNothingToUndo(t *testing.T) {
	b := inReadMode(t, "foo\n", 0, 0)

	press(t, "u")

	edtest.WantLines(t, b, "foo")
}
