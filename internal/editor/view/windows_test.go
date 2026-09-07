package view_test

import (
	"path/filepath"
	"testing"

	"github.com/nsf/termbox-go"

	"github.com/ArditZubaku/tex/internal/editor/edtest"
	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/ArditZubaku/tex/internal/editor/view"
)

func wantWindowCount(t *testing.T, e *state.Editor, want int) {
	t.Helper()

	if got := len(view.List(e)); got != want {
		t.Fatalf("%d windows, want %d", got, want)
	}
}

func wantRect(t *testing.T, w *view.Window, row, col, rows, cols int) {
	t.Helper()

	if w.Rect.Row != row || w.Rect.Col != col || w.Rect.Rows != rows || w.Rect.Cols != cols {
		t.Errorf("window at %d,%d %dx%d, want %d,%d %dx%d",
			w.Rect.Row, w.Rect.Col, w.Rect.Rows, w.Rect.Cols, row, col, rows, cols)
	}
}

func inWindows(t *testing.T, e *state.Editor, names ...string) []string {
	t.Helper()

	paths := inBuffers(t, e, names...)
	edtest.SingleWindow(e, 20, 80)

	return paths
}

func TestSplitStacksTwoWindowsAndTakesTheLowerOne(t *testing.T) {
	e := state.New()

	inWindows(t, e)

	edtest.Press(t, e, ":sp\n")

	wantWindowCount(t, e, 2)
	list := view.List(e)
	wantRect(t, list[0], state.TabBarRows, 0, 10, 80)
	wantRect(t, list[1], state.TabBarRows+11, 0, 9, 80)
	if view.Focused() != list[1] {
		t.Error("the cursor stayed above the split")
	}
	if len(view.Separators()) != 1 || view.Separators()[0].Vertical {
		t.Errorf("separators = %v, want one horizontal", view.Separators())
	}
}

func TestVerticalSplitPutsTheWindowsSideBySideAndTakesTheRightOne(t *testing.T) {
	e := state.New()

	inWindows(t, e)

	edtest.Press(t, e, ":vs\n")

	wantWindowCount(t, e, 2)
	list := view.List(e)
	wantRect(t, list[0], state.TabBarRows, 0, 20, 40)
	wantRect(t, list[1], state.TabBarRows, 41, 20, 39)
	if view.Focused() != list[1] {
		t.Error("the cursor stayed left of the split")
	}
	if len(view.Separators()) != 1 || !view.Separators()[0].Vertical {
		t.Errorf("separators = %v, want one vertical", view.Separators())
	}
}

func TestSplittingAgainTheSameWayShareTheRoomEqually(t *testing.T) {
	e := state.New()

	inWindows(t, e)
	edtest.SingleWindow(e, 20, 130)

	edtest.Press(t, e, ":vs\n")
	edtest.Press(t, e, ":vs\n")

	wantWindowCount(t, e, 3)
	for _, w := range view.List(e) {
		if w.Rect.Cols < 42 || w.Rect.Cols > 43 {
			t.Errorf("window %d columns wide, want a third of the 128 left by two separators", w.Rect.Cols)
		}
	}
}

func TestSplittingTheOtherWayNestsInsideTheWindowSplit(t *testing.T) {
	e := state.New()

	inWindows(t, e)

	edtest.Press(t, e, ":vs\n")
	edtest.Press(t, e, ":sp\n")

	wantWindowCount(t, e, 3)
	list := view.List(e)
	wantRect(t, list[0], state.TabBarRows, 0, 20, 40)  // the left window, untouched
	wantRect(t, list[1], state.TabBarRows, 41, 10, 39) // the right one, split in two
	wantRect(t, list[2], state.TabBarRows+11, 41, 9, 39)
}

func TestASplitShowsTheSameBufferAtTheSamePlace(t *testing.T) {
	e := state.New()

	inWindows(t, e)

	edtest.Press(t, e, "jl")
	edtest.Press(t, e, ":sp\n")

	list := view.List(e)
	if list[0].Entry != list[1].Entry {
		t.Error("the split window shows another buffer")
	}
	if list[1].CursorRow != list[0].CursorRow || list[1].CursorCol != list[0].CursorCol {
		t.Errorf("split cursor at %d,%d, want %d,%d",
			list[1].CursorRow, list[1].CursorCol, list[0].CursorRow, list[0].CursorCol)
	}
}

func TestEachWindowKeepsItsOwnCursor(t *testing.T) {
	e := state.New()

	inWindows(t, e)

	edtest.Press(t, e, ":sp\n")
	edtest.Press(t, e, "l") // only the lower window moves
	edtest.PressKey(t, e, termbox.KeyCtrlK)

	edtest.WantCursor(t, e, 0, 0)

	edtest.PressKey(t, e, termbox.KeyCtrlJ)

	edtest.WantCursor(t, e, 0, 1)
}

func TestTheLayoutPassLeavesTheCursorWhereItIs(t *testing.T) {
	e := state.New()

	inWindows(t, e)

	edtest.Press(t, e, "l")
	view.Layout(e) // which every redraw runs before anything is drawn

	edtest.WantCursor(t, e, 0, 1)
}

func TestCtrlJAndCtrlKMoveBetweenStackedWindows(t *testing.T) {
	e := state.New()

	inWindows(t, e)

	edtest.Press(t, e, ":sp\n")
	below := view.Focused()

	edtest.PressKey(t, e, termbox.KeyCtrlK)
	if view.Focused() == below {
		t.Fatal("Ctrl-K stayed in the lower window")
	}

	edtest.PressKey(t, e, termbox.KeyCtrlJ)
	if view.Focused() != below {
		t.Error("Ctrl-J did not come back down")
	}
}

func TestCtrlHAndCtrlLMoveBetweenSideBySideWindows(t *testing.T) {
	e := state.New()

	inWindows(t, e)

	edtest.Press(t, e, ":vs\n")
	right := view.Focused()

	edtest.PressKey(t, e, termbox.KeyCtrlH)
	if view.Focused() == right {
		t.Fatal("Ctrl-H stayed in the right window")
	}

	edtest.PressKey(t, e, termbox.KeyCtrlL)
	if view.Focused() != right {
		t.Error("Ctrl-L did not come back right")
	}
}

func TestAMoveWithNoWindowThatWayStaysPut(t *testing.T) {
	e := state.New()

	inWindows(t, e)

	edtest.Press(t, e, ":sp\n")
	below := view.Focused()

	edtest.PressKey(t, e, termbox.KeyCtrlJ)
	edtest.PressKey(t, e, termbox.KeyCtrlH)

	if view.Focused() != below {
		t.Error("a move with no window that way left the window")
	}
}

func TestCtrlHIsStillBackspaceWhileTyping(t *testing.T) {
	e := state.New()

	inWindows(t, e)

	edtest.Press(t, e, ":vs\n")
	right := view.Focused()
	edtest.Press(t, e, "lli")
	edtest.PressKey(t, e, termbox.KeyCtrlH)

	if view.Focused() != right {
		t.Error("Ctrl-H left the window while typing")
	}
	edtest.WantLines(t, e.Buf, "frst")
}

func TestClosingAWindowGivesItsRoomBack(t *testing.T) {
	e := state.New()

	inWindows(t, e)

	edtest.Press(t, e, ":sp\n")
	edtest.Press(t, e, ":close\n")

	wantWindowCount(t, e, 1)
	wantRect(t, view.List(e)[0], state.TabBarRows, 0, 20, 80)
	if len(view.Separators()) != 0 {
		t.Errorf("separators = %v, want none", view.Separators())
	}
}

func TestTheLastWindowCannotBeClosed(t *testing.T) {
	e := state.New()

	inWindows(t, e)

	edtest.Press(t, e, ":close\n")

	wantWindowCount(t, e, 1)
	if e.StatusMsg != "E444: Cannot close last window" {
		t.Errorf("statusMsg = %q", e.StatusMsg)
	}
}

func TestOnlyClosesEveryOtherWindow(t *testing.T) {
	e := state.New()

	inWindows(t, e)

	edtest.Press(t, e, ":sp\n")
	edtest.Press(t, e, ":vs\n")
	kept := view.Focused()
	edtest.Press(t, e, ":only\n")

	wantWindowCount(t, e, 1)
	if view.Focused() != kept {
		t.Error("only left another window than the one it was run in")
	}
	wantRect(t, view.Focused(), state.TabBarRows, 0, 20, 80)
}

func TestQuitClosesTheWindowUntilItIsTheLastOne(t *testing.T) {
	e := state.New()

	inWindows(t, e)

	edtest.Press(t, e, ":sp\n")
	edtest.Press(t, e, ":q\n")

	wantWindowCount(t, e, 1)
	if e.Quitting {
		t.Fatal("quit the editor while a window was left")
	}

	edtest.Press(t, e, ":q\n")

	if !e.Quitting {
		t.Error("still running after :q in the last window")
	}
}

func TestSplitWithANameOpensTheFileInTheNewWindowAlone(t *testing.T) {
	e := state.New()

	paths := inWindows(t, e, "a.txt")

	edtest.Press(t, e, ":sp "+paths[0]+"\n")

	list := view.List(e)
	if got := filepath.Base(list[1].Entry.Path); got != "a.txt" {
		t.Errorf("new window shows %q, want a.txt", got)
	}
	if got := filepath.Base(list[0].Entry.Path); got != "f.txt" {
		t.Errorf("old window shows %q, want f.txt", got)
	}
}

func TestSplitRefusesWhenThereIsNoRoomForIt(t *testing.T) {
	e := state.New()

	inWindows(t, e)
	edtest.SingleWindow(e, 4, 30)

	edtest.Press(t, e, ":vs\n")

	wantWindowCount(t, e, 1)
	if e.StatusMsg != "E36: Not enough room" {
		t.Errorf("statusMsg = %q", e.StatusMsg)
	}

	edtest.Press(t, e, ":sp\n")

	wantWindowCount(t, e, 1)
	if e.StatusMsg != "E36: Not enough room" {
		t.Errorf("statusMsg = %q", e.StatusMsg)
	}
}

func TestLeaderSplitsAndClosesWindows(t *testing.T) {
	e := state.New()

	inWindows(t, e)

	edtest.Press(t, e, " sv")
	wantWindowCount(t, e, 2)
	if view.Separators()[0].Vertical {
		t.Error("<leader>sv put the windows side by side")
	}

	edtest.Press(t, e, " sh")
	wantWindowCount(t, e, 3)
	if !view.Separators()[len(view.Separators())-1].Vertical {
		t.Error("<leader>sh stacked the windows")
	}

	edtest.Press(t, e, " wd")
	wantWindowCount(t, e, 2)

	edtest.Press(t, e, " wd")
	wantWindowCount(t, e, 1)
}

func TestCtrlHLeavesTheExplorerForTheWindowBeside(t *testing.T) {
	e := state.New()

	inWindows(t, e)

	edtest.Press(t, e, ":vs\n")
	right := view.Focused()
	edtest.Press(t, e, " e")
	edtest.PressKey(t, e, termbox.KeyCtrlH)

	if view.Focused() == right {
		t.Fatal("Ctrl-H stayed in the explorer's window")
	}
	if e.ExplorerOpen || e.Mode != state.ReadMode {
		t.Error("the explorer came along to the window moved to")
	}
}

func TestAMoveOutOfTheExplorerWithNoWindowThatWayKeepsIt(t *testing.T) {
	e := state.New()

	inWindows(t, e)

	edtest.Press(t, e, " e")
	edtest.PressKey(t, e, termbox.KeyCtrlL)

	if !e.ExplorerOpen || e.Mode != state.ExplorerMode {
		t.Error("the explorer closed on a move that had nowhere to go")
	}
}

func TestClosingABufferLeavesNoWindowShowingIt(t *testing.T) {
	e := state.New()

	paths := inWindows(t, e, "a.txt")

	edtest.Press(t, e, ":sp "+paths[0]+"\n")
	edtest.Press(t, e, ":bd\n")

	for _, w := range view.List(e) {
		if got := filepath.Base(w.Entry.Path); got != "f.txt" {
			t.Errorf("window still shows %q", got)
		}
	}
}

func TestADraggedLayoutKeepsItsProportionsWhenTheScreenGrows(t *testing.T) {
	e := state.New()

	inWindows(t, e)

	edtest.Press(t, e, ":sp\n")
	edtest.PressMouse(t, e, state.TabBarRows+10, 0)
	edtest.DragMouse(t, e, state.TabBarRows+9, 0)
	e.ScreenRows = 40
	view.Layout(e)

	list := view.List(e)
	wantRect(t, list[0], state.TabBarRows, 0, 19, 80)
	wantRect(t, list[1], state.TabBarRows+20, 0, 20, 80)
}

func TestDraggingTheLineBetweenTwoWindowsMovesIt(t *testing.T) {
	e := state.New()

	inWindows(t, e)

	edtest.Press(t, e, ":sp\n")
	edtest.PressMouse(t, e, state.TabBarRows+10, 4) // the separator the split drew
	edtest.DragMouse(t, e, state.TabBarRows+14, 4)
	edtest.ReleaseMouse(t, e, state.TabBarRows+14, 4)

	list := view.List(e)
	wantRect(t, list[0], state.TabBarRows, 0, 14, 80)
	wantRect(t, list[1], state.TabBarRows+15, 0, 5, 80)
}

func TestDraggingAVerticalLineMovesItSidewaysOnly(t *testing.T) {
	e := state.New()

	inWindows(t, e)

	edtest.Press(t, e, ":vs\n")
	edtest.PressMouse(t, e, state.TabBarRows+3, 40)
	edtest.DragMouse(t, e, state.TabBarRows+9, 30) // the row it wanders onto means nothing

	list := view.List(e)
	wantRect(t, list[0], state.TabBarRows, 0, 20, 30)
	wantRect(t, list[1], state.TabBarRows, 31, 20, 49)
}

func TestADragStopsWhereTheWindowItTakesFromRunsOut(t *testing.T) {
	e := state.New()

	inWindows(t, e)

	edtest.Press(t, e, ":sp\n")
	edtest.PressMouse(t, e, state.TabBarRows+10, 0)
	edtest.DragMouse(t, e, state.TabBarRows+40, 0) // far past the foot of the screen
	edtest.DragMouse(t, e, state.TabBarRows+10, 0) // and back to where it was taken

	list := view.List(e)
	wantRect(t, list[0], state.TabBarRows, 0, 10, 80)
	wantRect(t, list[1], state.TabBarRows+11, 0, 9, 80)
}

func TestPressingInsideAWindowStartsNoDrag(t *testing.T) {
	e := state.New()

	inWindows(t, e)

	edtest.Press(t, e, ":sp\n")
	edtest.PressMouse(t, e, state.TabBarRows+4, 10) // in the upper window, not on its edge
	edtest.DragMouse(t, e, state.TabBarRows+14, 10)

	list := view.List(e)
	wantRect(t, list[0], state.TabBarRows, 0, 10, 80)
	wantRect(t, list[1], state.TabBarRows+11, 0, 9, 80)
}

func TestLettingGoEndsTheDrag(t *testing.T) {
	e := state.New()

	inWindows(t, e)

	edtest.Press(t, e, ":sp\n")
	edtest.PressMouse(t, e, state.TabBarRows+10, 0)
	edtest.ReleaseMouse(t, e, state.TabBarRows+10, 0)
	edtest.DragMouse(t, e, state.TabBarRows+14, 0)

	list := view.List(e)
	wantRect(t, list[0], state.TabBarRows, 0, 10, 80)
	wantRect(t, list[1], state.TabBarRows+11, 0, 9, 80)
}

func TestDraggingTheLineOfANestedSplitMovesThatOneAlone(t *testing.T) {
	e := state.New()

	inWindows(t, e)

	edtest.Press(t, e, ":vs\n")
	edtest.Press(t, e, ":sp\n")
	edtest.PressMouse(t, e, state.TabBarRows+10, 50) // the line inside the right column
	edtest.DragMouse(t, e, state.TabBarRows+5, 50)

	list := view.List(e)
	wantRect(t, list[0], state.TabBarRows, 0, 20, 40)
	wantRect(t, list[1], state.TabBarRows, 41, 5, 39)
	wantRect(t, list[2], state.TabBarRows+6, 41, 14, 39)
}
