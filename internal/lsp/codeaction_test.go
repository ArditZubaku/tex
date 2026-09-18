package lsp

import (
	"encoding/json"
	"errors"
	"testing"
)

// codeacting is the package with a server up that has declared code action
// support during its handshake, which is as far as the handshake tests below
// start from — everything else about asking for one needs nothing more than a
// server that is up at all, the same as a definition or a hover.
func codeacting(t *testing.T) string {
	t.Helper()

	h := reconciling(t)
	path := project(t, "main.go", "package main\n")
	b := opening(t, path)

	Sync([]File{{Path: path, Buf: b}})
	h.client = only(t)

	asked := h.server.next()
	if asked.Method != methodInitialize {
		t.Fatalf("first message was %q, want %q", asked.Method, methodInitialize)
	}
	h.server.answer(asked.ID, initializeResult{
		Capabilities: serverCapability{CodeActionProvider: json.RawMessage("true")},
	})
	h.poll()

	if got := h.server.next(); got.Method != methodInitialized {
		t.Fatalf("after the handshake the client sent %q, want %q", got.Method, methodInitialized)
	}
	Sync([]File{{Path: path, Buf: b}})
	openedIn(t, h.sent())

	return path
}

func TestAServerSayingNothingAboutCodeActionsIsNotCodeActing(t *testing.T) {
	_, path := asking(t, "main.go", "package main\n")

	if CodeActing(path) {
		t.Fatal("a server that never mentioned code actions is being asked for them")
	}
}

func TestAServerDeclaringCodeActionSupportIsCodeActing(t *testing.T) {
	path := codeacting(t)

	if !CodeActing(path) {
		t.Fatal("a server that declared code action support is not being asked for any")
	}
}

// The protocol lets codeActionProvider be an object of the server's own
// options instead of a bare true, and a server saying either has still agreed
// to answer.
func TestACodeActionProvidersOwnOptionsStillCountsAsSupport(t *testing.T) {
	h := reconciling(t)
	path := project(t, "main.go", "package main\n")
	b := opening(t, path)
	Sync([]File{{Path: path, Buf: b}})
	h.client = only(t)

	asked := h.server.next()
	h.server.answer(asked.ID, initializeResult{Capabilities: serverCapability{
		CodeActionProvider: json.RawMessage(`{"codeActionKinds":["quickfix"]}`),
	}})
	h.poll()

	if got := h.server.next(); got.Method != methodInitialized {
		t.Fatalf("after the handshake the client sent %q, want %q", got.Method, methodInitialized)
	}
	Sync([]File{{Path: path, Buf: b}})
	openedIn(t, h.sent())

	if !CodeActing(path) {
		t.Fatal("a codeActionProvider sent as an object is not counted as support")
	}
}

func TestACodeActionProviderSayingFalseIsNotCodeActing(t *testing.T) {
	h := reconciling(t)
	path := project(t, "main.go", "package main\n")
	b := opening(t, path)
	Sync([]File{{Path: path, Buf: b}})
	h.client = only(t)

	asked := h.server.next()
	h.server.answer(asked.ID, initializeResult{
		Capabilities: serverCapability{CodeActionProvider: json.RawMessage("false")},
	})
	h.poll()

	if got := h.server.next(); got.Method != methodInitialized {
		t.Fatalf("after the handshake the client sent %q, want %q", got.Method, methodInitialized)
	}
	Sync([]File{{Path: path, Buf: b}})
	openedIn(t, h.sent())

	if CodeActing(path) {
		t.Fatal("a codeActionProvider of false is being asked for code actions")
	}
}

func TestACodeActionsRequestNamesAZeroWidthRangeAndItsDiagnostics(t *testing.T) {
	h, path := asking(t, "main.go", "package main\n")
	b := opening(t, path)

	CodeActions(path, b, 0, 8, []Diagnostic{{Message: "undefined: foo"}}, func([]CodeAction, error) {})

	asked := h.sent()
	if asked.Method != methodCodeAction {
		t.Fatalf("the server was asked %q, want %q", asked.Method, methodCodeAction)
	}

	var params codeActionParams
	if err := json.Unmarshal(asked.Params, &params); err != nil {
		t.Fatal(err)
	}
	if params.Range.Start != params.Range.End || params.Range.Start.Character != 8 {
		t.Errorf("the range asked for was %v, want a zero-width range at character 8", params.Range)
	}
	if len(params.Context.Diagnostics) != 1 || params.Context.Diagnostics[0].Message != "undefined: foo" {
		t.Errorf("the diagnostics at the cursor came back as %v", params.Context.Diagnostics)
	}
}

func TestCodeActionsComeBackInTheOrderTheServerSentThem(t *testing.T) {
	h, path := asking(t, "main.go", "package main\n")
	b := opening(t, path)

	var got []CodeAction
	CodeActions(path, b, 0, 8, nil, func(found []CodeAction, _ error) { got = found })

	h.server.answer(h.sent().ID, []CodeAction{
		{Title: "Organize imports", Kind: KindSourceOrganizeImports},
		{Title: "Add missing method", Kind: KindQuickFix},
	})
	h.poll()

	if len(got) != 2 || got[0].Title != "Organize imports" || got[1].Title != "Add missing method" {
		t.Fatalf("the actions came back as %v", got)
	}
}

func TestNoCodeActionsIsNoComplaint(t *testing.T) {
	h, path := asking(t, "main.go", "package main\n")
	b := opening(t, path)

	var got []CodeAction
	failed := ErrStopped
	CodeActions(path, b, 0, 8, nil, func(found []CodeAction, err error) { got, failed = found, err })

	h.server.answer(h.sent().ID, nil)
	h.poll()

	if failed != nil {
		t.Fatalf("a server offering nothing reported %v", failed)
	}
	if got != nil {
		t.Errorf("nothing offered came back as %v", got)
	}
}

// A result is (Command | CodeAction)[]: a server is free to send the bare
// shape asked to be handed to workspace/executeCommand instead of the literal
// everything here reads. Its "command" is a string naming the command rather
// than the object a CodeAction's own edit lives on, so it fails to decode as
// one and is dropped rather than misread as an edit-less action.
func TestABareCommandAmongTheResultsIsDropped(t *testing.T) {
	h, path := asking(t, "main.go", "package main\n")
	b := opening(t, path)

	var got []CodeAction
	CodeActions(path, b, 0, 8, nil, func(found []CodeAction, _ error) { got = found })

	h.server.answer(h.sent().ID, []any{
		CodeAction{Title: "Add missing method", Edit: &WorkspaceEdit{}},
		Command{Title: "Run test", Command: "gopls.test"},
	})
	h.poll()

	if len(got) != 1 || got[0].Title != "Add missing method" {
		t.Fatalf("the actions came back as %v, want the bare command dropped", got)
	}
}

func TestACodeActionWithNoTitleIsDropped(t *testing.T) {
	h, path := asking(t, "main.go", "package main\n")
	b := opening(t, path)

	var got []CodeAction
	CodeActions(path, b, 0, 8, nil, func(found []CodeAction, _ error) { got = found })

	h.server.answer(h.sent().ID, []json.RawMessage{json.RawMessage(`{"kind":"refactor.extract"}`)})
	h.poll()

	if len(got) != 0 {
		t.Errorf("a titleless action came back as %v", got)
	}
}

func TestAWorkspaceEditKeepsItsChangesPerFile(t *testing.T) {
	h, path := asking(t, "main.go", "package main\n")
	b := opening(t, path)

	var got []CodeAction
	CodeActions(path, b, 0, 8, nil, func(found []CodeAction, _ error) { got = found })

	h.server.answer(h.sent().ID, []CodeAction{{
		Title: "Add the import",
		Edit: &WorkspaceEdit{Changes: map[string][]TextEdit{
			"file:///a.go": {{NewText: "x"}},
			"file:///b.go": {{NewText: "y"}, {NewText: "z"}},
		}},
	}})
	h.poll()

	if len(got) != 1 || got[0].Edit == nil {
		t.Fatalf("the action came back as %v", got)
	}
	changes := got[0].Edit.Changes
	if len(changes["file:///a.go"]) != 1 || len(changes["file:///b.go"]) != 2 {
		t.Errorf("the changes came back as %v", changes)
	}
}

// A server's documentChanges is the richer, versioned shape of the same edit —
// gopls answers codeAction/resolve in it regardless of whether the handshake
// ever offered workspace.workspaceEdit.documentChanges, so it is read into the
// same Changes a plain "changes" object already fills.
func TestAWorkspaceEditsDocumentChangesAreFoldedIntoChanges(t *testing.T) {
	var got CodeAction
	raw := `{
		"title": "Add the import",
		"edit": {
			"documentChanges": [{
				"textDocument": {"uri": "file:///a.go", "version": 2},
				"edits": [{"newText": "x"}, {"newText": "y"}]
			}]
		}
	}`
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatal(err)
	}

	if got.Edit == nil || len(got.Edit.Changes["file:///a.go"]) != 2 {
		t.Fatalf("the changes came back as %v", got.Edit)
	}
	if got.Edit.FileOps {
		t.Error("a plain text edit among documentChanges was read as a file operation")
	}
}

// A create, rename, or delete is named by a "kind" a TextDocumentEdit never
// carries, which is what tells the two apart without a whole second type for
// the three operations nothing here otherwise does anything with.
func TestAFileOperationAmongDocumentChangesSetsFileOps(t *testing.T) {
	var got CodeAction
	raw := `{
		"title": "Extract to new file",
		"edit": {
			"documentChanges": [
				{"kind": "create", "uri": "file:///new.go"},
				{"textDocument": {"uri": "file:///a.go", "version": 1}, "edits": [{"newText": "x"}]}
			]
		}
	}`
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatal(err)
	}

	if !got.Edit.FileOps {
		t.Error("a create among documentChanges was not read as a file operation")
	}
	if len(got.Edit.Changes["file:///a.go"]) != 1 {
		t.Errorf("the plain edit alongside it came back as %v", got.Edit.Changes)
	}
}

// resolvingCodeActions is codeacting with resolveProvider also declared, which
// is what a server holding "declare the missing method" back as data instead
// of an edit needs a follow-up answered.
func resolvingCodeActions(t *testing.T) (*harness, string) {
	t.Helper()

	h := reconciling(t)
	path := project(t, "main.go", "package main\n")
	b := opening(t, path)

	Sync([]File{{Path: path, Buf: b}})
	h.client = only(t)

	asked := h.server.next()
	h.server.answer(asked.ID, initializeResult{
		Capabilities: serverCapability{CodeActionProvider: json.RawMessage(`{"resolveProvider":true}`)},
	})
	h.poll()

	if got := h.server.next(); got.Method != methodInitialized {
		t.Fatalf("after the handshake the client sent %q, want %q", got.Method, methodInitialized)
	}
	Sync([]File{{Path: path, Buf: b}})
	openedIn(t, h.sent())

	return h, path
}

func TestACodeActionProviderDeclaringResolveProviderIsCodeActionResolving(t *testing.T) {
	_, path := resolvingCodeActions(t)

	if !CodeActionResolves(path) {
		t.Fatal("a server declaring resolveProvider is not being asked to resolve anything")
	}
}

func TestACodeActionProviderSayingNothingAboutResolveIsNotCodeActionResolving(t *testing.T) {
	path := codeacting(t)

	if CodeActionResolves(path) {
		t.Fatal("a server that never mentioned resolveProvider is being asked to resolve something")
	}
}

func TestResolveCodeActionSendsTheActionBackAndReadsTheEditItAnswersWith(t *testing.T) {
	h, path := resolvingCodeActions(t)

	action := CodeAction{
		Title: "Declare missing method Thing.Missing",
		Kind:  KindQuickFix,
		Data:  json.RawMessage(`{"command":"gopls.apply_fix","arguments":[{"Fix":"stub_missing_called_function"}]}`),
	}

	var got CodeAction
	failed := ErrStopped
	ResolveCodeAction(path, action, func(resolved CodeAction, err error) { got, failed = resolved, err })

	asked := h.sent()
	if asked.Method != methodCodeActionResolve {
		t.Fatalf("the server was asked %q, want %q", asked.Method, methodCodeActionResolve)
	}

	var sentBack CodeAction
	if err := json.Unmarshal(asked.Params, &sentBack); err != nil {
		t.Fatal(err)
	}
	if string(sentBack.Data) != string(action.Data) {
		t.Errorf("the action sent back carried data %s, want %s", sentBack.Data, action.Data)
	}

	h.server.answer(asked.ID, json.RawMessage(`{
		"title": "Declare missing method Thing.Missing",
		"edit": {
			"documentChanges": [{
				"textDocument": {"uri": "`+FileURI(path)+`", "version": 1},
				"edits": [{"newText": "x"}]
			}]
		}
	}`))
	h.poll()

	if failed != nil {
		t.Fatalf("resolving reported %v", failed)
	}
	if got.Edit == nil {
		t.Fatal("the resolved action carried no edit")
	}
	if edits := got.Edit.Changes[FileURI(path)]; len(edits) != 1 || edits[0].NewText != "x" {
		t.Errorf("the resolved edit came back as %v", got.Edit.Changes)
	}
}

func TestResolveCodeActionWithNoServerSupportIsRefusedRatherThanSent(t *testing.T) {
	path := codeacting(t)

	var failed error
	ResolveCodeAction(path, CodeAction{Title: "Declare missing method"}, func(_ CodeAction, err error) { failed = err })

	if !errors.Is(failed, ErrStopped) {
		t.Errorf("resolving without resolveProvider reported %v, want %v", failed, ErrStopped)
	}
}
