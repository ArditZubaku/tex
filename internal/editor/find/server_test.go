package find

import (
	"os"
	"path/filepath"
	"strings"
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

func TestReferencesFromAServerAreLabelledWithTheLinesTheyFallOn(t *testing.T) {
	e, path := answered(t, "main.go", "package main\n\nvar total = 1\n\nfunc use() { _ = total }\n")

	listFound(e, "total", []lsp.Location{
		{URI: lsp.FileURI(path), Range: lsp.Range{Start: lsp.Position{Line: 2, Character: 4}}},
		{URI: lsp.FileURI(path), Range: lsp.Range{Start: lsp.Position{Line: 4, Character: 18}}},
	}, nil)

	if e.Mode != state.PickerMode {
		t.Fatalf("the references did not open the popup: %q", e.StatusMsg)
	}

	entries := e.Pick.Matched()
	if len(entries) != 2 {
		t.Fatalf("the popup lists %d rows, want 2", len(entries))
	}
	if want := "main.go:3: var total = 1"; entries[0].Label != want {
		t.Errorf("the first row reads %q, want %q", entries[0].Label, want)
	}
	if entries[1].Row != 4 || entries[1].Col != 18 {
		t.Errorf("the second row goes to %d,%d, want 4,18", entries[1].Row, entries[1].Col)
	}
}

func TestReferencesAcrossFilesOpenEachOfThemOnce(t *testing.T) {
	e, one := answered(t, "one.go", "package main\n\nvar total = 1\n")
	two := filepath.Join(filepath.Dir(one), "two.go")
	if err := os.WriteFile(two, []byte("package main\n\nfunc use() { _ = total }\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	listFound(e, "total", []lsp.Location{
		{URI: lsp.FileURI(two), Range: lsp.Range{Start: lsp.Position{Line: 2, Character: 18}}},
		{URI: lsp.FileURI(one), Range: lsp.Range{Start: lsp.Position{Line: 2, Character: 4}}},
		{URI: lsp.FileURI(two), Range: lsp.Range{Start: lsp.Position{Line: 0, Character: 8}}},
	}, nil)

	var labels []string
	for _, entry := range e.Pick.Matched() {
		labels = append(labels, entry.Label)
	}
	want := []string{
		"two.go:3: func use() { _ = total }",
		"two.go:1: package main",
		"one.go:3: var total = 1",
	}
	if strings.Join(labels, "\n") != strings.Join(want, "\n") {
		t.Errorf("the popup lists\n%s\nwant\n%s", strings.Join(labels, "\n"), strings.Join(want, "\n"))
	}
}

func TestAServerThatFoundNoReferencesSaysSoRatherThanSearchingTheText(t *testing.T) {
	e, _ := answered(t, "main.go", "package main\n\nvar total = 1\n")

	listFound(e, "total", nil, nil)

	if e.Mode == state.PickerMode {
		t.Fatal("nothing found still opened the popup")
	}
	if want := "no references to total"; e.StatusMsg != want {
		t.Errorf("it reported %q, want %q", e.StatusMsg, want)
	}
}

func TestAServerThatRefusedTheReferencesLeavesTheTextToList(t *testing.T) {
	e, _ := answered(t, "main.go", "package main\n\nvar total = 1\n\nfunc use() { _ = total }\n")
	e.Row, e.Col = 2, 4

	listFound(e, "total", nil, lsp.ErrStopped)

	if e.Mode != state.PickerMode {
		t.Fatalf("the text search did not open the popup: %q", e.StatusMsg)
	}
	if got := len(e.Pick.Matched()); got != 2 {
		t.Errorf("the text search listed %d mentions, want 2", got)
	}
}
