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
}

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
	PositionEncoding string `json:"positionEncoding"`
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
