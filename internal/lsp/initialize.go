package lsp

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// The subset of the handshake the editor has anything to say about. Everything
// omitted is a capability declined by saying nothing, which is what the
// protocol takes silence for.
type initializeParams struct {
	ProcessID        int               `json:"processId"`
	RootURI          string            `json:"rootUri"`
	Capabilities     clientCapability  `json:"capabilities"`
	WorkspaceFolders []workspaceFolder `json:"workspaceFolders"`
}

type workspaceFolder struct {
	URI  string `json:"uri"`
	Name string `json:"name"`
}

type clientCapability struct {
	General      generalCapability      `json:"general"`
	TextDocument textDocumentCapability `json:"textDocument"`
	Window       windowCapability       `json:"window"`
	Workspace    workspaceCapability    `json:"workspace"`
}

type generalCapability struct {
	PositionEncodings []string `json:"positionEncodings"`
}

type textDocumentCapability struct {
	PublishDiagnostics publishDiagnosticsCapability `json:"publishDiagnostics"`
	Synchronization    syncCapability               `json:"synchronization"`
	Definition         linkCapability               `json:"definition"`
	Hover              hoverCapability              `json:"hover"`
	Completion         completionCapability         `json:"completion"`
}

// contextSupport is what carries why a completion was asked for, which is the
// difference between the whole scope and what may follow a dot.
type completionCapability struct {
	ContextSupport bool                     `json:"contextSupport"`
	CompletionItem completionItemCapability `json:"completionItem"`
}

// The first two are declined rather than left unsaid. A snippet is a template
// with placeholders and tab stops, and there is nothing here to expand one; an
// insert-replace edit is two ranges for the caller to choose between, where one
// is all a candidate settled on ever needs.
//
// resolveSupport is the other way round: it is leave holding back the edits a
// candidate needs elsewhere until it is the one being settled on. Without it a
// server willing to answer a thousand candidates has to work out the import
// line for every one of them, so this is asked for rather than tolerated — and
// it is what typescript-language-server does whether it is asked or not.
type completionItemCapability struct {
	SnippetSupport       bool                     `json:"snippetSupport"`
	InsertReplaceSupport bool                     `json:"insertReplaceSupport"`
	ResolveSupport       resolveSupportCapability `json:"resolveSupport"`
}

type resolveSupportCapability struct {
	Properties []string `json:"properties"`
}

// What a candidate may arrive without. The edits are the only one acted on: a
// doc comment there is nowhere to show and a signature the menu manages without
// are named because rust-analyzer will not offer to resolve anything at all
// unless all three are, and the edits are what its 'use' line comes back in.
var resolvable = []string{"detail", "documentation", "additionalTextEdits"}

type publishDiagnosticsCapability struct {
	VersionSupport bool `json:"versionSupport"`
}

type syncCapability struct {
	DidSave bool `json:"didSave"`
}

// A definition answered as a LocationLink carries a range for the whole
// declaration as well as the name, which is one more thing to get right for an
// answer the editor uses only the start of.
type linkCapability struct {
	LinkSupport bool `json:"linkSupport"`
}

type hoverCapability struct {
	ContentFormat []string `json:"contentFormat"`
}

// Progress is a whole class of server request that exists to drive a spinner
// there is nowhere to draw.
type windowCapability struct {
	WorkDoneProgress bool `json:"workDoneProgress"`
}

type workspaceCapability struct {
	WorkspaceFolders bool `json:"workspaceFolders"`
}

type initializeResult struct {
	Capabilities serverCapability `json:"capabilities"`
}

type serverCapability struct {
	PositionEncoding   string              `json:"positionEncoding"`
	CompletionProvider *completionProvider `json:"completionProvider"`
}

// A server offering completion says so with this, and names the characters it
// would like to be woken on: '.' for Go, and ':' and '\” besides for Rust. They
// are the server's own rather than a list here, which is what lets a language
// added to the tables in sync.go bring its own along with it.
type completionProvider struct {
	TriggerCharacters []string `json:"triggerCharacters"`
	ResolveProvider   bool     `json:"resolveProvider"`
}

func handshake(root string) initializeParams {
	return initializeParams{
		ProcessID: os.Getpid(),
		RootURI:   FileURI(root),
		WorkspaceFolders: []workspaceFolder{{
			URI:  FileURI(root),
			Name: filepath.Base(root),
		}},
		Capabilities: clientCapability{
			General: generalCapability{PositionEncodings: Encodings},
			TextDocument: textDocumentCapability{
				// Without versionSupport there is no way to tell a payload that
				// crossed a newer document on the wire from a current one.
				PublishDiagnostics: publishDiagnosticsCapability{VersionSupport: true},
				Synchronization:    syncCapability{DidSave: true},
				Definition:         linkCapability{LinkSupport: false},
				Hover:              hoverCapability{ContentFormat: []string{"plaintext", "markdown"}},
				Completion: completionCapability{
					ContextSupport: true,
					CompletionItem: completionItemCapability{
						SnippetSupport:       false,
						InsertReplaceSupport: false,
						ResolveSupport:       resolveSupportCapability{Properties: resolvable},
					},
				},
			},
			Window:    windowCapability{WorkDoneProgress: false},
			Workspace: workspaceCapability{WorkspaceFolders: true},
		},
	}
}

// encodingFrom is what the server chose. A server that names neither of the two
// offered has answered with something the editor cannot count columns in, and
// counting them the other way puts every underline and every jump a column out
// in exactly the lines nobody tests — so it is refused rather than guessed at.
func encodingFrom(result json.RawMessage) (Encoding, error) {
	var got initializeResult
	if err := json.Unmarshal(result, &got); err != nil {
		return UTF16, err
	}
	// Saying nothing means UTF-16, which is the protocol's own default.
	if got.Capabilities.PositionEncoding == "" {
		return UTF16, nil
	}

	enc, ok := EncodingNamed(got.Capabilities.PositionEncoding)
	if !ok {
		return UTF16, &UnsupportedEncodingError{Name: got.Capabilities.PositionEncoding}
	}

	return enc, nil
}

type UnsupportedEncodingError struct{ Name string }

func (e *UnsupportedEncodingError) Error() string {
	return "lsp: server chose position encoding " + e.Name + ", which is not one of the offered " +
		Encodings[0] + " or " + Encodings[1]
}

// triggersFrom is what the server said about completion: the characters it
// asked to be woken on, whether it will answer a second question about one
// candidate, and whether it offers completion at all — the last of which is
// what stops it being asked. The triggers are folded to the runes they are so
// that a keystroke is checked against them without decoding anything; the
// protocol has them single characters, and one that is not is dropped rather
// than half-matched.
func triggersFrom(result json.RawMessage) (string, bool, bool) {
	var got initializeResult
	if err := json.Unmarshal(result, &got); err != nil || got.Capabilities.CompletionProvider == nil {
		return "", false, false
	}

	offered := got.Capabilities.CompletionProvider
	triggers := make([]rune, 0, len(offered.TriggerCharacters))
	for _, ch := range offered.TriggerCharacters {
		if runes := []rune(ch); len(runes) == 1 {
			triggers = append(triggers, runes[0])
		}
	}

	return string(triggers), offered.ResolveProvider, true
}
