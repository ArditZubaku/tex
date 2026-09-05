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

func inBuffers(e *state.Editor, t *testing.T, names ...string) []string {
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

	edtest.InReadMode(t, e, "first\n", 0, 0)
	edtest.SingleWindow(e, 20, 80)

	return paths
}

func openPaths(e *state.Editor, t *testing.T, paths ...string) {
	t.Helper()

	for _, path := range paths {
		edtest.Press(t, e, ":e "+path+"\n")
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

func wantCurrent(e *state.Editor, t *testing.T, want string) {
	t.Helper()

	if got := filepath.Base(e.SourceFile); got != want {
		t.Errorf("editing %q, want %q", got, want)
	}
}

func TestEditKeepsTheBufferItLeavesInTheList(t *testing.T) {
	e := state.New()

	paths := inBuffers(e, t, "a.txt", "b.txt")

	openPaths(e, t, paths...)

	wantBuffers(t, "f.txt", "a.txt", "b.txt")
	wantCurrent(e, t, "b.txt")
}

func TestTabWalksTheBufferListAndWrapsRound(t *testing.T) {
	e := state.New()

	paths := inBuffers(e, t, "a.txt")
	openPaths(e, t, paths...)

	edtest.Press(t, e, "\t")
	wantCurrent(e, t, "f.txt")

	edtest.Press(t, e, "\t")
	wantCurrent(e, t, "a.txt")
}

func TestTabIsStillAnIndentInEditMode(t *testing.T) {
	e := state.New()

	paths := inBuffers(e, t, "a.txt")
	openPaths(e, t, paths...)

	edtest.Press(t, e, "i\t")

	wantCurrent(e, t, "a.txt")
	edtest.WantLines(t, e.Buf, "    in a.txt")
}

func TestShiftHAndShiftLTakeTheBufferBeforeAndAfter(t *testing.T) {
	e := state.New()

	paths := inBuffers(e, t, "a.txt", "b.txt")
	openPaths(e, t, paths...)

	edtest.Press(t, e, "H")
	wantCurrent(e, t, "a.txt")

	edtest.Press(t, e, "L")
	wantCurrent(e, t, "b.txt")

	edtest.Press(t, e, "L")
	wantCurrent(e, t, "f.txt")
}

func TestSwitchingBuffersKeepsTheCursorEachOneWasLeftAt(t *testing.T) {
	e := state.New()

	paths := inBuffers(e, t, "a.txt")

	edtest.Press(t, e, "jl") // the starting buffer is one line, so this only moves the column
	openPaths(e, t, paths...)
	edtest.Press(t, e, "\t")

	wantCurrent(e, t, "f.txt")
	wantCursor(e, t, 0, 1)
}

func TestSwitchingBuffersKeepsUnsavedChangesAndTheirUndoHistory(t *testing.T) {
	e := state.New()

	paths := inBuffers(e, t, "a.txt")

	edtest.Press(t, e, "x")
	openPaths(e, t, paths...)
	edtest.Press(t, e, "\t")

	if !e.Modified {
		t.Error("the buffer came back saved")
	}
	edtest.WantLines(t, e.Buf, "irst")

	edtest.Press(t, e, "u")

	edtest.WantLines(t, e.Buf, "first")
}

func TestEditingAnOpenFileSwitchesToItRatherThanOpeningItTwice(t *testing.T) {
	e := state.New()

	paths := inBuffers(e, t, "a.txt")
	openPaths(e, t, paths...)
	first := e.SourceFile

	edtest.Press(t, e, "\t")
	edtest.Press(t, e, ":e "+first+"\n")

	wantBuffers(t, "f.txt", "a.txt")
	wantCurrent(e, t, "a.txt")
}

func TestRereadingTheCurrentFileStillRefusesToDropChanges(t *testing.T) {
	e := state.New()

	inBuffers(e, t)

	edtest.Press(t, e, "x")
	edtest.Press(t, e, ":e\n")

	if e.StatusMsg != state.NoWriteSinceChange {
		t.Errorf("statusMsg = %q, want %q", e.StatusMsg, state.NoWriteSinceChange)
	}
	edtest.WantLines(t, e.Buf, "irst")

	edtest.Press(t, e, ":e!\n")

	edtest.WantLines(t, e.Buf, "first")
	if len(view.Buffers()) != 1 {
		t.Errorf("%d buffers open, want 1", len(view.Buffers()))
	}
}

func TestBufferDeleteDropsItAndLandsOnTheNextOne(t *testing.T) {
	e := state.New()

	paths := inBuffers(e, t, "a.txt", "b.txt")
	openPaths(e, t, paths...)

	edtest.Press(t, e, "H")
	edtest.Press(t, e, ":bd\n")

	wantBuffers(t, "f.txt", "b.txt")
	wantCurrent(e, t, "b.txt")
}

func TestLeaderBdClosesTheBufferAsBdDoes(t *testing.T) {
	e := state.New()

	paths := inBuffers(e, t, "a.txt", "b.txt")
	openPaths(e, t, paths...)

	edtest.Press(t, e, "H")
	edtest.Press(t, e, " bd")

	wantBuffers(t, "f.txt", "b.txt")
	wantCurrent(e, t, "b.txt")
}

func TestLeaderBdLeavesUnsavedChangesAlone(t *testing.T) {
	e := state.New()

	paths := inBuffers(e, t, "a.txt")
	openPaths(e, t, paths...)

	edtest.Press(t, e, "x")
	edtest.Press(t, e, " bd")

	wantBuffers(t, "f.txt", "a.txt")
	if e.StatusMsg != state.NoWriteSinceChange {
		t.Errorf("statusMsg = %q, want %q", e.StatusMsg, state.NoWriteSinceChange)
	}
}

func TestLeaderBnAndLeaderBpWalkTheList(t *testing.T) {
	e := state.New()

	paths := inBuffers(e, t, "a.txt", "b.txt")
	openPaths(e, t, paths...)

	edtest.Press(t, e, " bp")
	wantCurrent(e, t, "a.txt")

	edtest.Press(t, e, " bn")
	wantCurrent(e, t, "b.txt")
}

func TestLeaderBbGoesBackToTheBufferLastLeft(t *testing.T) {
	e := state.New()

	paths := inBuffers(e, t, "a.txt", "b.txt")
	openPaths(e, t, paths...)

	edtest.Press(t, e, "H") // a.txt, leaving b.txt behind
	edtest.Press(t, e, " bb")
	wantCurrent(e, t, "b.txt")

	edtest.Press(t, e, " bb")
	wantCurrent(e, t, "a.txt")
}

func TestLeaderBoClosesEveryOtherBuffer(t *testing.T) {
	e := state.New()

	paths := inBuffers(e, t, "a.txt", "b.txt")
	openPaths(e, t, paths...)

	edtest.Press(t, e, " bo")

	wantBuffers(t, "b.txt")
	wantCurrent(e, t, "b.txt")
}

func TestLeaderBlAndLeaderBrCloseOneSideOfTheList(t *testing.T) {
	e := state.New()

	paths := inBuffers(e, t, "a.txt", "b.txt", "c.txt")
	openPaths(e, t, paths...)

	edtest.Press(t, e, "H") // b.txt, with two buffers to its left and one to its right
	edtest.Press(t, e, " br")
	wantBuffers(t, "f.txt", "a.txt", "b.txt")

	edtest.Press(t, e, " bl")
	wantBuffers(t, "b.txt")
	wantCurrent(e, t, "b.txt")
}

func TestClosingASideOfTheListRefusesWhenOneOfThemIsUnsaved(t *testing.T) {
	e := state.New()

	paths := inBuffers(e, t, "a.txt")

	edtest.Press(t, e, "x") // the starting buffer, which the open below leaves to the left
	openPaths(e, t, paths...)
	edtest.Press(t, e, " bl")

	wantBuffers(t, "f.txt", "a.txt")
	if want := view.Unwritten(view.Buffers()[0]); e.StatusMsg != want {
		t.Errorf("statusMsg = %q, want %q", e.StatusMsg, want)
	}
}

func TestClosingASideOfTheListSaysWhenThereIsNoSide(t *testing.T) {
	e := state.New()

	inBuffers(e, t)

	edtest.Press(t, e, " bl")

	if e.StatusMsg != "no buffers to the left to close" {
		t.Errorf("statusMsg = %q", e.StatusMsg)
	}
}

func TestBufferDeleteRefusesToDropUnsavedChanges(t *testing.T) {
	e := state.New()

	paths := inBuffers(e, t, "a.txt")
	openPaths(e, t, paths...)

	edtest.Press(t, e, "x")
	edtest.Press(t, e, ":bd\n")

	if e.StatusMsg != state.NoWriteSinceChange {
		t.Errorf("statusMsg = %q, want %q", e.StatusMsg, state.NoWriteSinceChange)
	}
	wantBuffers(t, "f.txt", "a.txt")

	edtest.Press(t, e, ":bd!\n")

	wantBuffers(t, "f.txt")
	wantCurrent(e, t, "f.txt")
}

func TestDeletingTheLastBufferLeavesAnEmptyOne(t *testing.T) {
	e := state.New()

	inBuffers(e, t)

	edtest.Press(t, e, ":bd\n")

	wantBuffers(t, state.DefaultFileName)
	if e.Buf.LineCount() != 1 {
		t.Errorf("%d lines, want the one of an empty buffer", e.Buf.LineCount())
	}
}

func TestQuitRefusesWhileAnotherBufferHasUnsavedChanges(t *testing.T) {
	e := state.New()

	paths := inBuffers(e, t, "a.txt")

	edtest.Press(t, e, "x")
	openPaths(e, t, paths...)
	edtest.Press(t, e, ":q\n")

	if e.Quitting {
		t.Error("quit with another buffer unsaved")
	}
	if want := `E162: No write since last change for buffer "` + view.Buffers()[0].Path + `"`; e.StatusMsg != want {
		t.Errorf("statusMsg = %q, want %q", e.StatusMsg, want)
	}

	edtest.Press(t, e, ":q!\n")

	if !e.Quitting {
		t.Error("still running after :q!")
	}
}

func TestBufferListNamesThemMarkingTheCurrentAndTheUnsaved(t *testing.T) {
	e := state.New()

	paths := inBuffers(e, t, "a.txt")

	edtest.Press(t, e, "x")
	openPaths(e, t, paths...)
	edtest.Press(t, e, ":ls\n")

	if want := " 1 f.txt ●  %2 a.txt"; e.StatusMsg != want {
		t.Errorf("statusMsg = %q, want %q", e.StatusMsg, want)
	}
}

func TestBufferLineListsTheOpenBuffersAndMarksTheUnsaved(t *testing.T) {
	e := state.New()

	paths := inBuffers(e, t, "a.txt")

	edtest.Press(t, e, "x")
	openPaths(e, t, paths...)

	open := render.Tabs(e)
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
