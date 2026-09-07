package find

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ArditZubaku/tex/internal/buffer"
	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/ArditZubaku/tex/internal/editor/view"
	"github.com/ArditZubaku/tex/internal/lsp"
)

// These are the answers a server gives, acted on without one: what crosses the
// wire is the language server package's own business, and what is left here is
// turning a range counted in the protocol's units into somewhere to go.

func answered(t *testing.T, name, content string) (*state.Editor, string) {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	view.Reset()
	Reset()
	t.Cleanup(func() {
		view.Reset()
		Reset()
	})

	e := state.New()
	e.Buf = buffer.Open(path)
	t.Cleanup(e.Buf.Close)
	e.SourceFile = path
	e.ScreenRows, e.ScreenCols, e.Rows, e.Cols = 20, 80, 20, 80

	return e, path
}

func TestAnAnswerOlderThanTheNewestLookupIsDropped(t *testing.T) {
	e, _ := answered(t, "main.go", "package main\n")

	first, from := ask(e)
	second, _ := ask(e)

	if stale(e, second, from) {
		t.Error("the newest lookup was taken for a stale one")
	}
	if !stale(e, first, from) {
		t.Error("an answer to the lookup before it was still acted on")
	}
}

func TestAnAnswerToACursorThatHasMovedOnIsDropped(t *testing.T) {
	e, _ := answered(t, "main.go", "package main\n\nfunc main() {}\n")

	token, from := ask(e)
	e.Row = 2

	if !stale(e, token, from) {
		t.Error("the answer was acted on after the cursor had left")
	}
}

func TestAServerAnswerLandsTheCursorOnTheRuneItNamed(t *testing.T) {
	e, path := answered(t, "wide.go", "package main\n\nvar 😀😀ab = 1\n")

	// 'var ' then two runes of two UTF-16 units each: 'a' is the eighth unit
	goToFirst(e, []lsp.Location{{
		URI:   lsp.FileURI(path),
		Range: lsp.Range{Start: lsp.Position{Line: 2, Character: 8}},
	}}, nil)

	if e.Row != 2 || e.Col != 6 {
		t.Errorf("the jump landed at %d,%d, want 2,6", e.Row, e.Col)
	}
}

func TestAServerThatFoundNothingLeavesTheTextToAnswer(t *testing.T) {
	e, _ := answered(t, "main.go", "package main\n\nvar total = 1\n\nfunc use() { _ = total }\n")
	e.Row, e.Col = 4, 18

	goToFirst(e, nil, nil)

	if e.Row != 2 {
		t.Errorf("the text search put the cursor on row %d, want 2", e.Row)
	}
	if e.StatusMsg != "" {
		t.Errorf("the fallback reported %q", e.StatusMsg)
	}
}

func TestAServerThatRefusedLeavesTheTextToAnswer(t *testing.T) {
	e, _ := answered(t, "main.go", "package main\n\nvar total = 1\n\nfunc use() { _ = total }\n")
	e.Row, e.Col = 4, 18

	goToFirst(e, nil, lsp.ErrStopped)

	if e.Row != 2 {
		t.Errorf("the text search put the cursor on row %d, want 2", e.Row)
	}
}

func TestAnAnswerInAFileThatIsGoneLeavesTheTextToAnswer(t *testing.T) {
	e, _ := answered(t, "main.go", "package main\n\nvar total = 1\n\nfunc use() { _ = total }\n")
	e.Row, e.Col = 4, 18

	goToFirst(e, []lsp.Location{{
		URI:   lsp.FileURI(filepath.Join(t.TempDir(), "vanished.go")),
		Range: lsp.Range{Start: lsp.Position{Line: 0}},
	}}, nil)

	if e.Row != 2 {
		t.Errorf("the text search put the cursor on row %d, want 2", e.Row)
	}
}
