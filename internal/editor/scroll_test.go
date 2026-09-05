package editor

import (
	"strings"
	"testing"

	"github.com/ArditZubaku/tex/internal/editor/edtest"
	"github.com/ArditZubaku/tex/internal/editor/render"
	"github.com/ArditZubaku/tex/internal/editor/state"
)

func inWindow(t *testing.T, e *state.Editor, lines, row int) {
	t.Helper()

	edtest.InReadMode(t, e, strings.Repeat("x\n", lines), row, 0)
	edtest.SingleWindow(e, 20, 80)
	e.OffsetRow, e.OffsetCol = 0, 0
}

func TestCenterViewPutsTheCursorLineInTheMiddle(t *testing.T) {
	e := state.New()

	inWindow(t, e, 100, 50)

	edtest.Press(t, e, "zz")

	if e.OffsetRow != 40 {
		t.Errorf("offsetRow = %d, want 40", e.OffsetRow)
	}
	if e.Row != 50 {
		t.Errorf("currentRow = %d, want 50", e.Row)
	}
}

func TestCenterViewNearTheTopStopsAtTheFirstLine(t *testing.T) {
	e := state.New()

	inWindow(t, e, 100, 3)

	edtest.Press(t, e, "zz")

	if e.OffsetRow != 0 {
		t.Errorf("offsetRow = %d, want 0", e.OffsetRow)
	}
}

func TestCenterViewNearTheEndScrollsPastTheLastLine(t *testing.T) {
	e := state.New()

	inWindow(t, e, 100, 99)

	edtest.Press(t, e, "zz")

	if e.OffsetRow != 89 {
		t.Errorf("offsetRow = %d, want 89", e.OffsetRow)
	}
}

func TestCountedCenterViewJumpsToThatLine(t *testing.T) {
	e := state.New()

	inWindow(t, e, 100, 0)

	edtest.Press(t, e, "40zz")

	if e.Row != 39 {
		t.Errorf("currentRow = %d, want 39", e.Row)
	}
	if e.OffsetRow != 29 {
		t.Errorf("offsetRow = %d, want 29", e.OffsetRow)
	}
}

func TestCountedCenterViewStopsAtTheLastLine(t *testing.T) {
	e := state.New()

	inWindow(t, e, 10, 0)

	edtest.Press(t, e, "99zz")

	if e.Row != 9 {
		t.Errorf("currentRow = %d, want 9", e.Row)
	}
}

func TestCenterViewKeepsTheColumn(t *testing.T) {
	e := state.New()

	edtest.InReadMode(t, e, "foo bar\nbaz\n", 0, 5)
	edtest.SingleWindow(e, 20, 80)
	e.OffsetRow = 0

	edtest.Press(t, e, "zz")

	if e.Col != 5 {
		t.Errorf("currentCol = %d, want 5", e.Col)
	}
}

func TestCenterViewIsNotUndoable(t *testing.T) {
	e := state.New()

	inWindow(t, e, 100, 50)

	edtest.Press(t, e, "zz")

	if e.Hist.CanUndo() {
		t.Error("scrolling recorded a change")
	}
}

func TestScrollingFollowsTheCursorAwayFromACenteredView(t *testing.T) {
	e := state.New()

	inWindow(t, e, 100, 50)

	edtest.Press(t, e, "zz")
	render.Scroll(e)
	if e.OffsetRow != 40 {
		t.Fatalf("offsetRow = %d, want the centered 40 left alone", e.OffsetRow)
	}

	edtest.Press(t, e, "30j")
	render.Scroll(e)
	if e.OffsetRow != 61 {
		t.Errorf("offsetRow = %d, want 61", e.OffsetRow)
	}
}
