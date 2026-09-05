package edit_test

import (
	"testing"

	"github.com/ArditZubaku/tex/internal/editor/edtest"
	"github.com/ArditZubaku/tex/internal/editor/state"
)

func TestYankLineAndPaste(t *testing.T) {
	e := state.New()

	b := edtest.InReadMode(t, e, "foo\nbar\n", 0, 0)

	edtest.Press(t, e, "yyp")

	edtest.WantLines(t, b, "foo", "foo", "bar")
	if e.Row != 1 || e.Col != 0 {
		t.Errorf("cursor at %d,%d, want 1,0", e.Row, e.Col)
	}
}

func TestPasteBeforePutsTheLineAbove(t *testing.T) {
	e := state.New()

	b := edtest.InReadMode(t, e, "foo\nbar\n", 1, 0)

	edtest.Press(t, e, "yyP")

	edtest.WantLines(t, b, "foo", "bar", "bar")
	if e.Row != 1 {
		t.Errorf("currentRow = %d, want 1", e.Row)
	}
}

func TestCountedPaste(t *testing.T) {
	e := state.New()

	b := edtest.InReadMode(t, e, "foo\n", 0, 0)

	edtest.Press(t, e, "yy3p")

	edtest.WantLines(t, b, "foo", "foo", "foo", "foo")
}

func TestCountedYankLine(t *testing.T) {
	e := state.New()

	b := edtest.InReadMode(t, e, "a\nb\nc\n", 0, 0)

	edtest.Press(t, e, "2yyp")

	edtest.WantLines(t, b, "a", "a", "b", "b", "c")
}

func TestCharwiseYankAndPaste(t *testing.T) {
	e := state.New()

	b := edtest.InReadMode(t, e, "foo bar\n", 0, 0)

	edtest.Press(t, e, "ywp")

	edtest.WantLines(t, b, "ffoo oo bar")
	if e.Col != 4 {
		t.Errorf("currentCol = %d, want 4", e.Col)
	}
}

func TestDeleteLineFillsTheRegister(t *testing.T) {
	e := state.New()

	b := edtest.InReadMode(t, e, "foo\nbar\n", 0, 0)

	edtest.Press(t, e, "ddp")

	edtest.WantLines(t, b, "bar", "foo")
}

func TestDeleteRuneFillsTheRegister(t *testing.T) {
	e := state.New()

	b := edtest.InReadMode(t, e, "abcdef\n", 0, 0)

	edtest.Press(t, e, "3xp")

	edtest.WantLines(t, b, "dabcef")
}

func TestPasteWithAnEmptyRegisterDoesNothing(t *testing.T) {
	e := state.New()

	b := edtest.InReadMode(t, e, "foo\n", 0, 0)

	edtest.Press(t, e, "p")

	edtest.WantLines(t, b, "foo")
	if e.Modified {
		t.Error("modified with nothing to paste")
	}
}
