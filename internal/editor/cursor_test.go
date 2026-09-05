package editor

import "testing"

func TestReadModeStopsOnTheLastRune(t *testing.T) {
	atCursor(t, "package main\n", 0, 0)
	mode = ReadMode

	for range 20 {
		right()
	}

	if currentRow != 0 || currentCol != 11 {
		t.Errorf("cursor at %d,%d, want 0,11", currentRow, currentCol)
	}
}

func TestEditModeReachesTheGapAfterTheLastRune(t *testing.T) {
	atCursor(t, "package main\n", 0, 0)
	mode = EditMode

	for range 20 {
		right()
	}

	if currentRow != 0 || currentCol != 12 {
		t.Errorf("cursor at %d,%d, want 0,12", currentRow, currentCol)
	}
}

func TestEmptyLineClampsToColumnZero(t *testing.T) {
	atCursor(t, "package main\n\n", 0, 11)
	mode = ReadMode

	down()
	clampCol()

	if currentRow != 1 || currentCol != 0 {
		t.Errorf("cursor at %d,%d, want 1,0", currentRow, currentCol)
	}
}

func TestEscStepsOffTheGap(t *testing.T) {
	atCursor(t, "package main\n", 0, 12)
	mode = EditMode

	esc()

	if mode != ReadMode || currentCol != 11 {
		t.Errorf("mode %v, currentCol = %d, want ReadMode, 11", mode, currentCol)
	}
}

func TestReadModeDoesNotWrapBetweenLines(t *testing.T) {
	atCursor(t, "package main\nfoo\n", 1, 0)
	mode = ReadMode

	left()
	if currentRow != 1 || currentCol != 0 {
		t.Errorf("left: cursor at %d,%d, want 1,0", currentRow, currentCol)
	}

	currentRow, currentCol = 0, 11
	right()
	if currentRow != 0 || currentCol != 11 {
		t.Errorf("right: cursor at %d,%d, want 0,11", currentRow, currentCol)
	}
}

func TestEditModeWrapsBetweenLines(t *testing.T) {
	atCursor(t, "package main\nfoo\n", 1, 0)
	mode = EditMode

	left()
	if currentRow != 0 || currentCol != 12 {
		t.Errorf("left: cursor at %d,%d, want 0,12", currentRow, currentCol)
	}

	right()
	if currentRow != 1 || currentCol != 0 {
		t.Errorf("right: cursor at %d,%d, want 1,0", currentRow, currentCol)
	}
}

func TestDeletingTheLastRunePullsTheCursorBack(t *testing.T) {
	b := atCursor(t, "abc\n", 0, 2)
	mode = ReadMode

	deleteRune()
	clampCol()

	wantLines(t, b, "ab")
	if currentCol != 1 {
		t.Errorf("currentCol = %d, want 1", currentCol)
	}
}
