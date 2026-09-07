package lsp

import (
	"encoding/json"
	"strings"

	"github.com/ArditZubaku/tex/internal/buffer"
)

// The lookups a server answers rather than publishes. Each is a request whose
// answer arrives some frames later, so it is handed to a callback instead of
// returned: the loop cannot wait for it, and by the time it comes the cursor
// may be somewhere else entirely.

const (
	methodDefinition      = "textDocument/definition"
	methodReferences      = "textDocument/references"
	methodDocumentSymbol  = "textDocument/documentSymbol"
	methodWorkspaceSymbol = "workspace/symbol"
	methodHover           = "textDocument/hover"
)

// A Location is somewhere in a file as a server names it: a URI, and a range
// counted in whichever encoding the handshake settled on.
type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

// A Symbol is one declaration a server knows about. The protocol has two shapes
// for it and a server picks which to send — a documentSymbol carries the name's
// own selectionRange, a symbolInformation a whole location — so both are read
// and At says which one came.
type Symbol struct {
	Name           string   `json:"name"`
	Detail         string   `json:"detail"`
	Kind           int      `json:"kind"`
	Location       Location `json:"location"`
	SelectionRange Range    `json:"selectionRange"`
	Children       []Symbol `json:"children"`
}

// At is where the symbol's own name starts, and In the file holding it — empty
// for the shape that leaves the file to the caller, which asked about one file.
func (s Symbol) At() Position {
	if s.Location.URI != "" {
		return s.Location.Range.Start
	}

	return s.SelectionRange.Start
}

func (s Symbol) In() string { return Path(s.Location.URI) }

// A hover is what a server has to say about what is under the cursor. Its
// contents are one of the three shapes the protocol has grown through — a
// marked-up block, a string, or a list of either — so the text is pulled out of
// whichever came rather than decoded into one of them.
type hover struct {
	Contents json.RawMessage `json:"contents"`
}

type positionParams struct {
	TextDocument ident    `json:"textDocument"`
	Position     Position `json:"position"`
}

type referenceParams struct {
	TextDocument ident            `json:"textDocument"`
	Position     Position         `json:"position"`
	Context      referenceContext `json:"context"`
}

// The declaration is asked for along with the uses of it, since 'gr' has always
// listed the line a name is declared on among the lines mentioning it.
type referenceContext struct {
	IncludeDeclaration bool `json:"includeDeclaration"`
}

type symbolQuery struct {
	Query string `json:"query"`
}

// Definition is where the name at row and col is declared. The buffer is read
// here because how far along a line a rune index falls is what the server is
// told, and that depends on every rune before it.
func Definition(path string, b *buffer.Buffer, row, col int, answer func([]Location, error)) {
	locate(path, methodDefinition, positionParams{
		TextDocument: ident{URI: FileURI(path)},
		Position:     PositionEncoding().Pos(b, row, col),
	}, answer)
}

// References is everywhere that name is mentioned, the declaration included.
func References(path string, b *buffer.Buffer, row, col int, answer func([]Location, error)) {
	locate(path, methodReferences, referenceParams{
		TextDocument: ident{URI: FileURI(path)},
		Position:     PositionEncoding().Pos(b, row, col),
		Context:      referenceContext{IncludeDeclaration: true},
	}, answer)
}

// DocumentSymbols is every declaration in one file.
func DocumentSymbols(path string, answer func([]Symbol, error)) {
	askServer(path, methodDocumentSymbol, identParams{TextDocument: ident{URI: FileURI(path)}},
		func(result json.RawMessage, err error) {
			if err != nil {
				answer(nil, err)

				return
			}
			answer(symbolsFrom(result))
		})
}

// WorkspaceSymbols is the same widened to the project. The query is the
// server's own filter, and an empty one is how much of the project a server is
// willing to list at once — gopls answers it in full, and a server that will
// not is what the text-based listing is still there for.
func WorkspaceSymbols(path, query string, answer func([]Symbol, error)) {
	askServer(path, methodWorkspaceSymbol, symbolQuery{Query: query},
		func(result json.RawMessage, err error) {
			if err != nil {
				answer(nil, err)

				return
			}
			answer(symbolsFrom(result))
		})
}

// Hover is what the server knows about the identifier at row and col: a doc
// comment, a signature, a type — as markup, in whichever of the protocol's
// three shapes for it the server chose.
func Hover(path string, b *buffer.Buffer, row, col int, answer func(string, error)) {
	askServer(path, methodHover, positionParams{
		TextDocument: ident{URI: FileURI(path)},
		Position:     PositionEncoding().Pos(b, row, col),
	}, func(result json.RawMessage, err error) {
		if err != nil {
			answer("", err)

			return
		}
		answer(hoverFrom(result))
	})
}

func locate(path, method string, params any, answer func([]Location, error)) {
	askServer(path, method, params, func(result json.RawMessage, err error) {
		if err != nil {
			answer(nil, err)

			return
		}
		answer(locationsFrom(result))
	})
}

// askServer is the one place a lookup is refused for want of a server, so that
// every caller has exactly one path back: the callback, with an error.
func askServer(path, method string, params any, answer func(json.RawMessage, error)) {
	if !Ready(path) {
		answer(nil, ErrStopped)

		return
	}
	if err := client.Request(method, params, answer); err != nil {
		answer(nil, err)
	}
}

// A definition is answered as one location, a list of them, or nothing at all,
// and which of the three is the server's choice rather than the request's.
func locationsFrom(result json.RawMessage) ([]Location, error) {
	if empty(result) {
		return nil, nil
	}

	var many []Location
	if err := json.Unmarshal(result, &many); err == nil {
		return many, nil
	}

	var one Location
	if err := json.Unmarshal(result, &one); err != nil {
		return nil, err
	}

	return []Location{one}, nil
}

// The nested shape is flattened because the popup is a list: a struct's fields
// and an interface's methods are rows of their own, in the order they were sent,
// which is the order they appear in the file.
func symbolsFrom(result json.RawMessage) ([]Symbol, error) {
	if empty(result) {
		return nil, nil
	}

	var tree []Symbol
	if err := json.Unmarshal(result, &tree); err != nil {
		return nil, err
	}

	return flattened(nil, tree), nil
}

func flattened(into, symbols []Symbol) []Symbol {
	for _, one := range symbols {
		children := one.Children
		one.Children = nil
		into = flattened(append(into, one), children)
	}

	return into
}

func empty(result json.RawMessage) bool {
	return len(result) == 0 || string(result) == "null"
}

func hoverFrom(result json.RawMessage) (string, error) {
	if empty(result) {
		return "", nil
	}

	var got hover
	if err := json.Unmarshal(result, &got); err != nil {
		return "", err
	}

	return markupText(got.Contents), nil
}

// A list is joined with blank lines between its blocks, which is how a server
// that still sends one means it to read.
func markupText(contents json.RawMessage) string {
	if empty(contents) {
		return ""
	}

	var many []json.RawMessage
	if err := json.Unmarshal(contents, &many); err == nil {
		blocks := make([]string, 0, len(many))
		for _, one := range many {
			if text := markupText(one); text != "" {
				blocks = append(blocks, text)
			}
		}

		return strings.Join(blocks, "\n\n")
	}

	var text string
	if err := json.Unmarshal(contents, &text); err == nil {
		return text
	}

	var block struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal(contents, &block); err != nil {
		return ""
	}

	return block.Value
}
