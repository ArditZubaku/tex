package editor

import (
	"strings"
	"testing"
)

func inWindow(t *testing.T, lines, row int) {
	t.Helper()

	inReadMode(t, strings.Repeat("x\n", lines), row, 0)
	singleWindow(20, 80)
	offsetRow, offsetCol = 0, 0
}

func TestCenterViewPutsTheCursorLineInTheMiddle(t *testing.T) {
	inWindow(t, 100, 50)

	press(t, "zz")

	if offsetRow != 40 {
		t.Errorf("offsetRow = %d, want 40", offsetRow)
	}
	if currentRow != 50 {
		t.Errorf("currentRow = %d, want 50", currentRow)
	}
}

func TestCenterViewNearTheTopStopsAtTheFirstLine(t *testing.T) {
	inWindow(t, 100, 3)

	press(t, "zz")

	if offsetRow != 0 {
		t.Errorf("offsetRow = %d, want 0", offsetRow)
	}
}

func TestCenterViewNearTheEndScrollsPastTheLastLine(t *testing.T) {
	inWindow(t, 100, 99)

	press(t, "zz")

	if offsetRow != 89 {
		t.Errorf("offsetRow = %d, want 89", offsetRow)
	}
}

func TestCountedCenterViewJumpsToThatLine(t *testing.T) {
	inWindow(t, 100, 0)

	press(t, "40zz")

	if currentRow != 39 {
		t.Errorf("currentRow = %d, want 39", currentRow)
	}
	if offsetRow != 29 {
		t.Errorf("offsetRow = %d, want 29", offsetRow)
	}
}

func TestCountedCenterViewStopsAtTheLastLine(t *testing.T) {
	inWindow(t, 10, 0)

	press(t, "99zz")

	if currentRow != 9 {
		t.Errorf("currentRow = %d, want 9", currentRow)
	}
}

func TestCenterViewKeepsTheColumn(t *testing.T) {
	inReadMode(t, "foo bar\nbaz\n", 0, 5)
	singleWindow(20, 80)
	offsetRow = 0

	press(t, "zz")

	if currentCol != 5 {
		t.Errorf("currentCol = %d, want 5", currentCol)
	}
}

func TestCenterViewIsNotUndoable(t *testing.T) {
	inWindow(t, 100, 50)

	press(t, "zz")

	if len(undoStack) != 0 {
		t.Errorf("undoStack holds %d changes, want none", len(undoStack))
	}
}

func TestScrollingFollowsTheCursorAwayFromACenteredView(t *testing.T) {
	inWindow(t, 100, 50)

	press(t, "zz")
	scrollTextBuffer()
	if offsetRow != 40 {
		t.Fatalf("offsetRow = %d, want the centered 40 left alone", offsetRow)
	}

	press(t, "30j")
	scrollTextBuffer()
	if offsetRow != 61 {
		t.Errorf("offsetRow = %d, want 61", offsetRow)
	}
}
