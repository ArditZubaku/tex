package editor

import (
	"strings"
	"testing"
)

func inWindow(t *testing.T, lines, row int) {
	t.Helper()

	inReadMode(t, strings.Repeat("x\n", lines), row, 0)
	singleWindow(20, 80)
	ed.OffsetRow, ed.OffsetCol = 0, 0
}

func TestCenterViewPutsTheCursorLineInTheMiddle(t *testing.T) {
	inWindow(t, 100, 50)

	press(t, "zz")

	if ed.OffsetRow != 40 {
		t.Errorf("offsetRow = %d, want 40", ed.OffsetRow)
	}
	if ed.Row != 50 {
		t.Errorf("currentRow = %d, want 50", ed.Row)
	}
}

func TestCenterViewNearTheTopStopsAtTheFirstLine(t *testing.T) {
	inWindow(t, 100, 3)

	press(t, "zz")

	if ed.OffsetRow != 0 {
		t.Errorf("offsetRow = %d, want 0", ed.OffsetRow)
	}
}

func TestCenterViewNearTheEndScrollsPastTheLastLine(t *testing.T) {
	inWindow(t, 100, 99)

	press(t, "zz")

	if ed.OffsetRow != 89 {
		t.Errorf("offsetRow = %d, want 89", ed.OffsetRow)
	}
}

func TestCountedCenterViewJumpsToThatLine(t *testing.T) {
	inWindow(t, 100, 0)

	press(t, "40zz")

	if ed.Row != 39 {
		t.Errorf("currentRow = %d, want 39", ed.Row)
	}
	if ed.OffsetRow != 29 {
		t.Errorf("offsetRow = %d, want 29", ed.OffsetRow)
	}
}

func TestCountedCenterViewStopsAtTheLastLine(t *testing.T) {
	inWindow(t, 10, 0)

	press(t, "99zz")

	if ed.Row != 9 {
		t.Errorf("currentRow = %d, want 9", ed.Row)
	}
}

func TestCenterViewKeepsTheColumn(t *testing.T) {
	inReadMode(t, "foo bar\nbaz\n", 0, 5)
	singleWindow(20, 80)
	ed.OffsetRow = 0

	press(t, "zz")

	if ed.Col != 5 {
		t.Errorf("currentCol = %d, want 5", ed.Col)
	}
}

func TestCenterViewIsNotUndoable(t *testing.T) {
	inWindow(t, 100, 50)

	press(t, "zz")

	if ed.Hist.CanUndo() {
		t.Error("scrolling recorded a change")
	}
}

func TestScrollingFollowsTheCursorAwayFromACenteredView(t *testing.T) {
	inWindow(t, 100, 50)

	press(t, "zz")
	scrollTextBuffer()
	if ed.OffsetRow != 40 {
		t.Fatalf("offsetRow = %d, want the centered 40 left alone", ed.OffsetRow)
	}

	press(t, "30j")
	scrollTextBuffer()
	if ed.OffsetRow != 61 {
		t.Errorf("offsetRow = %d, want 61", ed.OffsetRow)
	}
}
