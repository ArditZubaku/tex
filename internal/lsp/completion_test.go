package lsp

import (
	"encoding/json"
	"strings"
	"testing"
)

// completing is the package with a server up that offers completion, which is
// the handshake plus one document open — as far as every test below starts
// from.
func completing(t *testing.T) (*harness, string) {
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
	h.server.answer(asked.ID, initializeResult{Capabilities: serverCapability{
		CompletionProvider: &completionProvider{
			TriggerCharacters: []string{".", "::"},
			ResolveProvider:   true,
		},
	}})
	h.poll()

	if got := h.server.next(); got.Method != methodInitialized {
		t.Fatalf("after the handshake the client sent %q, want %q", got.Method, methodInitialized)
	}
	Sync([]File{{Path: path, Buf: b}})
	openedIn(t, h.sent())

	return h, path
}

func TestAServerOfferingNoCompletionIsNeverAskedForAny(t *testing.T) {
	h := reconciling(t)
	path := project(t, "main.go", "package main\n")
	b := opening(t, path)
	h.up(File{Path: path, Buf: b})
	openedIn(t, h.sent())

	if Completing(path) {
		t.Fatal("a server that never mentioned completion is being asked for it")
	}

	failed := error(nil)
	Complete(path, b, 0, 8, Invoked, "", func(_ Completion, err error) { failed = err })

	if failed == nil {
		t.Error("the request went out to a server that does not answer it")
	}
}

func TestTheTriggerCharactersAreTheServersOwn(t *testing.T) {
	_, path := completing(t)

	if !Completing(path) {
		t.Fatal("a server offering completion is not being asked for it")
	}
	if !TriggerRune(path, '.') {
		t.Error("the '.' the server asked to be woken on does not wake it")
	}
	// "::" is two runes, which the protocol has no place for and half of which
	// would fire on every ':' in a map literal.
	if TriggerRune(path, ':') {
		t.Error("half of a two-character trigger was taken for a trigger")
	}
	if TriggerRune(path, 'x') {
		t.Error("a character the server never named wakes it")
	}
}

func TestACompletionSaysWhyItWasAskedFor(t *testing.T) {
	h, path := completing(t)
	b := opening(t, path)

	Complete(path, b, 0, 8, TriggerChar, ".", func(Completion, error) {})

	asked := h.sent()
	if asked.Method != methodCompletion {
		t.Fatalf("the server was asked %q, want %q", asked.Method, methodCompletion)
	}

	var params completionParams
	if err := json.Unmarshal(asked.Params, &params); err != nil {
		t.Fatal(err)
	}
	if params.Context.TriggerKind != TriggerChar {
		t.Errorf("the request named trigger kind %d, want %d", params.Context.TriggerKind, TriggerChar)
	}
	if params.Context.TriggerCharacter != "." {
		t.Errorf("the request named trigger character %q, want %q", params.Context.TriggerCharacter, ".")
	}
	if params.Position.Character != 8 {
		t.Errorf("the request named character %d, want 8", params.Position.Character)
	}
}

// The reconciler holds a change back for a fifth of a second. A completion that
// waited with it would be answered out of the text as it was two keystrokes
// ago, which is a menu for the word that was there before.
func TestACompletionSendsTheTypingThatHasNotGoneOutYet(t *testing.T) {
	h, path := completing(t)
	b := opening(t, path)
	b.InsertRune(0, 12, 'x')

	Complete(path, b, 0, 13, Invoked, "", func(Completion, error) {})

	sent := h.sent()
	if sent.Method != methodDidChange {
		t.Fatalf("the server was sent %q before the request, want %q", sent.Method, methodDidChange)
	}

	var params didChangeParams
	if err := json.Unmarshal(sent.Params, &params); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(params.ContentChanges[0].Text, "package mainx") {
		t.Errorf("the server was sent %q, without the rune just typed", params.ContentChanges[0].Text)
	}
	if got := h.sent(); got.Method != methodCompletion {
		t.Errorf("the request after it was %q, want %q", got.Method, methodCompletion)
	}
}

func TestADocumentAlreadySentIsNotSentAgainToAskAboutIt(t *testing.T) {
	h, path := completing(t)
	b := opening(t, path)

	Complete(path, b, 0, 8, Invoked, "", func(Completion, error) {})

	if got := h.sent(); got.Method != methodCompletion {
		t.Errorf("the first message was %q, want %q", got.Method, methodCompletion)
	}
}

func TestCandidatesComeBackInTheServersOwnRanking(t *testing.T) {
	h, path := completing(t)
	b := opening(t, path)

	var got Completion
	Complete(path, b, 0, 8, Invoked, "", func(c Completion, _ error) { got = c })

	h.server.answer(h.sent().ID, completionList{IsIncomplete: true, Items: []completionItem{
		{Label: "Println", SortText: "00002"},
		{Label: "Print", SortText: "00001"},
		{Label: "Printf", SortText: "00003"},
	}})
	h.poll()

	if !got.Incomplete {
		t.Error("a list the server said was cut short came back as a whole one")
	}
	if names := labels(got.Items); names != "Print,Println,Printf" {
		t.Errorf("the candidates came back as %q, want the sortText order", names)
	}
}

// A server sending no sortText has said nothing about the order, and the label
// is what the protocol falls back to.
func TestCandidatesWithNothingToOrderThemAreOrderedByName(t *testing.T) {
	h, path := completing(t)
	b := opening(t, path)

	var got Completion
	Complete(path, b, 0, 8, Invoked, "", func(c Completion, _ error) { got = c })

	h.server.answer(h.sent().ID, []completionItem{{Label: "beta"}, {Label: "alpha"}})
	h.poll()

	if names := labels(got.Items); names != "alpha,beta" {
		t.Errorf("the candidates came back as %q, want %q", names, "alpha,beta")
	}
}

func TestAnAnswerAsABareArrayIsStillCandidates(t *testing.T) {
	h, path := completing(t)
	b := opening(t, path)

	var got Completion
	Complete(path, b, 0, 8, Invoked, "", func(c Completion, _ error) { got = c })

	h.server.answer(h.sent().ID, []completionItem{{Label: "one"}})
	h.poll()

	if len(got.Items) != 1 || got.Items[0].Label != "one" {
		t.Fatalf("a bare array came back as %v", got.Items)
	}
	if got.Incomplete {
		t.Error("a shape that cannot say it was cut short came back saying so")
	}
}

func TestNothingOfferedIsNoCandidatesAndNoComplaint(t *testing.T) {
	h, path := completing(t)
	b := opening(t, path)

	var got Completion
	failed := ErrStopped
	Complete(path, b, 0, 8, Invoked, "", func(c Completion, err error) { got, failed = c, err })

	h.server.answer(h.sent().ID, nil)
	h.poll()

	if failed != nil {
		t.Fatalf("a server offering nothing reported %v", failed)
	}
	if len(got.Items) != 0 {
		t.Errorf("nothing offered came back as %v", got.Items)
	}
}

func TestWhatGoesInIsTheEditTheServerNamedRatherThanTheLabel(t *testing.T) {
	h, path := completing(t)
	b := opening(t, path)

	var got Completion
	Complete(path, b, 0, 8, Invoked, "", func(c Completion, _ error) { got = c })

	at := Range{Start: Position{Line: 0, Character: 5}, End: Position{Line: 0, Character: 8}}
	h.server.answer(h.sent().ID, []completionItem{{
		Label:      "Println",
		InsertText: "ignored",
		TextEdit:   &TextEdit{Range: at, NewText: "Println"},
		AdditionalTextEdits: []TextEdit{{
			Range:   Range{Start: Position{Line: 1}, End: Position{Line: 1}},
			NewText: "import \"fmt\"\n",
		}},
	}})
	h.poll()

	if got.Items[0].Text != "Println" {
		t.Errorf("what goes in is %q, want the edit's own text", got.Items[0].Text)
	}
	if got.Items[0].Edit == nil || got.Items[0].Edit.Start.Character != 5 {
		t.Errorf("the range it replaces came back as %v", got.Items[0].Edit)
	}
	if len(got.Items[0].Extra) != 1 {
		t.Fatalf("the import the candidate needs came back as %v", got.Items[0].Extra)
	}
}

// The handshake declines snippets. One that arrives anyway is a template full
// of placeholders there is nothing here to expand.
func TestASnippetSentAnywayGoesInAsThePlainName(t *testing.T) {
	h, path := completing(t)
	b := opening(t, path)

	var got Completion
	Complete(path, b, 0, 8, Invoked, "", func(c Completion, _ error) { got = c })

	h.server.answer(h.sent().ID, []completionItem{{
		Label:            "Println",
		InsertText:       "Println(${1:a})$0",
		InsertTextFormat: snippet,
	}})
	h.poll()

	if got.Items[0].Text != "Println" {
		t.Errorf("a snippet went in as %q, want the plain name", got.Items[0].Text)
	}
}

func labels(items []Item) string {
	names := make([]string, 0, len(items))
	for _, one := range items {
		names = append(names, one.Label)
	}

	return strings.Join(names, ",")
}

// A server willing to answer a thousand candidates cannot work out the import
// line for every one of them, so it holds them back until one is settled on —
// which is what typescript-language-server does with every auto-import it
// offers.
func TestTheImportHeldBackIsAskedForByItself(t *testing.T) {
	h, path := completing(t)

	if !Resolves(path) {
		t.Fatal("a server that said it resolves is not being asked to")
	}

	var got []TextEdit
	Resolve(path, Item{Label: "computeTotal", Data: json.RawMessage(`{"entry":7}`)},
		func(extra []TextEdit, _ error) { got = extra })

	asked := h.sent()
	if asked.Method != methodResolve {
		t.Fatalf("the server was asked %q, want %q", asked.Method, methodResolve)
	}

	var quoted completionItem
	if err := json.Unmarshal(asked.Params, &quoted); err != nil {
		t.Fatal(err)
	}
	if quoted.Label != "computeTotal" || string(quoted.Data) != `{"entry":7}` {
		t.Errorf("the candidate went back as %+v, without the server's own bookmark", quoted)
	}
	// A kind of zero is a kind the protocol does not have, and a server reading
	// its own candidate back is entitled to be confused by one.
	if strings.Contains(string(asked.Params), `"kind"`) {
		t.Errorf("the candidate went back as %s, naming a kind it never had", asked.Params)
	}

	h.server.answer(asked.ID, completionItem{
		Label: "computeTotal",
		AdditionalTextEdits: []TextEdit{{
			Range:   Range{Start: Position{Line: 0}, End: Position{Line: 0}},
			NewText: "import { computeTotal } from \"./helper\";\n\n",
		}},
	})
	h.poll()

	if len(got) != 1 || !strings.Contains(got[0].NewText, "computeTotal") {
		t.Fatalf("the import came back as %v", got)
	}
}

func TestAServerThatResolvesNothingIsNoComplaint(t *testing.T) {
	h, path := completing(t)

	var got []TextEdit
	failed := ErrStopped
	Resolve(path, Item{Label: "x"}, func(extra []TextEdit, err error) { got, failed = extra, err })

	h.server.answer(h.sent().ID, nil)
	h.poll()

	if failed != nil {
		t.Fatalf("a candidate with nothing more to it reported %v", failed)
	}
	if got != nil {
		t.Errorf("nothing more came back as %v", got)
	}
}

// A server that never said it resolves is one whose candidates are whole
// already, and asking it is a round trip for an answer it has no method for.
func TestAServerThatDoesNotResolveIsNeverAsked(t *testing.T) {
	h := reconciling(t)
	path := project(t, "main.go", "package main\n")
	b := opening(t, path)
	h.up(File{Path: path, Buf: b})
	openedIn(t, h.sent())

	if Resolves(path) {
		t.Fatal("a server that never mentioned resolving is being asked to")
	}

	failed := error(nil)
	Resolve(path, Item{Label: "x"}, func(_ []TextEdit, err error) { failed = err })

	if failed == nil {
		t.Error("the request went out to a server that does not answer it")
	}
}
