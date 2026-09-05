package editor

import (
	"path/filepath"
	"testing"

	"github.com/ArditZubaku/tex/internal/editor/edtest"
	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/ArditZubaku/tex/internal/editor/view"
	"github.com/nsf/termbox-go"
)

// singleWindow is the state the editor's own layout pass leaves behind for one
// unsplit window filling the screen, which is what every test that draws or
// scrolls assumes it starts from.
func singleWindow(rows, cols int) {
	view.Reset()
	ed.ScreenRows, ed.ScreenCols = rows, cols
	ed.Rows, ed.Cols = rows, cols
	ed.WinRow, ed.WinCol = state.TabBarRows, 0
}

// The window moves are Ctrl-hjkl, which arrive as keys rather than runes and so
// take the path every special key does.
func pressKey(t *testing.T, key termbox.Key) {
	t.Helper()

	dispatchKey(termbox.Event{Key: key})
}

func wantWindowCount(t *testing.T, want int) {
	t.Helper()

	if got := len(view.List(ed)); got != want {
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

func inWindows(t *testing.T, names ...string) []string {
	t.Helper()

	paths := inBuffers(t, names...)
	singleWindow(20, 80)

	return paths
}

func TestSplitStacksTwoWindowsAndTakesTheLowerOne(t *testing.T) {
	inWindows(t)

	press(t, ":sp\n")

	wantWindowCount(t, 2)
	list := view.List(ed)
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
	inWindows(t)

	press(t, ":vs\n")

	wantWindowCount(t, 2)
	list := view.List(ed)
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
	inWindows(t)
	singleWindow(20, 130)

	press(t, ":vs\n")
	press(t, ":vs\n")

	wantWindowCount(t, 3)
	for _, w := range view.List(ed) {
		if w.Rect.Cols < 42 || w.Rect.Cols > 43 {
			t.Errorf("window %d columns wide, want a third of the 128 left by two separators", w.Rect.Cols)
		}
	}
}

func TestSplittingTheOtherWayNestsInsideTheWindowSplit(t *testing.T) {
	inWindows(t)

	press(t, ":vs\n")
	press(t, ":sp\n")

	wantWindowCount(t, 3)
	list := view.List(ed)
	wantRect(t, list[0], state.TabBarRows, 0, 20, 40)  // the left window, untouched
	wantRect(t, list[1], state.TabBarRows, 41, 10, 39) // the right one, split in two
	wantRect(t, list[2], state.TabBarRows+11, 41, 9, 39)
}

func TestASplitShowsTheSameBufferAtTheSamePlace(t *testing.T) {
	inWindows(t)

	press(t, "jl")
	press(t, ":sp\n")

	list := view.List(ed)
	if list[0].Entry != list[1].Entry {
		t.Error("the split window shows another buffer")
	}
	if list[1].CursorRow != list[0].CursorRow || list[1].CursorCol != list[0].CursorCol {
		t.Errorf("split cursor at %d,%d, want %d,%d",
			list[1].CursorRow, list[1].CursorCol, list[0].CursorRow, list[0].CursorCol)
	}
}

func TestEachWindowKeepsItsOwnCursor(t *testing.T) {
	inWindows(t)

	press(t, ":sp\n")
	press(t, "l") // only the lower window moves
	pressKey(t, termbox.KeyCtrlK)

	wantCursor(t, 0, 0)

	pressKey(t, termbox.KeyCtrlJ)

	wantCursor(t, 0, 1)
}

func TestTheLayoutPassLeavesTheCursorWhereItIs(t *testing.T) {
	inWindows(t)

	press(t, "l")
	view.Layout(ed) // which every redraw runs before anything is drawn

	wantCursor(t, 0, 1)
}

func TestCtrlJAndCtrlKMoveBetweenStackedWindows(t *testing.T) {
	inWindows(t)

	press(t, ":sp\n")
	below := view.Focused()

	pressKey(t, termbox.KeyCtrlK)
	if view.Focused() == below {
		t.Fatal("Ctrl-K stayed in the lower window")
	}

	pressKey(t, termbox.KeyCtrlJ)
	if view.Focused() != below {
		t.Error("Ctrl-J did not come back down")
	}
}

func TestCtrlHAndCtrlLMoveBetweenSideBySideWindows(t *testing.T) {
	inWindows(t)

	press(t, ":vs\n")
	right := view.Focused()

	pressKey(t, termbox.KeyCtrlH)
	if view.Focused() == right {
		t.Fatal("Ctrl-H stayed in the right window")
	}

	pressKey(t, termbox.KeyCtrlL)
	if view.Focused() != right {
		t.Error("Ctrl-L did not come back right")
	}
}

func TestAMoveWithNoWindowThatWayStaysPut(t *testing.T) {
	inWindows(t)

	press(t, ":sp\n")
	below := view.Focused()

	pressKey(t, termbox.KeyCtrlJ)
	pressKey(t, termbox.KeyCtrlH)

	if view.Focused() != below {
		t.Error("a move with no window that way left the window")
	}
}

func TestCtrlHIsStillBackspaceWhileTyping(t *testing.T) {
	inWindows(t)

	press(t, ":vs\n")
	right := view.Focused()
	press(t, "lli")
	pressKey(t, termbox.KeyCtrlH)

	if view.Focused() != right {
		t.Error("Ctrl-H left the window while typing")
	}
	edtest.WantLines(t, ed.Buf, "frst")
}

func TestClosingAWindowGivesItsRoomBack(t *testing.T) {
	inWindows(t)

	press(t, ":sp\n")
	press(t, ":close\n")

	wantWindowCount(t, 1)
	wantRect(t, view.List(ed)[0], state.TabBarRows, 0, 20, 80)
	if len(view.Separators()) != 0 {
		t.Errorf("separators = %v, want none", view.Separators())
	}
}

func TestTheLastWindowCannotBeClosed(t *testing.T) {
	inWindows(t)

	press(t, ":close\n")

	wantWindowCount(t, 1)
	if ed.StatusMsg != "E444: Cannot close last window" {
		t.Errorf("statusMsg = %q", ed.StatusMsg)
	}
}

func TestOnlyClosesEveryOtherWindow(t *testing.T) {
	inWindows(t)

	press(t, ":sp\n")
	press(t, ":vs\n")
	kept := view.Focused()
	press(t, ":only\n")

	wantWindowCount(t, 1)
	if view.Focused() != kept {
		t.Error("only left another window than the one it was run in")
	}
	wantRect(t, view.Focused(), state.TabBarRows, 0, 20, 80)
}

func TestQuitClosesTheWindowUntilItIsTheLastOne(t *testing.T) {
	inWindows(t)

	press(t, ":sp\n")
	press(t, ":q\n")

	wantWindowCount(t, 1)
	if ed.Quitting {
		t.Fatal("quit the editor while a window was left")
	}

	press(t, ":q\n")

	if !ed.Quitting {
		t.Error("still running after :q in the last window")
	}
}

func TestSplitWithANameOpensTheFileInTheNewWindowAlone(t *testing.T) {
	paths := inWindows(t, "a.txt")

	press(t, ":sp "+paths[0]+"\n")

	list := view.List(ed)
	if got := filepath.Base(list[1].Entry.Path); got != "a.txt" {
		t.Errorf("new window shows %q, want a.txt", got)
	}
	if got := filepath.Base(list[0].Entry.Path); got != "f.txt" {
		t.Errorf("old window shows %q, want f.txt", got)
	}
}

func TestSplitRefusesWhenThereIsNoRoomForIt(t *testing.T) {
	inWindows(t)
	singleWindow(4, 30)

	press(t, ":vs\n")

	wantWindowCount(t, 1)
	if ed.StatusMsg != "E36: Not enough room" {
		t.Errorf("statusMsg = %q", ed.StatusMsg)
	}

	press(t, ":sp\n")

	wantWindowCount(t, 1)
	if ed.StatusMsg != "E36: Not enough room" {
		t.Errorf("statusMsg = %q", ed.StatusMsg)
	}
}

func TestLeaderSplitsAndClosesWindows(t *testing.T) {
	inWindows(t)

	press(t, " sv")
	wantWindowCount(t, 2)
	if view.Separators()[0].Vertical {
		t.Error("<leader>sv put the windows side by side")
	}

	press(t, " sh")
	wantWindowCount(t, 3)
	if !view.Separators()[len(view.Separators())-1].Vertical {
		t.Error("<leader>sh stacked the windows")
	}

	press(t, " wd")
	wantWindowCount(t, 2)

	press(t, " wd")
	wantWindowCount(t, 1)
}

func TestCtrlHLeavesTheExplorerForTheWindowBeside(t *testing.T) {
	inWindows(t)

	press(t, ":vs\n")
	right := view.Focused()
	press(t, " e")
	pressKey(t, termbox.KeyCtrlH)

	if view.Focused() == right {
		t.Fatal("Ctrl-H stayed in the explorer's window")
	}
	if ed.ExplorerOpen || ed.Mode != state.ReadMode {
		t.Error("the explorer came along to the window moved to")
	}
}

func TestAMoveOutOfTheExplorerWithNoWindowThatWayKeepsIt(t *testing.T) {
	inWindows(t)

	press(t, " e")
	pressKey(t, termbox.KeyCtrlL)

	if !ed.ExplorerOpen || ed.Mode != state.ExplorerMode {
		t.Error("the explorer closed on a move that had nowhere to go")
	}
}

func TestClosingABufferLeavesNoWindowShowingIt(t *testing.T) {
	paths := inWindows(t, "a.txt")

	press(t, ":sp "+paths[0]+"\n")
	press(t, ":bd\n")

	for _, w := range view.List(ed) {
		if got := filepath.Base(w.Entry.Path); got != "f.txt" {
			t.Errorf("window still shows %q", got)
		}
	}
}
