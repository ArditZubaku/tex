// The tests sit outside the package because the harness they share with the
// rest of the editor imports it.
package view_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ArditZubaku/tex/internal/editor/edtest"
	"github.com/ArditZubaku/tex/internal/editor/render"
	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/ArditZubaku/tex/internal/editor/view"
)

func inBuffers(t *testing.T, e *state.Editor, names ...string) []string {
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

func openPaths(t *testing.T, e *state.Editor, paths ...string) {
	t.Helper()

	for _, path := range paths {
		edtest.Press(t, e, ":e "+path+"\n")
	}
}

func TestEditKeepsTheBufferItLeavesInTheList(t *testing.T) {
	e := state.New()

	paths := inBuffers(t, e, "a.txt", "b.txt")

	openPaths(t, e, paths...)

	edtest.WantBuffers(t, "f.txt", "a.txt", "b.txt")
	edtest.WantCurrent(t, e, "b.txt")
}

func TestTabWalksTheBufferListAndWrapsRound(t *testing.T) {
	e := state.New()

	paths := inBuffers(t, e, "a.txt")
	openPaths(t, e, paths...)

	edtest.Press(t, e, "\t")
	edtest.WantCurrent(t, e, "f.txt")

	edtest.Press(t, e, "\t")
	edtest.WantCurrent(t, e, "a.txt")
}

func TestTabIsStillAnIndentInEditMode(t *testing.T) {
	e := state.New()

	paths := inBuffers(t, e, "a.txt")
	openPaths(t, e, paths...)

	edtest.Press(t, e, "i\t")

	edtest.WantCurrent(t, e, "a.txt")
	edtest.WantLines(t, e.Buf, "    in a.txt")
}

func TestShiftHAndShiftLTakeTheBufferBeforeAndAfter(t *testing.T) {
	e := state.New()

	paths := inBuffers(t, e, "a.txt", "b.txt")
	openPaths(t, e, paths...)

	edtest.Press(t, e, "H")
	edtest.WantCurrent(t, e, "a.txt")

	edtest.Press(t, e, "L")
	edtest.WantCurrent(t, e, "b.txt")

	edtest.Press(t, e, "L")
	edtest.WantCurrent(t, e, "f.txt")
}

func TestSwitchingBuffersKeepsTheCursorEachOneWasLeftAt(t *testing.T) {
	e := state.New()

	paths := inBuffers(t, e, "a.txt")

	edtest.Press(t, e, "jl") // the starting buffer is one line, so this only moves the column
	openPaths(t, e, paths...)
	edtest.Press(t, e, "\t")

	edtest.WantCurrent(t, e, "f.txt")
	edtest.WantCursor(t, e, 0, 1)
}

func TestSwitchingBuffersKeepsUnsavedChangesAndTheirUndoHistory(t *testing.T) {
	e := state.New()

	paths := inBuffers(t, e, "a.txt")

	edtest.Press(t, e, "x")
	openPaths(t, e, paths...)
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

	paths := inBuffers(t, e, "a.txt")
	openPaths(t, e, paths...)
	first := e.SourceFile

	edtest.Press(t, e, "\t")
	edtest.Press(t, e, ":e "+first+"\n")

	edtest.WantBuffers(t, "f.txt", "a.txt")
	edtest.WantCurrent(t, e, "a.txt")
}

func TestRereadingTheCurrentFileStillRefusesToDropChanges(t *testing.T) {
	e := state.New()

	inBuffers(t, e)

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

	paths := inBuffers(t, e, "a.txt", "b.txt")
	openPaths(t, e, paths...)

	edtest.Press(t, e, "H")
	edtest.Press(t, e, ":bd\n")

	edtest.WantBuffers(t, "f.txt", "b.txt")
	edtest.WantCurrent(t, e, "b.txt")
}

func TestLeaderBdClosesTheBufferAsBdDoes(t *testing.T) {
	e := state.New()

	paths := inBuffers(t, e, "a.txt", "b.txt")
	openPaths(t, e, paths...)

	edtest.Press(t, e, "H")
	edtest.Press(t, e, " bd")

	edtest.WantBuffers(t, "f.txt", "b.txt")
	edtest.WantCurrent(t, e, "b.txt")
}

func TestLeaderBdLeavesUnsavedChangesAlone(t *testing.T) {
	e := state.New()

	paths := inBuffers(t, e, "a.txt")
	openPaths(t, e, paths...)

	edtest.Press(t, e, "x")
	edtest.Press(t, e, " bd")

	edtest.WantBuffers(t, "f.txt", "a.txt")
	if e.StatusMsg != state.NoWriteSinceChange {
		t.Errorf("statusMsg = %q, want %q", e.StatusMsg, state.NoWriteSinceChange)
	}
}

func TestLeaderBnAndLeaderBpWalkTheList(t *testing.T) {
	e := state.New()

	paths := inBuffers(t, e, "a.txt", "b.txt")
	openPaths(t, e, paths...)

	edtest.Press(t, e, " bp")
	edtest.WantCurrent(t, e, "a.txt")

	edtest.Press(t, e, " bn")
	edtest.WantCurrent(t, e, "b.txt")
}

func TestLeaderBbGoesBackToTheBufferLastLeft(t *testing.T) {
	e := state.New()

	paths := inBuffers(t, e, "a.txt", "b.txt")
	openPaths(t, e, paths...)

	edtest.Press(t, e, "H") // a.txt, leaving b.txt behind
	edtest.Press(t, e, " bb")
	edtest.WantCurrent(t, e, "b.txt")

	edtest.Press(t, e, " bb")
	edtest.WantCurrent(t, e, "a.txt")
}

func TestLeaderBoClosesEveryOtherBuffer(t *testing.T) {
	e := state.New()

	paths := inBuffers(t, e, "a.txt", "b.txt")
	openPaths(t, e, paths...)

	edtest.Press(t, e, " bo")

	edtest.WantBuffers(t, "b.txt")
	edtest.WantCurrent(t, e, "b.txt")
}

func TestLeaderBlAndLeaderBrCloseOneSideOfTheList(t *testing.T) {
	e := state.New()

	paths := inBuffers(t, e, "a.txt", "b.txt", "c.txt")
	openPaths(t, e, paths...)

	edtest.Press(t, e, "H") // b.txt, with two buffers to its left and one to its right
	edtest.Press(t, e, " br")
	edtest.WantBuffers(t, "f.txt", "a.txt", "b.txt")

	edtest.Press(t, e, " bl")
	edtest.WantBuffers(t, "b.txt")
	edtest.WantCurrent(t, e, "b.txt")
}

func TestClosingASideOfTheListRefusesWhenOneOfThemIsUnsaved(t *testing.T) {
	e := state.New()

	paths := inBuffers(t, e, "a.txt")

	edtest.Press(t, e, "x") // the starting buffer, which the open below leaves to the left
	openPaths(t, e, paths...)
	edtest.Press(t, e, " bl")

	edtest.WantBuffers(t, "f.txt", "a.txt")
	if want := view.Unwritten(view.Buffers()[0]); e.StatusMsg != want {
		t.Errorf("statusMsg = %q, want %q", e.StatusMsg, want)
	}
}

func TestClosingASideOfTheListSaysWhenThereIsNoSide(t *testing.T) {
	e := state.New()

	inBuffers(t, e)

	edtest.Press(t, e, " bl")

	if e.StatusMsg != "no buffers to the left to close" {
		t.Errorf("statusMsg = %q", e.StatusMsg)
	}
}

func TestBufferDeleteRefusesToDropUnsavedChanges(t *testing.T) {
	e := state.New()

	paths := inBuffers(t, e, "a.txt")
	openPaths(t, e, paths...)

	edtest.Press(t, e, "x")
	edtest.Press(t, e, ":bd\n")

	if e.StatusMsg != state.NoWriteSinceChange {
		t.Errorf("statusMsg = %q, want %q", e.StatusMsg, state.NoWriteSinceChange)
	}
	edtest.WantBuffers(t, "f.txt", "a.txt")

	edtest.Press(t, e, ":bd!\n")

	edtest.WantBuffers(t, "f.txt")
	edtest.WantCurrent(t, e, "f.txt")
}

func TestDeletingTheLastBufferLeavesAnEmptyOne(t *testing.T) {
	e := state.New()

	inBuffers(t, e)

	edtest.Press(t, e, ":bd\n")

	edtest.WantBuffers(t, state.DefaultFileName)
	if e.Buf.LineCount() != 1 {
		t.Errorf("%d lines, want the one of an empty buffer", e.Buf.LineCount())
	}
}

func TestQuitAsksAboutABufferItIsNotEvenShowing(t *testing.T) {
	e := state.New()

	paths := inBuffers(t, e, "a.txt")

	edtest.Press(t, e, "x")
	openPaths(t, e, paths...)
	edtest.Press(t, e, ":q\n")

	if e.Quitting {
		t.Error("quit with another buffer unsaved")
	}
	if want := filepath.Base(view.Buffers()[0].Path); !strings.Contains(e.Prompt.Text(), want) {
		t.Errorf("prompt = %q, want it to name %q", e.Prompt.Text(), want)
	}

	edtest.Press(t, e, "\x1b")
	edtest.Press(t, e, ":q!\n")

	if !e.Quitting {
		t.Error("still running after :q!")
	}
}

func TestBufferListNamesThemMarkingTheCurrentAndTheUnsaved(t *testing.T) {
	e := state.New()

	paths := inBuffers(t, e, "a.txt")

	edtest.Press(t, e, "x")
	openPaths(t, e, paths...)
	edtest.Press(t, e, ":ls\n")

	if want := " 1 f.txt ●  %2 a.txt"; e.StatusMsg != want {
		t.Errorf("statusMsg = %q, want %q", e.StatusMsg, want)
	}
}

func TestBufferLineListsTheOpenBuffersAndMarksTheUnsaved(t *testing.T) {
	e := state.New()

	paths := inBuffers(t, e, "a.txt")

	edtest.Press(t, e, "x")
	openPaths(t, e, paths...)

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
