package editor

import (
	"testing"

	"github.com/ArditZubaku/tex/internal/editor/state"
)

func TestVisualDeleteRunesOnOneLine(t *testing.T) {
	b := inReadMode(t, "abcdef\n", 0, 0)

	press(t, "vlld")

	wantLines(t, b, "def")
	if ed.Mode != state.ReadMode {
		t.Errorf("mode = %v, want ReadMode", ed.Mode)
	}
	if ed.Col != 0 {
		t.Errorf("currentCol = %d, want 0", ed.Col)
	}
}

func TestVisualDeleteAcrossLinesJoinsWhatIsLeft(t *testing.T) {
	b := inReadMode(t, "foo\nbar\nbaz\n", 0, 1)

	press(t, "vjld")

	wantLines(t, b, "f", "baz")
	if ed.Row != 0 || ed.Col != 0 {
		t.Errorf("cursor at %d,%d, want 0,0", ed.Row, ed.Col)
	}
}

func TestVisualDeleteAndPutRoundTrips(t *testing.T) {
	b := inReadMode(t, "foo\nbar\nbaz\n", 0, 1)

	press(t, "vjld")
	press(t, "p")

	wantLines(t, b, "foo", "bar", "baz")
}

func TestVisualLineDelete(t *testing.T) {
	b := inReadMode(t, "a\nb\nc\n", 0, 0)

	press(t, "Vjd")

	wantLines(t, b, "c")
	press(t, "P")
	wantLines(t, b, "a", "b", "c")
}

func TestVisualSelectionRunsBackwardsToo(t *testing.T) {
	b := inReadMode(t, "a\nb\nc\n", 2, 0)

	press(t, "Vkd")

	wantLines(t, b, "a")
}

func TestVisualYankLeavesTheCursorAtTheStart(t *testing.T) {
	b := inReadMode(t, "foo bar\n", 0, 4)

	press(t, "vhhy")

	wantLines(t, b, "foo bar")
	if ed.Col != 2 {
		t.Errorf("currentCol = %d, want 2", ed.Col)
	}
	if got := string(ed.Clip.Content()[0]); got != "o b" {
		t.Errorf("register = %q, want %q", got, "o b")
	}
}

func TestVisualYankAcrossLinesPutsBackAsARun(t *testing.T) {
	b := inReadMode(t, "foo\nbar\n", 0, 1)

	press(t, "vjy")
	press(t, "p")

	wantLines(t, b, "fooo", "bao", "bar")
}

func TestCountedMotionExtendsTheSelection(t *testing.T) {
	b := inReadMode(t, "abcdef\n", 0, 0)

	press(t, "v3ld")

	wantLines(t, b, "ef")
}

// A count typed before an operator repeats it, and the repeats must not carry
// on eating the text that followed the selection.
func TestCountedOperatorDeletesTheSelectionOnce(t *testing.T) {
	b := inReadMode(t, "abcdef\n", 0, 0)

	press(t, "vl2d")

	wantLines(t, b, "cdef")
}

func TestVisualKeysSwitchAndLeaveTheMode(t *testing.T) {
	inReadMode(t, "abc\n", 0, 0)

	press(t, "v")
	if ed.Mode != state.VisualMode || ed.VisualLine {
		t.Fatalf("v gave mode %v, linewise %v", ed.Mode, ed.VisualLine)
	}

	press(t, "V")
	if ed.Mode != state.VisualMode || !ed.VisualLine {
		t.Fatalf("V gave mode %v, linewise %v", ed.Mode, ed.VisualLine)
	}

	press(t, "V")
	if ed.Mode != state.ReadMode {
		t.Errorf("V again gave mode %v, want ReadMode", ed.Mode)
	}

	press(t, "vv")
	if ed.Mode != state.ReadMode {
		t.Errorf("vv gave mode %v, want ReadMode", ed.Mode)
	}
}

func TestEscLeavesVisualMode(t *testing.T) {
	b := inReadMode(t, "abc\n", 0, 0)

	press(t, "vl")
	esc()

	if ed.Mode != state.ReadMode {
		t.Errorf("mode = %v, want ReadMode", ed.Mode)
	}
	press(t, "d")
	wantLines(t, b, "abc")
}

func TestVisualOSwapsTheEndsOfTheSelection(t *testing.T) {
	b := inReadMode(t, "abcdef\n", 0, 2)

	press(t, "vllohhd")

	wantLines(t, b, "f")
}

func TestVisualChangeReplacesTheRunes(t *testing.T) {
	b := inReadMode(t, "foo bar\n", 0, 0)

	typeIn(t, "vllcBAZ")

	wantLines(t, b, "BAZ bar")
	if ed.Mode != state.EditMode {
		t.Errorf("mode = %v, want EditMode", ed.Mode)
	}
}

func TestVisualLineChangeLeavesOneLineToTypeOn(t *testing.T) {
	b := inReadMode(t, "a\nb\nc\n", 0, 0)

	typeIn(t, "VjcX")

	wantLines(t, b, "X", "c")
}

func TestVisualDeleteAcrossLinesUndoesInOneStep(t *testing.T) {
	b := inReadMode(t, "foo\nbar\nbaz\nqux\n", 0, 1)

	press(t, "vjjld")
	wantLines(t, b, "f", "qux")

	press(t, "u")
	wantLines(t, b, "foo", "bar", "baz", "qux")
}

func TestVisualLineChangeUndoesInOneStep(t *testing.T) {
	b := inReadMode(t, "a\nb\nc\n", 0, 0)

	typeIn(t, "VjcX")
	esc()

	press(t, "u")
	wantLines(t, b, "a", "b", "c")
}

func TestSelectingAnEmptyLineYanksNothingToPutBack(t *testing.T) {
	b := inReadMode(t, "\nfoo\n", 0, 0)

	press(t, "vyjp")

	wantLines(t, b, "", "foo")
	if ed.Col != 0 {
		t.Errorf("currentCol = %d, want 0", ed.Col)
	}
}
