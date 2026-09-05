package editor

import (
	"testing"

	"github.com/ArditZubaku/tex/internal/editor/edit"
	"github.com/ArditZubaku/tex/internal/editor/state"
)

func TestReadModeStopsOnTheLastRune(t *testing.T) {
	atCursor(t, "package main\n", 0, 0)
	ed.Mode = state.ReadMode

	for range 20 {
		ed.Right()
	}

	if ed.Row != 0 || ed.Col != 11 {
		t.Errorf("cursor at %d,%d, want 0,11", ed.Row, ed.Col)
	}
}

func TestEditModeReachesTheGapAfterTheLastRune(t *testing.T) {
	atCursor(t, "package main\n", 0, 0)
	ed.Mode = state.EditMode

	for range 20 {
		ed.Right()
	}

	if ed.Row != 0 || ed.Col != 12 {
		t.Errorf("cursor at %d,%d, want 0,12", ed.Row, ed.Col)
	}
}

func TestEmptyLineClampsToColumnZero(t *testing.T) {
	atCursor(t, "package main\n\n", 0, 11)
	ed.Mode = state.ReadMode

	ed.Down()
	ed.ClampCol()

	if ed.Row != 1 || ed.Col != 0 {
		t.Errorf("cursor at %d,%d, want 1,0", ed.Row, ed.Col)
	}
}

func TestEscStepsOffTheGap(t *testing.T) {
	atCursor(t, "package main\n", 0, 12)
	ed.Mode = state.EditMode

	esc()

	if ed.Mode != state.ReadMode || ed.Col != 11 {
		t.Errorf("mode %v, currentCol = %d, want ReadMode, 11", ed.Mode, ed.Col)
	}
}

func TestReadModeDoesNotWrapBetweenLines(t *testing.T) {
	atCursor(t, "package main\nfoo\n", 1, 0)
	ed.Mode = state.ReadMode

	ed.Left()
	if ed.Row != 1 || ed.Col != 0 {
		t.Errorf("left: cursor at %d,%d, want 1,0", ed.Row, ed.Col)
	}

	ed.Row, ed.Col = 0, 11
	ed.Right()
	if ed.Row != 0 || ed.Col != 11 {
		t.Errorf("right: cursor at %d,%d, want 0,11", ed.Row, ed.Col)
	}
}

func TestEditModeWrapsBetweenLines(t *testing.T) {
	atCursor(t, "package main\nfoo\n", 1, 0)
	ed.Mode = state.EditMode

	ed.Left()
	if ed.Row != 0 || ed.Col != 12 {
		t.Errorf("left: cursor at %d,%d, want 0,12", ed.Row, ed.Col)
	}

	ed.Right()
	if ed.Row != 1 || ed.Col != 0 {
		t.Errorf("right: cursor at %d,%d, want 1,0", ed.Row, ed.Col)
	}
}

func TestDeletingTheLastRunePullsTheCursorBack(t *testing.T) {
	b := atCursor(t, "abc\n", 0, 2)
	ed.Mode = state.ReadMode

	edit.DeleteRune(ed)
	ed.ClampCol()

	wantLines(t, b, "ab")
	if ed.Col != 1 {
		t.Errorf("currentCol = %d, want 1", ed.Col)
	}
}
