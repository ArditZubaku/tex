package lsp

import (
	"encoding/json"

	"github.com/ArditZubaku/tex/internal/buffer"
)

const (
	methodCodeAction        = "textDocument/codeAction"
	methodCodeActionResolve = "codeAction/resolve"
)

type CodeActionKind string

const (
	KindQuickFix              CodeActionKind = "quickfix"
	KindRefactor              CodeActionKind = "refactor"
	KindRefactorExtract       CodeActionKind = "refactor.extract"
	KindRefactorInline        CodeActionKind = "refactor.inline"
	KindRefactorRewrite       CodeActionKind = "refactor.rewrite"
	KindSource                CodeActionKind = "source"
	KindSourceOrganizeImports CodeActionKind = "source.organizeImports"
)

var codeActionKinds = []CodeActionKind{
	KindQuickFix, KindRefactor, KindRefactorExtract, KindRefactorInline,
	KindRefactorRewrite, KindSource, KindSourceOrganizeImports,
}

// A Command is the protocol's other shape for what a code action runs, asked
// for through workspace/executeCommand — which nothing here sends, so a
// CodeAction naming one rather than an Edit is offered in the menu but refused
// if it is chosen.
type Command struct {
	Title     string          `json:"title"`
	Command   string          `json:"command"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// A WorkspaceEdit is a fix spread across however many files a server likes.
// changes is the plain map this used to be the whole of, keyed by the URI of
// each file. documentChanges is the richer, versioned array: gopls answers
// codeAction/resolve in that shape regardless of whether
// workspace.workspaceEdit.documentChanges was ever declared, so it is read
// too, and folded into the same Changes a caller already knows how to walk.
//
// A create, rename, or delete among documentChanges is not a text edit at
// all, and FileOps says one was there: v1 has nothing to do with one but
// refuse the action whole, the same as an edit that reaches another file.
type WorkspaceEdit struct {
	Changes map[string][]TextEdit `json:"changes"`
	FileOps bool                  `json:"-"`
}

func (w *WorkspaceEdit) UnmarshalJSON(data []byte) error {
	var shape struct {
		Changes         map[string][]TextEdit `json:"changes"`
		DocumentChanges []json.RawMessage     `json:"documentChanges"`
	}
	if err := json.Unmarshal(data, &shape); err != nil {
		return err
	}

	w.Changes = shape.Changes
	for _, raw := range shape.DocumentChanges {
		var op struct {
			Kind string `json:"kind"`
		}
		_ = json.Unmarshal(raw, &op) // malformed leaves Kind empty, so it is tried as an edit next
		if op.Kind != "" {
			w.FileOps = true

			continue
		}

		var edit struct {
			TextDocument ident      `json:"textDocument"`
			Edits        []TextEdit `json:"edits"`
		}
		if err := json.Unmarshal(raw, &edit); err != nil {
			continue
		}
		if w.Changes == nil {
			w.Changes = map[string][]TextEdit{}
		}
		w.Changes[edit.TextDocument.URI] = append(w.Changes[edit.TextDocument.URI], edit.Edits...)
	}

	return nil
}

// A CodeAction is one fix or refactoring a server offers at a position: a title
// for the menu, and an Edit to apply, a Command to run, or Data naming neither
// yet — a server holding the edit back until codeAction/resolve is asked about
// this one action specifically, the shape gopls answers most of its own
// quickfixes in.
type CodeAction struct {
	Title       string          `json:"title"`
	Kind        CodeActionKind  `json:"kind"`
	IsPreferred bool            `json:"isPreferred"`
	Edit        *WorkspaceEdit  `json:"edit"`
	Command     *Command        `json:"command"`
	Data        json.RawMessage `json:"data,omitempty"`
}

type codeActionParams struct {
	TextDocument ident             `json:"textDocument"`
	Range        Range             `json:"range"`
	Context      codeActionContext `json:"context"`
}

type codeActionContext struct {
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// CodeActions is what a server offers at row and col — gopls's "create the
// missing method" among them, which needs nothing special here: it is a
// quickfix like any other, tied to the diagnostic that named the method
// missing. diagnostics is the server's own notes covering that position,
// without which a quickfix keyed to one has nothing to key to.
func CodeActions(
	path string, b *buffer.Buffer, row, col int, diagnostics []Diagnostic, answer func([]CodeAction, error),
) {
	at := PositionEncoding(path).Pos(b, row, col)
	askServer(path, methodCodeAction, codeActionParams{
		TextDocument: ident{URI: FileURI(path)},
		Range:        Range{Start: at, End: at},
		Context:      codeActionContext{Diagnostics: diagnostics},
	}, func(result json.RawMessage, err error) {
		if err != nil {
			answer(nil, err)

			return
		}
		answer(codeActionsFrom(result))
	})
}

// A result is (Command | CodeAction)[] in the protocol's own words: a server
// free to send either shape in the same list. The bare Command is dropped
// rather than read, since every server this talks to sends the literal in
// preference to it once codeActionLiteralSupport has been declared.
func codeActionsFrom(result json.RawMessage) ([]CodeAction, error) {
	if empty(result) {
		return nil, nil
	}

	var raw []json.RawMessage
	if err := json.Unmarshal(result, &raw); err != nil {
		return nil, err
	}

	actions := make([]CodeAction, 0, len(raw))
	for _, one := range raw {
		var action CodeAction
		if err := json.Unmarshal(one, &action); err != nil {
			continue
		}
		if action.Title == "" {
			continue
		}
		actions = append(actions, action)
	}

	return actions, nil
}

// CodeActionResolves says whether the server holding this file fills in the
// edit of a code action sent back with data instead, which is what a choice
// with no edit of its own is waiting on before it can be applied.
func CodeActionResolves(path string) bool {
	doc := held(path)

	return doc != nil && doc.srv.client.Ready() && doc.srv.client.CodeActionResolves()
}

// ResolveCodeAction is the rest of one action: the edit a server held back
// until it knew which one was being applied. It is asked with the action
// exactly as it came back, data included, since that is a server's own
// bookmark for which one this is.
func ResolveCodeAction(path string, action CodeAction, answer func(CodeAction, error)) {
	doc := held(path)
	if doc == nil || !doc.srv.client.Ready() || !doc.srv.client.CodeActionResolves() {
		answer(CodeAction{}, ErrStopped)

		return
	}

	err := doc.srv.client.Request(methodCodeActionResolve, action, func(result json.RawMessage, err error) {
		if err != nil {
			answer(CodeAction{}, err)

			return
		}
		answer(resolvedActionFrom(result))
	})
	if err != nil {
		answer(CodeAction{}, err)
	}
}

func resolvedActionFrom(result json.RawMessage) (CodeAction, error) {
	if empty(result) {
		return CodeAction{}, nil
	}

	var action CodeAction
	if err := json.Unmarshal(result, &action); err != nil {
		return CodeAction{}, err
	}

	return action, nil
}
