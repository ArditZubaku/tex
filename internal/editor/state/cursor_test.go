// The tests sit outside the package because the harness they share with the
// rest of the editor imports it.
package state_test

import (
	"strings"
	"testing"

	"github.com/ArditZubaku/tex/internal/editor/edtest"
	"github.com/ArditZubaku/tex/internal/editor/state"
)

func TestReadModeStopsOnTheLastRune(t *testing.T) {
	e := state.New()
	edtest.AtCursor(t, e, "package main\n", 0, 0)
	e.Mode = state.ReadMode

	for range 20 {
		e.Right()
	}

	if e.Row != 0 || e.Col != 11 {
		t.Errorf("cursor at %d,%d, want 0,11", e.Row, e.Col)
	}
}

func TestEditModeReachesTheGapAfterTheLastRune(t *testing.T) {
	e := state.New()
	edtest.AtCursor(t, e, "package main\n", 0, 0)
	e.Mode = state.EditMode

	for range 20 {
		e.Right()
	}

	if e.Row != 0 || e.Col != 12 {
		t.Errorf("cursor at %d,%d, want 0,12", e.Row, e.Col)
	}
}

func TestEmptyLineClampsToColumnZero(t *testing.T) {
	e := state.New()
	edtest.AtCursor(t, e, "package main\n\n", 0, 11)
	e.Mode = state.ReadMode

	e.Down()
	e.ClampCol()

	if e.Row != 1 || e.Col != 0 {
		t.Errorf("cursor at %d,%d, want 1,0", e.Row, e.Col)
	}
}

func TestReadModeDoesNotWrapBetweenLines(t *testing.T) {
	e := state.New()
	edtest.AtCursor(t, e, "package main\nfoo\n", 1, 0)
	e.Mode = state.ReadMode

	e.Left()
	if e.Row != 1 || e.Col != 0 {
		t.Errorf("left: cursor at %d,%d, want 1,0", e.Row, e.Col)
	}

	e.Row, e.Col = 0, 11
	e.Right()
	if e.Row != 0 || e.Col != 11 {
		t.Errorf("right: cursor at %d,%d, want 0,11", e.Row, e.Col)
	}
}

func TestEditModeWrapsBetweenLines(t *testing.T) {
	e := state.New()
	edtest.AtCursor(t, e, "package main\nfoo\n", 1, 0)
	e.Mode = state.EditMode

	e.Left()
	if e.Row != 0 || e.Col != 12 {
		t.Errorf("left: cursor at %d,%d, want 0,12", e.Row, e.Col)
	}

	e.Right()
	if e.Row != 1 || e.Col != 0 {
		t.Errorf("right: cursor at %d,%d, want 1,0", e.Row, e.Col)
	}
}

func TestCentringOnAJumpDoesNotReadTheCountThatRanIt(t *testing.T) {
	e := state.New()
	edtest.InReadMode(t, e, strings.Repeat("x\n", 200), 0, 0)
	edtest.SingleWindow(e, 20, 80)

	// what '3gd' leaves the editor in the middle of: the third jump has landed
	// off screen, and the count that ran all three is still set
	e.Row, e.OffsetRow = 150, 0
	e.CmdCount, e.HadCount = 3, true

	e.CenterIfOffScreen()

	if e.Row != 150 {
		t.Errorf("centring moved the cursor to line %d, want it left on 150", e.Row)
	}
	if e.OffsetRow != 140 {
		t.Errorf("window starts at line %d, want 140", e.OffsetRow)
	}
}

func TestCentringOnItsOwnStillReadsTheCount(t *testing.T) {
	e := state.New()
	edtest.InReadMode(t, e, strings.Repeat("x\n", 200), 0, 0)
	edtest.SingleWindow(e, 20, 80)

	e.Row, e.CmdCount, e.HadCount = 150, 3, true
	e.CenterView()

	if e.Row != 2 {
		t.Errorf("'3zz' put the cursor on line %d, want 2", e.Row)
	}
}
