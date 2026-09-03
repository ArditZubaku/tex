package main

import "testing"

func TestVisualDeleteRunesOnOneLine(t *testing.T) {
	b := inReadMode(t, "abcdef\n", 0, 0)

	press(t, "vlld")

	wantLines(t, b, "def")
	if mode != ReadMode {
		t.Errorf("mode = %v, want ReadMode", mode)
	}
	if currentCol != 0 {
		t.Errorf("currentCol = %d, want 0", currentCol)
	}
}

func TestVisualDeleteAcrossLinesJoinsWhatIsLeft(t *testing.T) {
	b := inReadMode(t, "foo\nbar\nbaz\n", 0, 1)

	press(t, "vjld")

	wantLines(t, b, "f", "baz")
	if currentRow != 0 || currentCol != 0 {
		t.Errorf("cursor at %d,%d, want 0,0", currentRow, currentCol)
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
	if currentCol != 2 {
		t.Errorf("currentCol = %d, want 2", currentCol)
	}
	if got := string(clipboard.lines[0]); got != "o b" {
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
	if mode != VisualMode || visualLine {
		t.Fatalf("v gave mode %v, linewise %v", mode, visualLine)
	}

	press(t, "V")
	if mode != VisualMode || !visualLine {
		t.Fatalf("V gave mode %v, linewise %v", mode, visualLine)
	}

	press(t, "V")
	if mode != ReadMode {
		t.Errorf("V again gave mode %v, want ReadMode", mode)
	}

	press(t, "vv")
	if mode != ReadMode {
		t.Errorf("vv gave mode %v, want ReadMode", mode)
	}
}

func TestEscLeavesVisualMode(t *testing.T) {
	b := inReadMode(t, "abc\n", 0, 0)

	press(t, "vl")
	esc()

	if mode != ReadMode {
		t.Errorf("mode = %v, want ReadMode", mode)
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
	if mode != EditMode {
		t.Errorf("mode = %v, want EditMode", mode)
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

func TestSelectionCovers(t *testing.T) {
	charwise := selection{startRow: 0, startCol: 1, endRow: 1, endCol: 2, active: true}
	oneLine := selection{startRow: 0, startCol: 1, endRow: 0, endCol: 2, active: true}
	byLine := selection{startRow: 0, startCol: 3, endRow: 0, endCol: 1, linewise: true, active: true}

	cases := []struct {
		name     string
		sel      selection
		row, col int
		lineLen  int
		want     bool
	}{
		{"before the start", charwise, 0, 0, 3, false},
		{"at the start", charwise, 0, 1, 3, true},
		{"the line break it runs over", charwise, 0, 3, 3, true},
		{"past the line break", charwise, 0, 4, 3, false},
		{"the head of its last line", charwise, 1, 0, 3, true},
		{"at its end", charwise, 1, 2, 3, true},
		{"past its end", charwise, 1, 3, 3, false},
		{"a row below it", charwise, 2, 0, 3, false},
		{"a run inside one line", oneLine, 0, 2, 3, true},
		{"the line break it stops before", oneLine, 0, 3, 3, false},
		{"a line taken whole", byLine, 0, 0, 3, true},
		{"the break of a line taken whole", byLine, 0, 3, 3, true},
		{"an empty line taken whole", byLine, 0, 0, 0, true},
		{"no selection at all", selection{}, 0, 0, 3, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.sel.covers(tc.row, tc.col, tc.lineLen); got != tc.want {
				t.Errorf("covers(%d, %d, %d) = %v, want %v", tc.row, tc.col, tc.lineLen, got, tc.want)
			}
		})
	}
}
