package editor

import (
	"testing"

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
	wantLines(t, b, "bar")

	press(t, "u")
	wantLines(t, b, "foo", "bar")
	if currentRow != 0 || currentCol != 0 {
		t.Errorf("cursor at %d,%d, want 0,0", currentRow, currentCol)
	}

	redo()
	wantLines(t, b, "bar")
}

func TestUndoCountedDeleteLineInOneStep(t *testing.T) {
	b := inReadMode(t, "a\nb\nc\n", 0, 0)

	press(t, "2dd")
	wantLines(t, b, "c")

	press(t, "u")
	wantLines(t, b, "a", "b", "c")
}

func TestUndoInsertSessionAsOneChange(t *testing.T) {
	b := inReadMode(t, "foo\n", 0, 0)

	press(t, "i")
	typeIn(t, "abc")
	esc()
	wantLines(t, b, "abcfoo")

	press(t, "u")
	wantLines(t, b, "foo")
	if hist.CanUndo() {
		t.Error("the insert session left more than one change behind")
	}

	redo()
	wantLines(t, b, "abcfoo")
}

func TestUndoOpenedLine(t *testing.T) {
	b := inReadMode(t, "foo\n", 0, 0)

	press(t, "o")
	typeIn(t, "bar")
	esc()
	wantLines(t, b, "foo", "bar")

	press(t, "u")
	wantLines(t, b, "foo")
}

func TestUndoSplit(t *testing.T) {
	b := inReadMode(t, "foobar\n", 0, 3)

	press(t, "i")
	enter()
	esc()
	wantLines(t, b, "foo", "bar")

	press(t, "u")
	wantLines(t, b, "foobar")

	redo()
	wantLines(t, b, "foo", "bar")
}

func TestUndoJoinRestoresBothLines(t *testing.T) {
	b := inReadMode(t, "foo\nbar\n", 1, 0)

	press(t, "i")
	backspace()
	esc()
	wantLines(t, b, "foobar")

	press(t, "u")
	wantLines(t, b, "foo", "bar")
}

func TestUndoPaste(t *testing.T) {
	b := inReadMode(t, "foo\n", 0, 0)

	press(t, "yy3p")
	wantLines(t, b, "foo", "foo", "foo", "foo")

	press(t, "u")
	wantLines(t, b, "foo")
}

func TestNewEditClearsTheRedoStack(t *testing.T) {
	b := inReadMode(t, "a\nb\n", 0, 0)

	press(t, "ddu")
	if !hist.CanRedo() {
		t.Fatal("the undone delete left nothing to redo")
	}

	press(t, "x")
	if hist.CanRedo() {
		t.Error("the new edit left the redo stack standing")
	}
	wantLines(t, b, "", "b")
}

func TestUndoEmptyingTheLastLine(t *testing.T) {
	b := inReadMode(t, "solo\n", 0, 0)

	press(t, "dd")
	wantLines(t, b, "")

	press(t, "u")
	wantLines(t, b, "solo")
}

func TestUndoDeleteWord(t *testing.T) {
	b := inReadMode(t, "foo bar baz\n", 0, 0)

	press(t, "dw")
	wantLines(t, b, "bar baz")

	press(t, "u")
	wantLines(t, b, "foo bar baz")

	redo()
	wantLines(t, b, "bar baz")
}

func TestUndoWithNothingToUndo(t *testing.T) {
	b := inReadMode(t, "foo\n", 0, 0)

	press(t, "u")

	wantLines(t, b, "foo")
}
