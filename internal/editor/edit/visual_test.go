package edit_test

import (
	"testing"

	"github.com/ArditZubaku/tex/internal/editor/edtest"
	"github.com/ArditZubaku/tex/internal/editor/state"
)

func TestVisualDeleteRunesOnOneLine(t *testing.T) {
	e := state.New()

	b := edtest.InReadMode(t, e, "abcdef\n", 0, 0)

	edtest.Press(t, e, "vlld")

	edtest.WantLines(t, b, "def")
	if e.Mode != state.ReadMode {
		t.Errorf("mode = %v, want ReadMode", e.Mode)
	}
	if e.Col != 0 {
		t.Errorf("currentCol = %d, want 0", e.Col)
	}
}

func TestVisualDeleteAcrossLinesJoinsWhatIsLeft(t *testing.T) {
	e := state.New()

	b := edtest.InReadMode(t, e, "foo\nbar\nbaz\n", 0, 1)

	edtest.Press(t, e, "vjld")

	edtest.WantLines(t, b, "f", "baz")
	if e.Row != 0 || e.Col != 0 {
		t.Errorf("cursor at %d,%d, want 0,0", e.Row, e.Col)
	}
}

func TestVisualDeleteAndPutRoundTrips(t *testing.T) {
	e := state.New()

	b := edtest.InReadMode(t, e, "foo\nbar\nbaz\n", 0, 1)

	edtest.Press(t, e, "vjld")
	edtest.Press(t, e, "p")

	edtest.WantLines(t, b, "foo", "bar", "baz")
}

func TestVisualLineDelete(t *testing.T) {
	e := state.New()

	b := edtest.InReadMode(t, e, "a\nb\nc\n", 0, 0)

	edtest.Press(t, e, "Vjd")

	edtest.WantLines(t, b, "c")
	edtest.Press(t, e, "P")
	edtest.WantLines(t, b, "a", "b", "c")
}

func TestVisualSelectionRunsBackwardsToo(t *testing.T) {
	e := state.New()

	b := edtest.InReadMode(t, e, "a\nb\nc\n", 2, 0)

	edtest.Press(t, e, "Vkd")

	edtest.WantLines(t, b, "a")
}

func TestVisualYankLeavesTheCursorAtTheStart(t *testing.T) {
	e := state.New()

	b := edtest.InReadMode(t, e, "foo bar\n", 0, 4)

	edtest.Press(t, e, "vhhy")

	edtest.WantLines(t, b, "foo bar")
	if e.Col != 2 {
		t.Errorf("currentCol = %d, want 2", e.Col)
	}
	if got := string(e.Clip.Content()[0]); got != "o b" {
		t.Errorf("register = %q, want %q", got, "o b")
	}
}

func TestVisualYankAcrossLinesPutsBackAsARun(t *testing.T) {
	e := state.New()

	b := edtest.InReadMode(t, e, "foo\nbar\n", 0, 1)

	edtest.Press(t, e, "vjy")
	edtest.Press(t, e, "p")

	edtest.WantLines(t, b, "fooo", "bao", "bar")
}

func TestVisualPasteReplacesTheSelection(t *testing.T) {
	e := state.New()

	b := edtest.InReadMode(t, e, "foo bar\n", 0, 0)

	edtest.Press(t, e, "vlly")
	edtest.Press(t, e, "llllvllp")

	edtest.WantLines(t, b, "foo foo")
	if got := string(e.Clip.Content()[0]); got != "bar" {
		t.Errorf("register = %q, want %q", got, "bar")
	}
}

func TestVisualLinePasteReplacesTheLine(t *testing.T) {
	e := state.New()

	b := edtest.InReadMode(t, e, "foo\nbar\nbaz\n", 0, 0)

	edtest.Press(t, e, "yy")
	edtest.Press(t, e, "jVp")

	edtest.WantLines(t, b, "foo", "foo", "baz")
}

func TestVisualPasteWithAnEmptyRegisterDoesNothing(t *testing.T) {
	e := state.New()

	b := edtest.InReadMode(t, e, "foo\n", 0, 0)

	edtest.Press(t, e, "vlp")

	edtest.WantLines(t, b, "foo")
	if e.Mode != state.VisualMode {
		t.Errorf("mode = %v, want VisualMode", e.Mode)
	}
}

func TestVisualDollarSelectsToEndOfLine(t *testing.T) {
	e := state.New()

	b := edtest.InReadMode(t, e, "abcdef\n", 0, 1)

	edtest.Press(t, e, "v$d")

	edtest.WantLines(t, b, "a")
}

func TestCountedMotionExtendsTheSelection(t *testing.T) {
	e := state.New()

	b := edtest.InReadMode(t, e, "abcdef\n", 0, 0)

	edtest.Press(t, e, "v3ld")

	edtest.WantLines(t, b, "ef")
}

// A count typed before an operator repeats it, and the repeats must not carry
// on eating the text that followed the selection.
func TestCountedOperatorDeletesTheSelectionOnce(t *testing.T) {
	e := state.New()

	b := edtest.InReadMode(t, e, "abcdef\n", 0, 0)

	edtest.Press(t, e, "vl2d")

	edtest.WantLines(t, b, "cdef")
}

func TestVisualKeysSwitchAndLeaveTheMode(t *testing.T) {
	e := state.New()

	edtest.InReadMode(t, e, "abc\n", 0, 0)

	edtest.Press(t, e, "v")
	if e.Mode != state.VisualMode || e.VisualLine {
		t.Fatalf("v gave mode %v, linewise %v", e.Mode, e.VisualLine)
	}

	edtest.Press(t, e, "V")
	if e.Mode != state.VisualMode || !e.VisualLine {
		t.Fatalf("V gave mode %v, linewise %v", e.Mode, e.VisualLine)
	}

	edtest.Press(t, e, "V")
	if e.Mode != state.ReadMode {
		t.Errorf("V again gave mode %v, want ReadMode", e.Mode)
	}

	edtest.Press(t, e, "vv")
	if e.Mode != state.ReadMode {
		t.Errorf("vv gave mode %v, want ReadMode", e.Mode)
	}
}

func TestEscLeavesVisualMode(t *testing.T) {
	e := state.New()

	b := edtest.InReadMode(t, e, "abc\n", 0, 0)

	edtest.Press(t, e, "vl")
	edtest.Esc(t, e)

	if e.Mode != state.ReadMode {
		t.Errorf("mode = %v, want ReadMode", e.Mode)
	}
	edtest.Press(t, e, "d")
	edtest.WantLines(t, b, "abc")
}

func TestVisualOSwapsTheEndsOfTheSelection(t *testing.T) {
	e := state.New()

	b := edtest.InReadMode(t, e, "abcdef\n", 0, 2)

	edtest.Press(t, e, "vllohhd")

	edtest.WantLines(t, b, "f")
}

func TestVisualChangeReplacesTheRunes(t *testing.T) {
	e := state.New()

	b := edtest.InReadMode(t, e, "foo bar\n", 0, 0)

	edtest.TypeIn(t, e, "vllcBAZ")

	edtest.WantLines(t, b, "BAZ bar")
	if e.Mode != state.EditMode {
		t.Errorf("mode = %v, want EditMode", e.Mode)
	}
}

func TestVisualLineChangeLeavesOneLineToTypeOn(t *testing.T) {
	e := state.New()

	b := edtest.InReadMode(t, e, "a\nb\nc\n", 0, 0)

	edtest.TypeIn(t, e, "VjcX")

	edtest.WantLines(t, b, "X", "c")
}

func TestVisualDeleteAcrossLinesUndoesInOneStep(t *testing.T) {
	e := state.New()

	b := edtest.InReadMode(t, e, "foo\nbar\nbaz\nqux\n", 0, 1)

	edtest.Press(t, e, "vjjld")
	edtest.WantLines(t, b, "f", "qux")

	edtest.Press(t, e, "u")
	edtest.WantLines(t, b, "foo", "bar", "baz", "qux")
}

func TestVisualLineChangeUndoesInOneStep(t *testing.T) {
	e := state.New()

	b := edtest.InReadMode(t, e, "a\nb\nc\n", 0, 0)

	edtest.TypeIn(t, e, "VjcX")
	edtest.Esc(t, e)

	edtest.Press(t, e, "u")
	edtest.WantLines(t, b, "a", "b", "c")
}

func TestSelectingAnEmptyLineYanksNothingToPutBack(t *testing.T) {
	e := state.New()

	b := edtest.InReadMode(t, e, "\nfoo\n", 0, 0)

	edtest.Press(t, e, "vyjp")

	edtest.WantLines(t, b, "", "foo")
	if e.Col != 0 {
		t.Errorf("currentCol = %d, want 0", e.Col)
	}
}
