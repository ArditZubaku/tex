package command_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nsf/termbox-go"

	"github.com/ArditZubaku/tex/internal/buffer"
	"github.com/ArditZubaku/tex/internal/editor/edtest"
	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/ArditZubaku/tex/internal/format"
)

// fakeFormatter puts a gofumpt on the PATH running the shell body it is given,
// with the file to format left in "$f".
func fakeFormatter(t *testing.T, body string) {
	t.Helper()

	dir := t.TempDir()
	script := "#!/bin/sh\nfor f; do :; done\n" + body + "\n"
	if err := os.WriteFile(filepath.Join(dir, "gofumpt"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", dir)
	format.Reset()
}

// openGo is edtest's own setup against a file the formatter table knows.
func openGo(t *testing.T, e *state.Editor, content string, row, col int) string {
	t.Helper()

	edtest.InReadMode(t, e, "", 0, 0)

	path := filepath.Join(t.TempDir(), "a.go")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	b := buffer.Open(path)
	t.Cleanup(b.Close)
	e.Buf, e.SourceFile, e.Row, e.Col, e.Modified = b, path, row, col, false
	edtest.SingleWindow(e, 24, 80)

	return path
}

func TestWritingAFileShowsWhatTheFormatterMadeOfIt(t *testing.T) {
	fakeFormatter(t, `printf 'package main\n\nfunc main() {}\n' > "$f"`)

	e := state.New()
	path := openGo(t, e, "package main\nfunc main()   {}\n", 1, 0)

	edtest.Press(t, e, ":w\n")

	edtest.WantLines(t, e.Buf, "package main", "", "func main() {}")
	if got := readFile(t, path); got != "package main\n\nfunc main() {}\n" {
		t.Errorf("file = %q", got)
	}
	if !strings.Contains(e.StatusMsg, "gofumpt") {
		t.Errorf("status = %q, want it to name the formatter", e.StatusMsg)
	}
}

func TestCtrlSFormatsTheSameWayWriteDoes(t *testing.T) {
	fakeFormatter(t, `printf 'package main\n' > "$f"`)

	e := state.New()
	openGo(t, e, "package   main\n", 0, 0)

	edtest.PressKey(t, e, termbox.KeyCtrlS)

	edtest.WantLines(t, e.Buf, "package main")
}

func TestFormattingLeavesTheCursorInsideTheFileItShortened(t *testing.T) {
	fakeFormatter(t, `printf 'pkg\n' > "$f"`)

	e := state.New()
	openGo(t, e, "package main\n\n\n\n\nvar x = 1\n", 5, 7)

	edtest.Press(t, e, ":w\n")

	edtest.WantCursor(t, e, 0, 2)
}

func TestAFileTheFormatterRefusesIsStillWritten(t *testing.T) {
	fakeFormatter(t, `echo "a.go:2:6: expected declaration" >&2; exit 2`)

	e := state.New()
	path := openGo(t, e, "package main\nfunc(\n", 1, 0)

	edtest.Press(t, e, ":w\n")

	if got := readFile(t, path); got != "package main\nfunc(\n" {
		t.Errorf("file = %q, want the save to have landed anyway", got)
	}
	if !strings.Contains(e.StatusMsg, "written") || strings.Contains(e.StatusMsg, "expected") {
		t.Errorf("status = %q, want the write alone, the complaint being the box's", e.StatusMsg)
	}
	if !e.Note.Showing() || e.Note.Text() != "a.go:2:6: expected declaration" {
		t.Errorf("box = %q, want the formatter's own complaint", e.Note.Text())
	}
	if e.Modified {
		t.Error("buffer still counts as modified")
	}
}

func TestAWriteThatFormatsCleanlyTakesTheBoxDown(t *testing.T) {
	fakeFormatter(t, `echo "a.go:2:6: expected declaration" >&2; exit 2`)

	e := state.New()
	openGo(t, e, "package main\nfunc(\n", 1, 0)
	edtest.Press(t, e, ":w\n")
	if !e.Note.Showing() {
		t.Fatal("nothing was raised, so the test proves nothing")
	}

	fakeFormatter(t, `printf 'package main\n' > "$f"`)
	edtest.Press(t, e, ":w\n")

	if e.Note.Showing() {
		t.Errorf("box still showing %q after a clean write", e.Note.Text())
	}
}

func TestEscTakesTheBoxDown(t *testing.T) {
	fakeFormatter(t, `echo "a.go:2:6: expected declaration" >&2; exit 2`)

	e := state.New()
	openGo(t, e, "package main\nfunc(\n", 1, 0)
	edtest.Press(t, e, ":w\n")

	edtest.Esc(t, e)

	if e.Note.Showing() {
		t.Errorf("box still showing %q after Esc", e.Note.Text())
	}
}

func TestReindentingKeepsTheUndoHistory(t *testing.T) {
	fakeFormatter(t, `printf 'package main\n\tvar x = 1\n' > "$f"`)

	e := state.New()
	openGo(t, e, "package main\nvar x = 2\n", 1, 8)

	edtest.Press(t, e, "x")
	edtest.Press(t, e, ":w\n")

	if !e.Hist.CanUndo() {
		t.Fatal("undo was dropped by a formatter that moved no lines")
	}
}

func TestAFormatterThatMovesLinesDropsTheUndoHistory(t *testing.T) {
	fakeFormatter(t, `printf 'package main\n' > "$f"`)

	e := state.New()
	openGo(t, e, "package main\nx\n", 1, 0)

	edtest.Press(t, e, "ohello")
	edtest.Esc(t, e)
	if !e.Hist.CanUndo() {
		t.Fatal("nothing to undo before the write, so the test proves nothing")
	}

	edtest.Press(t, e, ":w\n")

	if e.Hist.CanUndo() {
		t.Fatal("undo would put lines back at rows the formatter moved")
	}
}

func TestWriteAllFormatsEveryFileItWrites(t *testing.T) {
	fakeFormatter(t, `printf 'package main\n' > "$f"`)

	e := state.New()
	first := openGo(t, e, "package   main\n", 0, 0)

	second := filepath.Join(t.TempDir(), "b.go")
	if err := os.WriteFile(second, []byte("package   two\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	edtest.Press(t, e, ":e "+second+"\n")
	edtest.Press(t, e, "x")

	edtest.Press(t, e, ":wa\n")

	if got := readFile(t, first); got != "package   main\n" {
		t.Errorf("unmodified file = %q, want it untouched", got)
	}
	if got := readFile(t, second); got != "package main\n" {
		t.Errorf("written file = %q, want it formatted", got)
	}
	edtest.WantLines(t, e.Buf, "package main")
}

func TestAFileOfALanguageWithNoFormatterIsWrittenAsItIs(t *testing.T) {
	fakeFormatter(t, `printf 'formatted\n' > "$f"`)

	e := state.New()
	edtest.InReadMode(t, e, "one   two\n", 0, 0)
	path := e.SourceFile

	edtest.Press(t, e, ":w\n")

	if got := readFile(t, path); got != "one   two\n" {
		t.Errorf("file = %q, want a .txt file left alone", got)
	}
}
