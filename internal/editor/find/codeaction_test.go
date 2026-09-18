package find

import (
	"path/filepath"
	"testing"

	"github.com/ArditZubaku/tex/internal/editor/diag"
	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/ArditZubaku/tex/internal/lsp"
)

func TestPressingLeaderCAWithNoServerSaysSoRatherThanNothing(t *testing.T) {
	e, path := answered(t, "notes.txt", "some prose\n")

	OpenCodeActions(e)

	if want := "no language server for " + filepath.Base(path); e.StatusMsg != want {
		t.Errorf("it reported %q, want %q", e.StatusMsg, want)
	}
}

func TestAServerWithNoCodeActionsSaysSoRatherThanOpeningTheMenu(t *testing.T) {
	e, _ := answered(t, "main.go", "package main\n")

	listCodeActions(e, nil, nil)

	if e.Mode == state.PickerMode {
		t.Fatal("nothing found still opened the menu")
	}
	if want := "no code actions here"; e.StatusMsg != want {
		t.Errorf("it reported %q, want %q", e.StatusMsg, want)
	}
}

func TestAServerThatDidNotAnswerTheCodeActionsSaysSo(t *testing.T) {
	e, _ := answered(t, "main.go", "package main\n")

	listCodeActions(e, nil, lsp.ErrStopped)

	if e.Mode == state.PickerMode {
		t.Fatal("a refused lookup still opened the menu")
	}
	if want := "the language server did not answer"; e.StatusMsg != want {
		t.Errorf("it reported %q, want %q", e.StatusMsg, want)
	}
}

func TestCodeActionsFromAServerOpenAMenuEvenForOne(t *testing.T) {
	e, _ := answered(t, "main.go", "package main\n")

	listCodeActions(e, []lsp.CodeAction{{Title: "Add missing method"}}, nil)

	if e.Mode != state.PickerMode {
		t.Fatalf("a single action did not open the menu: %q", e.StatusMsg)
	}

	entries := e.Pick.Matched()
	if len(entries) != 1 || entries[0].Label != "Add missing method" {
		t.Errorf("the menu lists %v, want one row reading %q", entries, "Add missing method")
	}
}

// The menu's Enter handler goes through Index rather than always running the
// first action, which a two-entry menu is what tells that from always picking
// entry zero.
func TestChoosingACodeActionRunsTheOneAtItsOwnIndex(t *testing.T) {
	e, path := answered(t, "main.go", "foo\n")

	found := []lsp.CodeAction{
		{Title: "Organize imports"},
		{
			Title: "Append bar",
			Edit: &lsp.WorkspaceEdit{Changes: map[string][]lsp.TextEdit{
				lsp.FileURI(path): {{
					Range:   lsp.Range{Start: lsp.Position{Character: 3}, End: lsp.Position{Character: 3}},
					NewText: "bar",
				}},
			}},
		},
	}
	listCodeActions(e, found, nil)

	entries := e.Pick.Matched()
	if len(entries) != 2 {
		t.Fatalf("the menu lists %d rows, want 2", len(entries))
	}
	if e.PickAction == nil {
		t.Fatal("the menu carried no action to run on Enter")
	}
	e.PickAction(entries[1])

	if got := string(e.Buf.Line(0)); got != "foobar" {
		t.Errorf("the line reads %q, want %q", got, "foobar")
	}
}

func TestApplyingACodeActionEditsTheBufferAndCanBeUndone(t *testing.T) {
	e, path := answered(t, "main.go", "foo\n")

	action := lsp.CodeAction{
		Title: "Append bar",
		Edit: &lsp.WorkspaceEdit{Changes: map[string][]lsp.TextEdit{
			lsp.FileURI(path): {{
				Range:   lsp.Range{Start: lsp.Position{Character: 3}, End: lsp.Position{Character: 3}},
				NewText: "bar",
			}},
		}},
	}

	applyCodeAction(e, action)

	if got := string(e.Buf.Line(0)); got != "foobar" {
		t.Fatalf("the line reads %q, want %q", got, "foobar")
	}
	if want := "applied: Append bar"; e.StatusMsg != want {
		t.Errorf("it reported %q, want %q", e.StatusMsg, want)
	}

	e.Undo()
	if got := string(e.Buf.Line(0)); got != "foo" {
		t.Errorf("undo left the line reading %q, want the original %q", got, "foo")
	}
}

// gopls answers "declare the missing method" with data rather than an edit,
// and this bare editor never has a server behind it to resolve one with — the
// same shape a real one answers when it never declared resolveProvider.
func TestACodeActionWithDataButNothingToResolveItHasNothingToApply(t *testing.T) {
	e, _ := answered(t, "main.go", "foo\n")

	applyCodeAction(e, lsp.CodeAction{Title: "Declare missing method", Data: []byte(`{}`)})

	if got := string(e.Buf.Line(0)); got != "foo" {
		t.Errorf("a data-only action with nothing to resolve it changed the buffer to %q", got)
	}
	if want := "code action has nothing to apply: Declare missing method"; e.StatusMsg != want {
		t.Errorf("it reported %q, want %q", e.StatusMsg, want)
	}
}

func TestACodeActionThatCreatesRenamesOrDeletesAFileIsRefusedWhole(t *testing.T) {
	e, path := answered(t, "main.go", "foo\n")

	applyCodeAction(e, lsp.CodeAction{
		Title: "Extract to new file",
		Edit: &lsp.WorkspaceEdit{
			FileOps: true,
			Changes: map[string][]lsp.TextEdit{lsp.FileURI(path): {{NewText: "bar"}}},
		},
	})

	if got := string(e.Buf.Line(0)); got != "foo" {
		t.Errorf("a refused file operation still changed the buffer to %q", got)
	}
	if want := "code action creates, renames, or deletes a file, not yet supported: Extract to new file"; e.StatusMsg != want {
		t.Errorf("it reported %q, want %q", e.StatusMsg, want)
	}
}

func TestACodeActionThatRunsACommandIsRefusedRatherThanIgnored(t *testing.T) {
	e, _ := answered(t, "main.go", "foo\n")

	applyCodeAction(e, lsp.CodeAction{Title: "Run gopls.test", Command: &lsp.Command{Command: "gopls.test"}})

	if got := string(e.Buf.Line(0)); got != "foo" {
		t.Errorf("a command action changed the buffer to %q", got)
	}
	if want := "code action runs a command, not yet supported: Run gopls.test"; e.StatusMsg != want {
		t.Errorf("it reported %q, want %q", e.StatusMsg, want)
	}
}

func TestACodeActionWithNoEditHasNothingToApply(t *testing.T) {
	e, _ := answered(t, "main.go", "foo\n")

	applyCodeAction(e, lsp.CodeAction{Title: "Nothing doing"})

	if want := "code action has nothing to apply: Nothing doing"; e.StatusMsg != want {
		t.Errorf("it reported %q, want %q", e.StatusMsg, want)
	}
}

func TestACodeActionWithAnEmptyEditForThisFileHasNothingToApply(t *testing.T) {
	e, path := answered(t, "main.go", "foo\n")

	applyCodeAction(e, lsp.CodeAction{
		Title: "Nothing doing",
		Edit:  &lsp.WorkspaceEdit{Changes: map[string][]lsp.TextEdit{lsp.FileURI(path): {}}},
	})

	if want := "code action has nothing to apply: Nothing doing"; e.StatusMsg != want {
		t.Errorf("it reported %q, want %q", e.StatusMsg, want)
	}
}

func TestACodeActionThatReachesAnotherFileIsRefusedWhole(t *testing.T) {
	e, path := answered(t, "main.go", "foo\n")
	other := filepath.Join(filepath.Dir(path), "other.go")

	applyCodeAction(e, lsp.CodeAction{
		Title: "Rename across files",
		Edit: &lsp.WorkspaceEdit{Changes: map[string][]lsp.TextEdit{
			lsp.FileURI(path):  {{NewText: "bar"}},
			lsp.FileURI(other): {{NewText: "baz"}},
		}},
	})

	if got := string(e.Buf.Line(0)); got != "foo" {
		t.Errorf("a refused action still changed the buffer to %q", got)
	}
	if want := "code action reaches beyond this file, not yet supported: Rename across files"; e.StatusMsg != want {
		t.Errorf("it reported %q, want %q", e.StatusMsg, want)
	}
}

func TestDiagnosticsAtTheCursorAreCarriedIntoTheServersUnits(t *testing.T) {
	e, path := answered(t, "main.go", "func use() { _ = missing }\n")
	t.Cleanup(diag.Reset)

	diag.Set(path, diag.File{
		{Message: "undefined: missing", Row: 0, Col: 17, EndRow: 0, EndCol: 24},
		{Message: "somewhere else entirely", Row: 5, Col: 0, EndRow: 5, EndCol: 3},
	})

	found := diagnosticsAt(path, e.Buf, 0, 20)

	if len(found) != 1 || found[0].Message != "undefined: missing" {
		t.Fatalf("the diagnostics at the cursor came back as %v", found)
	}
	if found[0].Range.Start.Character != 17 || found[0].Range.End.Character != 24 {
		t.Errorf("the range came back as %v, want 17..24", found[0].Range)
	}
}

// The range's end is exclusive, the same as everywhere else one crosses the
// wire: the rune right after the word a diagnostic underlines is not part of
// what it is complaining about.
func TestACursorOnTheDiagnosticsExclusiveEndIsNotCoveredByIt(t *testing.T) {
	e, path := answered(t, "main.go", "func use() { _ = missing }\n")
	t.Cleanup(diag.Reset)

	diag.Set(path, diag.File{{Message: "undefined: missing", Row: 0, Col: 17, EndRow: 0, EndCol: 24}})

	if found := diagnosticsAt(path, e.Buf, 0, 24); len(found) != 0 {
		t.Errorf("a cursor on the exclusive end still picked up %v", found)
	}
	if found := diagnosticsAt(path, e.Buf, 0, 23); len(found) != 1 {
		t.Errorf("a cursor on the last rune it covers came back as %v, want it picked up", found)
	}
	if found := diagnosticsAt(path, e.Buf, 0, 16); len(found) != 0 {
		t.Errorf("a cursor before the diagnostic still picked it up: %v", found)
	}
}
