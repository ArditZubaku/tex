// The tests sit outside the package because the harness they share with the
// rest of the editor imports it.
package command_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ArditZubaku/tex/internal/editor/edtest"
	"github.com/ArditZubaku/tex/internal/editor/find"
	"github.com/ArditZubaku/tex/internal/editor/state"
)

func readFile(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	return string(content)
}

func TestWriteCommandSavesTheBuffer(t *testing.T) {
	e := state.New()

	edtest.InReadMode(t, e, "one\ntwo\n", 0, 0)
	path := e.SourceFile

	edtest.Press(t, e, "x")
	edtest.Press(t, e, ":w\n")

	if got := readFile(t, path); got != "ne\ntwo\n" {
		t.Errorf("file = %q, want %q", got, "ne\ntwo\n")
	}
	if e.Modified {
		t.Error("buffer still marked modified after :w")
	}
}

func TestWriteCommandWithANameContinuesEditingIt(t *testing.T) {
	e := state.New()

	edtest.InReadMode(t, e, "one\n", 0, 0)
	other := filepath.Join(filepath.Dir(e.SourceFile), "other.txt")

	edtest.Press(t, e, ":w "+other+"\n")

	if got := readFile(t, other); got != "one\n" {
		t.Errorf("%s = %q, want %q", other, got, "one\n")
	}
	if e.SourceFile != other {
		t.Errorf("sourceFile = %q, want %q", e.SourceFile, other)
	}
}

func TestQuitCommandAsksBeforeDroppingChanges(t *testing.T) {
	e := state.New()

	edtest.InReadMode(t, e, "one\n", 0, 0)

	edtest.Press(t, e, "x")
	edtest.Press(t, e, ":q\n")

	if e.Quitting {
		t.Error("quit with unsaved changes")
	}
	if e.Mode != state.PromptMode {
		t.Fatalf("mode = %v, want PromptMode", e.Mode)
	}
	if want := filepath.Base(e.SourceFile); !strings.Contains(e.Prompt.Text(), want) {
		t.Errorf("prompt = %q, want it to name %q", e.Prompt.Text(), want)
	}
}

func TestForcedQuitCommandDropsChanges(t *testing.T) {
	e := state.New()

	edtest.InReadMode(t, e, "one\n", 0, 0)
	path := e.SourceFile

	edtest.Press(t, e, "x")
	edtest.Press(t, e, ":q!\n")

	if !e.Quitting {
		t.Error("still running after :q!")
	}
	if got := readFile(t, path); got != "one\n" {
		t.Errorf("file = %q, want it untouched", got)
	}
}

func TestQuitCommandOnASavedBuffer(t *testing.T) {
	e := state.New()

	edtest.InReadMode(t, e, "one\n", 0, 0)

	edtest.Press(t, e, ":q\n")

	if !e.Quitting {
		t.Error("still running after :q on an unmodified buffer")
	}
}

func TestWriteQuitCommandDoesBoth(t *testing.T) {
	e := state.New()

	edtest.InReadMode(t, e, "one\ntwo\n", 0, 0)
	path := e.SourceFile

	edtest.Press(t, e, "dd")
	edtest.Press(t, e, ":wq\n")

	if got := readFile(t, path); got != "two\n" {
		t.Errorf("file = %q, want %q", got, "two\n")
	}
	if !e.Quitting {
		t.Error("still running after :wq")
	}
}

func TestWriteQuitAliasX(t *testing.T) {
	e := state.New()

	edtest.InReadMode(t, e, "one\n", 0, 0)

	edtest.Press(t, e, ":x\n")

	if !e.Quitting {
		t.Error("still running after :x")
	}
}

func TestLineNumberCommandJumps(t *testing.T) {
	e := state.New()

	edtest.InReadMode(t, e, "a\nb\nc\nd\ne\n", 0, 0)

	edtest.Press(t, e, ":4\n")

	edtest.WantCursor(t, e, 3, 0)
}

func TestLineNumberCommandClamps(t *testing.T) {
	e := state.New()

	edtest.InReadMode(t, e, "a\nb\nc\n", 0, 0)

	edtest.Press(t, e, ":99\n")
	edtest.WantCursor(t, e, 2, 0)

	edtest.Press(t, e, ":0\n")
	edtest.WantCursor(t, e, 0, 0)
}

func TestDollarCommandJumpsToTheLastLine(t *testing.T) {
	e := state.New()

	edtest.InReadMode(t, e, "a\nb\nc\n", 0, 0)

	edtest.Press(t, e, ":$\n")

	edtest.WantCursor(t, e, 2, 0)
}

func TestALineAddressIsRememberedByTheJumpList(t *testing.T) {
	e := state.New()

	edtest.InReadMode(t, e, "a\nb\nc\nd\n", 1, 0)

	edtest.Press(t, e, ":4\n")
	edtest.WantCursor(t, e, 3, 0)

	find.JumpBack(e)
	edtest.WantCursor(t, e, 1, 0)
}

func TestUnknownCommandIsReported(t *testing.T) {
	e := state.New()

	edtest.InReadMode(t, e, "one\n", 0, 0)

	edtest.Press(t, e, ":bogus\n")

	if e.StatusMsg != "E492: Not an editor command: bogus" {
		t.Errorf("statusMsg = %q", e.StatusMsg)
	}
	if e.Quitting {
		t.Error("quit on an unknown command")
	}
}

func TestNohlsearchCommandClearsTheHighlight(t *testing.T) {
	e := state.New()

	edtest.InReadMode(t, e, "hit and hit\n", 0, 0)

	edtest.Press(t, e, "/hit\n")
	edtest.Press(t, e, ":noh\n")

	if hits := find.LineHits(e, 0); len(hits.Cols()) != 0 {
		t.Errorf("still highlighting %v after :noh", hits.Cols())
	}
	edtest.Press(t, e, "n")
	edtest.WantCursor(t, e, 0, 0)
}

func TestEscLeavesTheCommandPromptWithoutRunningIt(t *testing.T) {
	e := state.New()

	edtest.InReadMode(t, e, "one\n", 0, 0)

	edtest.Press(t, e, ":q")
	edtest.Press(t, e, string(rune(27)))

	if e.Quitting {
		t.Error("ran the command the prompt was cancelled on")
	}
	if e.Mode != state.ReadMode {
		t.Errorf("mode = %v, want ReadMode", e.Mode)
	}
}

func TestThePromptShowsTheCommandBeingTyped(t *testing.T) {
	e := state.New()

	edtest.InReadMode(t, e, "one\n", 0, 0)

	edtest.Press(t, e, ":wq")

	if txt, ok := e.PromptStatus(); !ok || txt != ":wq" {
		t.Errorf("promptStatus = %q,%v, want \":wq\",true", txt, ok)
	}
}

func TestEmptyCommandDoesNothing(t *testing.T) {
	e := state.New()

	edtest.InReadMode(t, e, "one\n", 0, 0)

	edtest.Press(t, e, ":\n")

	if e.StatusMsg != "" || e.Quitting {
		t.Errorf("statusMsg = %q, quitting = %v", e.StatusMsg, e.Quitting)
	}
}
