package editor

import (
	"os"
	"path/filepath"
	"testing"

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
	inReadMode(t, "one\ntwo\n", 0, 0)
	path := ed.SourceFile

	press(t, "x")
	press(t, ":w\n")

	if got := readFile(t, path); got != "ne\ntwo\n" {
		t.Errorf("file = %q, want %q", got, "ne\ntwo\n")
	}
	if ed.Modified {
		t.Error("buffer still marked modified after :w")
	}
}

func TestWriteCommandWithANameContinuesEditingIt(t *testing.T) {
	inReadMode(t, "one\n", 0, 0)
	other := filepath.Join(filepath.Dir(ed.SourceFile), "other.txt")

	press(t, ":w "+other+"\n")

	if got := readFile(t, other); got != "one\n" {
		t.Errorf("%s = %q, want %q", other, got, "one\n")
	}
	if ed.SourceFile != other {
		t.Errorf("sourceFile = %q, want %q", ed.SourceFile, other)
	}
}

func TestQuitCommandRefusesToDropChanges(t *testing.T) {
	inReadMode(t, "one\n", 0, 0)

	press(t, "x")
	press(t, ":q\n")

	if ed.Quitting {
		t.Error("quit with unsaved changes")
	}
	if ed.StatusMsg != "E37: No write since last change (add ! to override)" {
		t.Errorf("statusMsg = %q", ed.StatusMsg)
	}
}

func TestForcedQuitCommandDropsChanges(t *testing.T) {
	inReadMode(t, "one\n", 0, 0)
	path := ed.SourceFile

	press(t, "x")
	press(t, ":q!\n")

	if !ed.Quitting {
		t.Error("still running after :q!")
	}
	if got := readFile(t, path); got != "one\n" {
		t.Errorf("file = %q, want it untouched", got)
	}
}

func TestQuitCommandOnASavedBuffer(t *testing.T) {
	inReadMode(t, "one\n", 0, 0)

	press(t, ":q\n")

	if !ed.Quitting {
		t.Error("still running after :q on an unmodified buffer")
	}
}

func TestWriteQuitCommandDoesBoth(t *testing.T) {
	inReadMode(t, "one\ntwo\n", 0, 0)
	path := ed.SourceFile

	press(t, "dd")
	press(t, ":wq\n")

	if got := readFile(t, path); got != "two\n" {
		t.Errorf("file = %q, want %q", got, "two\n")
	}
	if !ed.Quitting {
		t.Error("still running after :wq")
	}
}

func TestWriteQuitAliasX(t *testing.T) {
	inReadMode(t, "one\n", 0, 0)

	press(t, ":x\n")

	if !ed.Quitting {
		t.Error("still running after :x")
	}
}

func TestLineNumberCommandJumps(t *testing.T) {
	inReadMode(t, "a\nb\nc\nd\ne\n", 0, 0)

	press(t, ":4\n")

	wantCursor(t, 3, 0)
}

func TestLineNumberCommandClamps(t *testing.T) {
	inReadMode(t, "a\nb\nc\n", 0, 0)

	press(t, ":99\n")
	wantCursor(t, 2, 0)

	press(t, ":0\n")
	wantCursor(t, 0, 0)
}

func TestDollarCommandJumpsToTheLastLine(t *testing.T) {
	inReadMode(t, "a\nb\nc\n", 0, 0)

	press(t, ":$\n")

	wantCursor(t, 2, 0)
}

func TestUnknownCommandIsReported(t *testing.T) {
	inReadMode(t, "one\n", 0, 0)

	press(t, ":bogus\n")

	if ed.StatusMsg != "E492: Not an editor command: bogus" {
		t.Errorf("statusMsg = %q", ed.StatusMsg)
	}
	if ed.Quitting {
		t.Error("quit on an unknown command")
	}
}

func TestNohlsearchCommandClearsTheHighlight(t *testing.T) {
	inReadMode(t, "hit and hit\n", 0, 0)

	press(t, "/hit\n")
	press(t, ":noh\n")

	if hits := find.LineHits(ed, 0); len(hits.Cols()) != 0 {
		t.Errorf("still highlighting %v after :noh", hits.Cols())
	}
	press(t, "n")
	wantCursor(t, 0, 0)
}

func TestEscLeavesTheCommandPromptWithoutRunningIt(t *testing.T) {
	inReadMode(t, "one\n", 0, 0)

	press(t, ":q")
	press(t, string(rune(27)))

	if ed.Quitting {
		t.Error("ran the command the prompt was cancelled on")
	}
	if ed.Mode != state.ReadMode {
		t.Errorf("mode = %v, want ReadMode", ed.Mode)
	}
}

func TestThePromptShowsTheCommandBeingTyped(t *testing.T) {
	inReadMode(t, "one\n", 0, 0)

	press(t, ":wq")

	if txt, ok := ed.PromptStatus(); !ok || txt != ":wq" {
		t.Errorf("promptStatus = %q,%v, want \":wq\",true", txt, ok)
	}
}

func TestEmptyCommandDoesNothing(t *testing.T) {
	inReadMode(t, "one\n", 0, 0)

	press(t, ":\n")

	if ed.StatusMsg != "" || ed.Quitting {
		t.Errorf("statusMsg = %q, quitting = %v", ed.StatusMsg, ed.Quitting)
	}
}
