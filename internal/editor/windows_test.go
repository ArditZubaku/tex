package editor

import (
	"path/filepath"
	"testing"

	"github.com/nsf/termbox-go"
)

// singleWindow is the state the editor's own layout pass leaves behind for one
// unsplit window filling the screen, which is what every test that draws or
// scrolls assumes it starts from.
func singleWindow(rows, cols int) {
	root, current = nil, nil
	screenRows, screenCols = rows, cols
	ROWS, COLS = rows, cols
	winRow, winCol = tabBarRows, 0
}

// The window moves are Ctrl-hjkl, which arrive as keys rather than runes and so
// take the path every special key does.
func pressKey(t *testing.T, key termbox.Key) {
	t.Helper()

	dispatchKey(termbox.Event{Key: key})
}

func wantWindowCount(t *testing.T, want int) {
	t.Helper()

	if got := len(windowList()); got != want {
		t.Fatalf("%d windows, want %d", got, want)
	}
}

func wantRect(t *testing.T, w *window, row, col, rows, cols int) {
	t.Helper()

	if w.row != row || w.col != col || w.rows != rows || w.cols != cols {
		t.Errorf("window at %d,%d %dx%d, want %d,%d %dx%d",
			w.row, w.col, w.rows, w.cols, row, col, rows, cols)
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
	list := windowList()
	wantRect(t, list[0], tabBarRows, 0, 10, 80)
	wantRect(t, list[1], tabBarRows+11, 0, 9, 80)
	if current != list[1] {
		t.Error("the cursor stayed above the split")
	}
	if len(separators) != 1 || separators[0].vertical {
		t.Errorf("separators = %v, want one horizontal", separators)
	}
}

func TestVerticalSplitPutsTheWindowsSideBySideAndTakesTheRightOne(t *testing.T) {
	inWindows(t)

	press(t, ":vs\n")

	wantWindowCount(t, 2)
	list := windowList()
	wantRect(t, list[0], tabBarRows, 0, 20, 40)
	wantRect(t, list[1], tabBarRows, 41, 20, 39)
	if current != list[1] {
		t.Error("the cursor stayed left of the split")
	}
	if len(separators) != 1 || !separators[0].vertical {
		t.Errorf("separators = %v, want one vertical", separators)
	}
}

func TestSplittingAgainTheSameWayShareTheRoomEqually(t *testing.T) {
	inWindows(t)
	singleWindow(20, 130)

	press(t, ":vs\n")
	press(t, ":vs\n")

	wantWindowCount(t, 3)
	for _, w := range windowList() {
		if w.cols < 42 || w.cols > 43 {
			t.Errorf("window %d columns wide, want a third of the 128 left by two separators", w.cols)
		}
	}
}

func TestSplittingTheOtherWayNestsInsideTheWindowSplit(t *testing.T) {
	inWindows(t)

	press(t, ":vs\n")
	press(t, ":sp\n")

	wantWindowCount(t, 3)
	list := windowList()
	wantRect(t, list[0], tabBarRows, 0, 20, 40)  // the left window, untouched
	wantRect(t, list[1], tabBarRows, 41, 10, 39) // the right one, split in two
	wantRect(t, list[2], tabBarRows+11, 41, 9, 39)
}

func TestASplitShowsTheSameBufferAtTheSamePlace(t *testing.T) {
	inWindows(t)

	press(t, "jl")
	press(t, ":sp\n")

	list := windowList()
	if list[0].entry != list[1].entry {
		t.Error("the split window shows another buffer")
	}
	if list[1].cursorRow != list[0].cursorRow || list[1].cursorCol != list[0].cursorCol {
		t.Errorf("split cursor at %d,%d, want %d,%d",
			list[1].cursorRow, list[1].cursorCol, list[0].cursorRow, list[0].cursorCol)
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
	layoutWindows() // which every redraw runs before anything is drawn

	wantCursor(t, 0, 1)
}

func TestCtrlJAndCtrlKMoveBetweenStackedWindows(t *testing.T) {
	inWindows(t)

	press(t, ":sp\n")
	below := current

	pressKey(t, termbox.KeyCtrlK)
	if current == below {
		t.Fatal("Ctrl-K stayed in the lower window")
	}

	pressKey(t, termbox.KeyCtrlJ)
	if current != below {
		t.Error("Ctrl-J did not come back down")
	}
}

func TestCtrlHAndCtrlLMoveBetweenSideBySideWindows(t *testing.T) {
	inWindows(t)

	press(t, ":vs\n")
	right := current

	pressKey(t, termbox.KeyCtrlH)
	if current == right {
		t.Fatal("Ctrl-H stayed in the right window")
	}

	pressKey(t, termbox.KeyCtrlL)
	if current != right {
		t.Error("Ctrl-L did not come back right")
	}
}

func TestAMoveWithNoWindowThatWayStaysPut(t *testing.T) {
	inWindows(t)

	press(t, ":sp\n")
	below := current

	pressKey(t, termbox.KeyCtrlJ)
	pressKey(t, termbox.KeyCtrlH)

	if current != below {
		t.Error("a move with no window that way left the window")
	}
}

func TestCtrlHIsStillBackspaceWhileTyping(t *testing.T) {
	inWindows(t)

	press(t, ":vs\n")
	right := current
	press(t, "lli")
	pressKey(t, termbox.KeyCtrlH)

	if current != right {
		t.Error("Ctrl-H left the window while typing")
	}
	wantLines(t, buf, "frst")
}

func TestClosingAWindowGivesItsRoomBack(t *testing.T) {
	inWindows(t)

	press(t, ":sp\n")
	press(t, ":close\n")

	wantWindowCount(t, 1)
	wantRect(t, windowList()[0], tabBarRows, 0, 20, 80)
	if len(separators) != 0 {
		t.Errorf("separators = %v, want none", separators)
	}
}

func TestTheLastWindowCannotBeClosed(t *testing.T) {
	inWindows(t)

	press(t, ":close\n")

	wantWindowCount(t, 1)
	if statusMsg != "E444: Cannot close last window" {
		t.Errorf("statusMsg = %q", statusMsg)
	}
}

func TestOnlyClosesEveryOtherWindow(t *testing.T) {
	inWindows(t)

	press(t, ":sp\n")
	press(t, ":vs\n")
	kept := current
	press(t, ":only\n")

	wantWindowCount(t, 1)
	if current != kept {
		t.Error("only left another window than the one it was run in")
	}
	wantRect(t, current, tabBarRows, 0, 20, 80)
}

func TestQuitClosesTheWindowUntilItIsTheLastOne(t *testing.T) {
	inWindows(t)

	press(t, ":sp\n")
	press(t, ":q\n")

	wantWindowCount(t, 1)
	if quitting {
		t.Fatal("quit the editor while a window was left")
	}

	press(t, ":q\n")

	if !quitting {
		t.Error("still running after :q in the last window")
	}
}

func TestSplitWithANameOpensTheFileInTheNewWindowAlone(t *testing.T) {
	paths := inWindows(t, "a.txt")

	press(t, ":sp "+paths[0]+"\n")

	list := windowList()
	if got := filepath.Base(list[1].entry.path); got != "a.txt" {
		t.Errorf("new window shows %q, want a.txt", got)
	}
	if got := filepath.Base(list[0].entry.path); got != "f.txt" {
		t.Errorf("old window shows %q, want f.txt", got)
	}
}

func TestSplitRefusesWhenThereIsNoRoomForIt(t *testing.T) {
	inWindows(t)
	singleWindow(4, 30)

	press(t, ":vs\n")

	wantWindowCount(t, 1)
	if statusMsg != "E36: Not enough room" {
		t.Errorf("statusMsg = %q", statusMsg)
	}

	press(t, ":sp\n")

	wantWindowCount(t, 1)
	if statusMsg != "E36: Not enough room" {
		t.Errorf("statusMsg = %q", statusMsg)
	}
}

func TestLeaderSplitsAndClosesWindows(t *testing.T) {
	inWindows(t)

	press(t, " sv")
	wantWindowCount(t, 2)
	if separators[0].vertical {
		t.Error("<leader>sv put the windows side by side")
	}

	press(t, " sh")
	wantWindowCount(t, 3)
	if !separators[len(separators)-1].vertical {
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
	right := current
	press(t, " e")
	pressKey(t, termbox.KeyCtrlH)

	if current == right {
		t.Fatal("Ctrl-H stayed in the explorer's window")
	}
	if explorerOpen || mode != ReadMode {
		t.Error("the explorer came along to the window moved to")
	}
}

func TestAMoveOutOfTheExplorerWithNoWindowThatWayKeepsIt(t *testing.T) {
	inWindows(t)

	press(t, " e")
	pressKey(t, termbox.KeyCtrlL)

	if !explorerOpen || mode != ExplorerMode {
		t.Error("the explorer closed on a move that had nowhere to go")
	}
}

func TestClosingABufferLeavesNoWindowShowingIt(t *testing.T) {
	paths := inWindows(t, "a.txt")

	press(t, ":sp "+paths[0]+"\n")
	press(t, ":bd\n")

	for _, w := range windowList() {
		if got := filepath.Base(w.entry.path); got != "f.txt" {
			t.Errorf("window still shows %q", got)
		}
	}
}
