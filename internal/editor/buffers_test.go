package editor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ArditZubaku/tex/internal/editor/edtest"
	"github.com/ArditZubaku/tex/internal/editor/render"
	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/ArditZubaku/tex/internal/editor/view"
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
	singleWindow(20, 80)

	return paths
}

func openPaths(t *testing.T, paths ...string) {
	t.Helper()

	for _, path := range paths {
		press(t, ":e "+path+"\n")
	}
}

func bufferNames() []string {
	names := make([]string, 0, len(view.Buffers()))
	for _, entry := range view.Buffers() {
		names = append(names, filepath.Base(entry.Path))
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

	if got := filepath.Base(ed.SourceFile); got != want {
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
	edtest.WantLines(t, ed.Buf, "    in a.txt")
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

	if !ed.Modified {
		t.Error("the buffer came back saved")
	}
	edtest.WantLines(t, ed.Buf, "irst")

	press(t, "u")

	edtest.WantLines(t, ed.Buf, "first")
}

func TestEditingAnOpenFileSwitchesToItRatherThanOpeningItTwice(t *testing.T) {
	paths := inBuffers(t, "a.txt")
	openPaths(t, paths...)
	first := ed.SourceFile

	press(t, "\t")
	press(t, ":e "+first+"\n")

	wantBuffers(t, "f.txt", "a.txt")
	wantCurrent(t, "a.txt")
}

func TestRereadingTheCurrentFileStillRefusesToDropChanges(t *testing.T) {
	inBuffers(t)

	press(t, "x")
	press(t, ":e\n")

	if ed.StatusMsg != state.NoWriteSinceChange {
		t.Errorf("statusMsg = %q, want %q", ed.StatusMsg, state.NoWriteSinceChange)
	}
	edtest.WantLines(t, ed.Buf, "irst")

	press(t, ":e!\n")

	edtest.WantLines(t, ed.Buf, "first")
	if len(view.Buffers()) != 1 {
		t.Errorf("%d buffers open, want 1", len(view.Buffers()))
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
	if ed.StatusMsg != state.NoWriteSinceChange {
		t.Errorf("statusMsg = %q, want %q", ed.StatusMsg, state.NoWriteSinceChange)
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
	if want := view.Unwritten(view.Buffers()[0]); ed.StatusMsg != want {
		t.Errorf("statusMsg = %q, want %q", ed.StatusMsg, want)
	}
}

func TestClosingASideOfTheListSaysWhenThereIsNoSide(t *testing.T) {
	inBuffers(t)

	press(t, " bl")

	if ed.StatusMsg != "no buffers to the left to close" {
		t.Errorf("statusMsg = %q", ed.StatusMsg)
	}
}

func TestBufferDeleteRefusesToDropUnsavedChanges(t *testing.T) {
	paths := inBuffers(t, "a.txt")
	openPaths(t, paths...)

	press(t, "x")
	press(t, ":bd\n")

	if ed.StatusMsg != state.NoWriteSinceChange {
		t.Errorf("statusMsg = %q, want %q", ed.StatusMsg, state.NoWriteSinceChange)
	}
	wantBuffers(t, "f.txt", "a.txt")

	press(t, ":bd!\n")

	wantBuffers(t, "f.txt")
	wantCurrent(t, "f.txt")
}

func TestDeletingTheLastBufferLeavesAnEmptyOne(t *testing.T) {
	inBuffers(t)

	press(t, ":bd\n")

	wantBuffers(t, state.DefaultFileName)
	if ed.Buf.LineCount() != 1 {
		t.Errorf("%d lines, want the one of an empty buffer", ed.Buf.LineCount())
	}
}

func TestQuitRefusesWhileAnotherBufferHasUnsavedChanges(t *testing.T) {
	paths := inBuffers(t, "a.txt")

	press(t, "x")
	openPaths(t, paths...)
	press(t, ":q\n")

	if ed.Quitting {
		t.Error("quit with another buffer unsaved")
	}
	if want := `E162: No write since last change for buffer "` + view.Buffers()[0].Path + `"`; ed.StatusMsg != want {
		t.Errorf("statusMsg = %q, want %q", ed.StatusMsg, want)
	}

	press(t, ":q!\n")

	if !ed.Quitting {
		t.Error("still running after :q!")
	}
}

func TestBufferListNamesThemMarkingTheCurrentAndTheUnsaved(t *testing.T) {
	paths := inBuffers(t, "a.txt")

	press(t, "x")
	openPaths(t, paths...)
	press(t, ":ls\n")

	if want := " 1 f.txt ●  %2 a.txt"; ed.StatusMsg != want {
		t.Errorf("statusMsg = %q, want %q", ed.StatusMsg, want)
	}
}

func TestBufferLineListsTheOpenBuffersAndMarksTheUnsaved(t *testing.T) {
	paths := inBuffers(t, "a.txt")

	press(t, "x")
	openPaths(t, paths...)

	open := render.Tabs(ed)
	if len(open) != 2 {
		t.Fatalf("%d tabs, want 2", len(open))
	}
	if open[0].Name != "f.txt" || !open[0].Modified {
		t.Errorf("tab 0 = %+v, want f.txt unsaved", open[0])
	}
	if open[1].Name != "a.txt" || open[1].Modified {
		t.Errorf("tab 1 = %+v, want a.txt saved", open[1])
	}
	if view.Index() != 1 {
		t.Errorf("current buffer = %d, want the one just opened", view.Index())
	}
}
