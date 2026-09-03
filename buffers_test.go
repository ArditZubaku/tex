package main

import (
	"os"
	"path/filepath"
	"testing"
)

func inBuffers(t *testing.T, names ...string) []string {
	t.Helper()

	dir := t.TempDir()
	paths := make([]string, 0, len(names))
	for _, name := range names {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("in "+name+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
	}

	inReadMode(t, "first\n", 0, 0)
	ROWS, COLS = 20, 80

	return paths
}

func openPaths(t *testing.T, paths ...string) {
	t.Helper()

	for _, path := range paths {
		press(t, ":e "+path+"\n")
	}
}

func bufferNames() []string {
	names := make([]string, 0, len(buffers))
	for _, entry := range buffers {
		names = append(names, filepath.Base(entry.path))
	}

	return names
}

func wantBuffers(t *testing.T, want ...string) {
	t.Helper()

	got := bufferNames()
	if len(got) != len(want) {
		t.Fatalf("buffers = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("buffers = %v, want %v", got, want)
		}
	}
}

func wantCurrent(t *testing.T, want string) {
	t.Helper()

	if got := filepath.Base(sourceFile); got != want {
		t.Errorf("editing %q, want %q", got, want)
	}
}

func TestEditKeepsTheBufferItLeavesInTheList(t *testing.T) {
	paths := inBuffers(t, "a.txt", "b.txt")

	openPaths(t, paths...)

	wantBuffers(t, "f.txt", "a.txt", "b.txt")
	wantCurrent(t, "b.txt")
}

func TestTabWalksTheBufferListAndWrapsRound(t *testing.T) {
	paths := inBuffers(t, "a.txt")
	openPaths(t, paths...)

	press(t, "\t")
	wantCurrent(t, "f.txt")

	press(t, "\t")
	wantCurrent(t, "a.txt")
}

func TestTabIsStillAnIndentInEditMode(t *testing.T) {
	paths := inBuffers(t, "a.txt")
	openPaths(t, paths...)

	press(t, "i\t")

	wantCurrent(t, "a.txt")
	wantLines(t, buf, "    in a.txt")
}

func TestShiftHAndShiftLTakeTheBufferBeforeAndAfter(t *testing.T) {
	paths := inBuffers(t, "a.txt", "b.txt")
	openPaths(t, paths...)

	press(t, "H")
	wantCurrent(t, "a.txt")

	press(t, "L")
	wantCurrent(t, "b.txt")

	press(t, "L")
	wantCurrent(t, "f.txt")
}

func TestSwitchingBuffersKeepsTheCursorEachOneWasLeftAt(t *testing.T) {
	paths := inBuffers(t, "a.txt")

	press(t, "jl") // the starting buffer is one line, so this only moves the column
	openPaths(t, paths...)
	press(t, "\t")

	wantCurrent(t, "f.txt")
	wantCursor(t, 0, 1)
}

func TestSwitchingBuffersKeepsUnsavedChangesAndTheirUndoHistory(t *testing.T) {
	paths := inBuffers(t, "a.txt")

	press(t, "x")
	openPaths(t, paths...)
	press(t, "\t")

	if !modified {
		t.Error("the buffer came back saved")
	}
	wantLines(t, buf, "irst")

	press(t, "u")

	wantLines(t, buf, "first")
}

func TestEditingAnOpenFileSwitchesToItRatherThanOpeningItTwice(t *testing.T) {
	paths := inBuffers(t, "a.txt")
	openPaths(t, paths...)
	first := sourceFile

	press(t, "\t")
	press(t, ":e "+first+"\n")

	wantBuffers(t, "f.txt", "a.txt")
	wantCurrent(t, "a.txt")
}

func TestRereadingTheCurrentFileStillRefusesToDropChanges(t *testing.T) {
	inBuffers(t)

	press(t, "x")
	press(t, ":e\n")

	if statusMsg != noWriteSinceChange {
		t.Errorf("statusMsg = %q, want %q", statusMsg, noWriteSinceChange)
	}
	wantLines(t, buf, "irst")

	press(t, ":e!\n")

	wantLines(t, buf, "first")
	if len(buffers) != 1 {
		t.Errorf("%d buffers open, want 1", len(buffers))
	}
}

func TestBufferDeleteDropsItAndLandsOnTheNextOne(t *testing.T) {
	paths := inBuffers(t, "a.txt", "b.txt")
	openPaths(t, paths...)

	press(t, "H")
	press(t, ":bd\n")

	wantBuffers(t, "f.txt", "b.txt")
	wantCurrent(t, "b.txt")
}

func TestLeaderBdClosesTheBufferAsBdDoes(t *testing.T) {
	paths := inBuffers(t, "a.txt", "b.txt")
	openPaths(t, paths...)

	press(t, "H")
	press(t, " bd")

	wantBuffers(t, "f.txt", "b.txt")
	wantCurrent(t, "b.txt")
}

func TestLeaderBdLeavesUnsavedChangesAlone(t *testing.T) {
	paths := inBuffers(t, "a.txt")
	openPaths(t, paths...)

	press(t, "x")
	press(t, " bd")

	wantBuffers(t, "f.txt", "a.txt")
	if statusMsg != noWriteSinceChange {
		t.Errorf("statusMsg = %q, want %q", statusMsg, noWriteSinceChange)
	}
}

func TestLeaderBnAndLeaderBpWalkTheList(t *testing.T) {
	paths := inBuffers(t, "a.txt", "b.txt")
	openPaths(t, paths...)

	press(t, " bp")
	wantCurrent(t, "a.txt")

	press(t, " bn")
	wantCurrent(t, "b.txt")
}

func TestLeaderBbGoesBackToTheBufferLastLeft(t *testing.T) {
	paths := inBuffers(t, "a.txt", "b.txt")
	openPaths(t, paths...)

	press(t, "H") // a.txt, leaving b.txt behind
	press(t, " bb")
	wantCurrent(t, "b.txt")

	press(t, " bb")
	wantCurrent(t, "a.txt")
}

func TestLeaderBoClosesEveryOtherBuffer(t *testing.T) {
	paths := inBuffers(t, "a.txt", "b.txt")
	openPaths(t, paths...)

	press(t, " bo")

	wantBuffers(t, "b.txt")
	wantCurrent(t, "b.txt")
}

func TestLeaderBlAndLeaderBrCloseOneSideOfTheList(t *testing.T) {
	paths := inBuffers(t, "a.txt", "b.txt", "c.txt")
	openPaths(t, paths...)

	press(t, "H") // b.txt, with two buffers to its left and one to its right
	press(t, " br")
	wantBuffers(t, "f.txt", "a.txt", "b.txt")

	press(t, " bl")
	wantBuffers(t, "b.txt")
	wantCurrent(t, "b.txt")
}

func TestClosingASideOfTheListRefusesWhenOneOfThemIsUnsaved(t *testing.T) {
	paths := inBuffers(t, "a.txt")

	press(t, "x") // the starting buffer, which the open below leaves to the left
	openPaths(t, paths...)
	press(t, " bl")

	wantBuffers(t, "f.txt", "a.txt")
	if want := unwritten(buffers[0]); statusMsg != want {
		t.Errorf("statusMsg = %q, want %q", statusMsg, want)
	}
}

func TestClosingASideOfTheListSaysWhenThereIsNoSide(t *testing.T) {
	inBuffers(t)

	press(t, " bl")

	if statusMsg != "no buffers to the left to close" {
		t.Errorf("statusMsg = %q", statusMsg)
	}
}

func TestBufferDeleteRefusesToDropUnsavedChanges(t *testing.T) {
	paths := inBuffers(t, "a.txt")
	openPaths(t, paths...)

	press(t, "x")
	press(t, ":bd\n")

	if statusMsg != noWriteSinceChange {
		t.Errorf("statusMsg = %q, want %q", statusMsg, noWriteSinceChange)
	}
	wantBuffers(t, "f.txt", "a.txt")

	press(t, ":bd!\n")

	wantBuffers(t, "f.txt")
	wantCurrent(t, "f.txt")
}

func TestDeletingTheLastBufferLeavesAnEmptyOne(t *testing.T) {
	inBuffers(t)

	press(t, ":bd\n")

	wantBuffers(t, defaultFileName)
	if buf.LineCount() != 1 {
		t.Errorf("%d lines, want the one of an empty buffer", buf.LineCount())
	}
}

func TestQuitRefusesWhileAnotherBufferHasUnsavedChanges(t *testing.T) {
	paths := inBuffers(t, "a.txt")

	press(t, "x")
	openPaths(t, paths...)
	press(t, ":q\n")

	if quitting {
		t.Error("quit with another buffer unsaved")
	}
	if want := `E162: No write since last change for buffer "` + buffers[0].path + `"`; statusMsg != want {
		t.Errorf("statusMsg = %q, want %q", statusMsg, want)
	}

	press(t, ":q!\n")

	if !quitting {
		t.Error("still running after :q!")
	}
}

func TestBufferListNamesThemMarkingTheCurrentAndTheUnsaved(t *testing.T) {
	paths := inBuffers(t, "a.txt")

	press(t, "x")
	openPaths(t, paths...)
	press(t, ":ls\n")

	if want := " 1 f.txt ●  %2 a.txt"; statusMsg != want {
		t.Errorf("statusMsg = %q, want %q", statusMsg, want)
	}
}

func TestBufferLinePicksOutTheCurrentBufferAndMarksTheUnsaved(t *testing.T) {
	paths := inBuffers(t, "a.txt")

	press(t, "x")
	openPaths(t, paths...)

	cells, from, to := bufferLineCells()
	if got := cellText(cells); got != " f.txt ●  a.txt " {
		t.Errorf("buffer line = %q", got)
	}
	if got := cellText(cells[from:to]); got != " a.txt " {
		t.Errorf("current tab = %q, want %q", got, " a.txt ")
	}
	for i, cell := range cells {
		want := active.tabFg
		switch {
		case i >= from && i < to:
			want = active.tabActiveFg
		case cell.ch == modifiedMark:
			want = active.tabModified
		}
		if cell.fg != want {
			t.Errorf("cell %d (%q) fg = %v, want %v", i, cell.ch, cell.fg, want)
		}
	}
}

func TestBufferLineScrollsTheCurrentBufferIntoView(t *testing.T) {
	names := make([]string, 0, 20)
	for i := range 20 {
		names = append(names, string(rune('a'+i))+".txt")
	}
	paths := inBuffers(t, names...)
	openPaths(t, paths...)

	cells, from, to := bufferLineCells()
	scrollBufferLine(len(cells), from, to)

	if len(cells) <= COLS {
		t.Fatalf("buffer line is %d cells wide, want wider than the window", len(cells))
	}
	if from < tabBarOffset || to > tabBarOffset+COLS {
		t.Errorf("current tab at [%d,%d) is outside the window at %d", from, to, tabBarOffset)
	}
}

func cellText(cells []tabCell) string {
	out := make([]rune, 0, len(cells))
	for _, cell := range cells {
		if cell.ch != 0 {
			out = append(out, cell.ch)
		}
	}

	return string(out)
}
