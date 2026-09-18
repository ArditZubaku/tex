package find

import (
	"path/filepath"

	"github.com/ArditZubaku/tex/internal/buffer"
	"github.com/ArditZubaku/tex/internal/editor/diag"
	"github.com/ArditZubaku/tex/internal/editor/edit"
	"github.com/ArditZubaku/tex/internal/editor/picker"
	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/ArditZubaku/tex/internal/lsp"
)

// '<leader>ca' is whatever a language server offers at the cursor — gopls's
// "create the missing method" among them, chosen from the same menu every
// other kind is. The menu is shown even for a single action, the same as
// every other listing in this package.
//
// gopls answers that one, and most of its other quickfixes, with data rather
// than an edit: a bookmark resolved into the edit only once a choice is made,
// through codeAction/resolve. That round trip happens where an edit is
// missing but data is not, so v1 only refuses a fix outright when there is
// truly nothing to turn into one — a Command run through
// workspace/executeCommand, still not sent here, or a WorkspaceEdit that
// reaches outside the file being edited, refused whole rather than left out
// or partly applied.
func OpenCodeActions(e *state.Editor) {
	if !lsp.CodeActing(e.SourceFile) {
		e.StatusMsg = "no language server for " + filepath.Base(e.SourceFile)

		return
	}

	askCodeActions(e)
}

func askCodeActions(e *state.Editor) {
	token, from := ask(e)
	path, b, row, col := e.SourceFile, e.Buf, e.Row, e.Col

	lsp.CodeActions(path, b, row, col, diagnosticsAt(path, b, row, col),
		func(found []lsp.CodeAction, err error) {
			if !stale(e, token, from) {
				listCodeActions(e, found, err)
			}
		})
}

func listCodeActions(e *state.Editor, found []lsp.CodeAction, err error) {
	if err != nil {
		e.StatusMsg = "the language server did not answer"

		return
	}
	if len(found) == 0 {
		e.StatusMsg = "no code actions here"

		return
	}

	entries := make([]picker.Entry, len(found))
	for i, action := range found {
		entries[i] = picker.Entry{Label: action.Title, Index: i}
	}

	e.PickAction = func(entry picker.Entry) { applyCodeAction(e, found[entry.Index]) }
	showPicker(e, "Code actions", entries)
}

// applyCodeAction is what Enter does with the one chosen. An Edit already in
// hand is applied; one that reaches beyond the file being edited is refused
// whole rather than in part; a Command is not run, nothing here sends
// workspace/executeCommand. Data with no Edit yet is the one case worth a
// round trip rather than a refusal — resolveCodeAction asks for the rest of
// this same action and calls back in here with whatever it gets.
func applyCodeAction(e *state.Editor, action lsp.CodeAction) {
	if action.Edit == nil && action.Data != nil {
		resolveCodeAction(e, action)

		return
	}
	if action.Command != nil {
		e.StatusMsg = "code action runs a command, not yet supported: " + action.Title

		return
	}
	if action.Edit == nil {
		e.StatusMsg = "code action has nothing to apply: " + action.Title

		return
	}
	if action.Edit.FileOps {
		e.StatusMsg = "code action creates, renames, or deletes a file, not yet supported: " + action.Title

		return
	}

	textEdits, ok := currentFileEdits(e, *action.Edit)
	if !ok {
		e.StatusMsg = "code action reaches beyond this file, not yet supported: " + action.Title

		return
	}
	if len(textEdits) == 0 {
		e.StatusMsg = "code action has nothing to apply: " + action.Title

		return
	}

	e.BeginChange()
	edit.Splice(e, edits(e, lsp.PositionEncoding(e.SourceFile), textEdits))
	e.EndChange()
	e.StatusMsg = "applied: " + action.Title
}

// resolveCodeAction is what a choice with data instead of an edit is waiting
// on: gopls answers "declare the missing method" this way, holding the edit
// back until codeAction/resolve is asked about this one action specifically.
// The title is kept from the choice actually made in the menu, rather than
// whatever the resolved answer names itself, since a server is only ever
// asked to fill in the edit, not to relabel the row that was chosen.
func resolveCodeAction(e *state.Editor, action lsp.CodeAction) {
	if !lsp.CodeActionResolves(e.SourceFile) {
		e.StatusMsg = "code action has nothing to apply: " + action.Title

		return
	}

	token, from := ask(e)
	lsp.ResolveCodeAction(e.SourceFile, action, func(resolved lsp.CodeAction, err error) {
		if stale(e, token, from) {
			return
		}
		if err != nil {
			e.StatusMsg = "the language server did not answer"

			return
		}

		resolved.Title = action.Title
		applyCodeAction(e, resolved)
	})
}

// currentFileEdits is a WorkspaceEdit narrowed to what v1 can apply: nothing
// at all, the moment anything in it names a file other than the one open,
// since some of a fix landing is worse than none of it.
func currentFileEdits(e *state.Editor, wsEdit lsp.WorkspaceEdit) ([]lsp.TextEdit, bool) {
	uri := lsp.FileURI(e.SourceFile)
	for file := range wsEdit.Changes {
		if file != uri {
			return nil, false
		}
	}

	return wsEdit.Changes[uri], true
}

// diagnosticsAt is what the editor already has cached about path that covers
// row and col, carried back into the server's own units: a quickfix is keyed
// to the diagnostic it fixes, and asking without it is asking for a fix to
// nothing.
func diagnosticsAt(path string, b *buffer.Buffer, row, col int) []lsp.Diagnostic {
	encoding := lsp.PositionEncoding(path)

	var found []lsp.Diagnostic
	for _, note := range diag.Of(path) {
		if !covers(note, row, col) {
			continue
		}
		found = append(found, lsp.Diagnostic{
			Range: lsp.Range{
				Start: encoding.Pos(b, note.Row, note.Col),
				End:   encoding.Pos(b, note.EndRow, note.EndCol),
			},
			Severity: int(note.Severity),
			Source:   note.Source,
			Message:  note.Message,
		})
	}

	return found
}

func covers(n diag.Note, row, col int) bool {
	if row < n.Row || row > n.EndRow {
		return false
	}
	if row == n.Row && col < n.Col {
		return false
	}
	if row == n.EndRow && col >= n.EndCol {
		return false
	}

	return true
}
