package command_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ArditZubaku/tex/internal/editor/edtest"
	"github.com/ArditZubaku/tex/internal/editor/state"
)

// withEdits is an editor over several files, each of them typed into and none
// of them written, which is the state quitting has to ask about.
func withEdits(t *testing.T, e *state.Editor, names ...string) []string {
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

	for _, path := range paths {
		edtest.Press(t, e, ":e "+path+"\n")
		edtest.Press(t, e, "x")
	}

	return paths
}

func wantAsking(t *testing.T, e *state.Editor, names ...string) {
	t.Helper()

	if e.Quitting {
		t.Fatal("quit without asking about unsaved changes")
	}
	if e.Mode != state.PromptMode {
		t.Fatalf("mode = %v, want PromptMode", e.Mode)
	}
	for _, name := range names {
		if !strings.Contains(e.Prompt.Text(), name) {
			t.Errorf("prompt = %q, want it to name %q", e.Prompt.Text(), name)
		}
	}
}

func TestQuittingWithNothingUnsavedDoesNotAsk(t *testing.T) {
	e := state.New()

	edtest.InReadMode(t, e, "one\n", 0, 0)
	edtest.SingleWindow(e, 20, 80)

	edtest.Press(t, e, "q")

	if !e.Quitting {
		t.Error("did not quit with a clean buffer")
	}
}

func TestTheQuitKeyAsksBeforeDroppingChanges(t *testing.T) {
	e := state.New()

	edtest.InReadMode(t, e, "one\n", 0, 0)
	edtest.SingleWindow(e, 20, 80)

	edtest.Press(t, e, "x")
	edtest.Press(t, e, "q")

	wantAsking(t, e, filepath.Base(e.SourceFile))
}

func TestQuittingNamesEveryUnsavedBufferNotOnlyTheOneOnScreen(t *testing.T) {
	e := state.New()

	withEdits(t, e, "a.txt", "b.txt")

	edtest.Press(t, e, "q")

	wantAsking(t, e, "a.txt", "b.txt")
}

func TestQuittingCountsUnsavedBuffersItHasNoRoomToName(t *testing.T) {
	e := state.New()

	withEdits(t, e, "a.txt", "b.txt", "c.txt", "d.txt")

	edtest.Press(t, e, "q")

	// f.txt, the buffer the editor started in, is clean: the four opened here
	// are what the prompt has to fit, and it names three of them
	wantAsking(t, e, "and 1 more")
}

func TestAnsweringYesWritesEveryUnsavedBufferThenQuits(t *testing.T) {
	e := state.New()

	paths := withEdits(t, e, "a.txt", "b.txt")

	edtest.Press(t, e, "q")
	edtest.Press(t, e, "y\n")

	if !e.Quitting {
		t.Error("did not quit after saving")
	}
	for _, path := range paths {
		if got := readFile(t, path); strings.HasPrefix(got, "in ") {
			t.Errorf("%q = %q, want the edit written", filepath.Base(path), got)
		}
	}
}

func TestAnsweringNoQuitsAndLeavesTheFilesAsTheyWere(t *testing.T) {
	e := state.New()

	paths := withEdits(t, e, "a.txt")

	edtest.Press(t, e, "q")
	edtest.Press(t, e, "n\n")

	if !e.Quitting {
		t.Error("did not quit after giving the changes up")
	}
	if got, want := readFile(t, paths[0]), "in a.txt\n"; got != want {
		t.Errorf("%q = %q, want %q", filepath.Base(paths[0]), got, want)
	}
}

func TestAnsweringAnythingElseKeepsTheEditorAndTheChanges(t *testing.T) {
	e := state.New()

	withEdits(t, e, "a.txt")

	edtest.Press(t, e, "q")
	edtest.Press(t, e, "c\n")

	if e.Quitting {
		t.Error("quit on an answer that was neither yes nor no")
	}
	if !e.Modified {
		t.Error("changes lost by a cancelled quit")
	}
}

func TestEscapingTheQuitPromptKeepsTheEditorAndTheChanges(t *testing.T) {
	e := state.New()

	withEdits(t, e, "a.txt")

	edtest.Press(t, e, "q")
	edtest.Press(t, e, "\x1b")

	if e.Quitting {
		t.Error("quit on an escaped prompt")
	}
	if e.Mode != state.ReadMode {
		t.Errorf("mode = %v, want ReadMode", e.Mode)
	}
	if !e.Modified {
		t.Error("changes lost by an escaped quit")
	}
}

func TestForcedQuitAllDropsEveryUnsavedBufferWithoutAsking(t *testing.T) {
	e := state.New()

	paths := withEdits(t, e, "a.txt")

	edtest.Press(t, e, ":qa!\n")

	if !e.Quitting {
		t.Error("still running after :qa!")
	}
	if got, want := readFile(t, paths[0]), "in a.txt\n"; got != want {
		t.Errorf("%q = %q, want %q", filepath.Base(paths[0]), got, want)
	}
}

func TestQuitAllAsksTheWayTheQuitKeyDoes(t *testing.T) {
	e := state.New()

	withEdits(t, e, "a.txt")

	edtest.Press(t, e, ":qa\n")

	wantAsking(t, e, "a.txt")
}

// ':wq' writes the file it was typed in, which leaves the other unsaved buffers
// for the same prompt every other way out asks with.
func TestWriteQuitStillAsksAboutTheBuffersItDidNotWrite(t *testing.T) {
	e := state.New()

	withEdits(t, e, "a.txt", "b.txt")

	edtest.Press(t, e, ":wq\n")

	wantAsking(t, e, "a.txt")
	if strings.Contains(e.Prompt.Text(), "b.txt") {
		t.Errorf("prompt = %q, want it to leave out the file ':wq' wrote", e.Prompt.Text())
	}
}

func TestAWriteThatFailsKeepsTheEditorOpen(t *testing.T) {
	e := state.New()

	paths := withEdits(t, e, "a.txt")
	if err := os.Chmod(filepath.Dir(paths[0]), 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Dir(paths[0]), 0o700) })

	e.SourceFile = filepath.Join(filepath.Dir(paths[0]), "new.txt")

	edtest.Press(t, e, "q")
	edtest.Press(t, e, "y\n")

	if e.Quitting {
		t.Error("quit on top of a failed write")
	}
	if !strings.HasPrefix(e.StatusMsg, "E212:") {
		t.Errorf("statusMsg = %q, want the write to be reported", e.StatusMsg)
	}
}
