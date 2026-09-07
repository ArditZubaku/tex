package find

import (
	"fmt"
	"path/filepath"

	"github.com/ArditZubaku/tex/internal/decl"
	"github.com/ArditZubaku/tex/internal/editor/picker"
	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/ArditZubaku/tex/internal/lsp"
	"github.com/ArditZubaku/tex/internal/project"
)

// '<leader>ss' is LazyVim's document symbols: the declarations of the file being
// edited, listed in the popup in the order they appear. A language server knows
// them properly and is asked when there is one; failing that they are
// recognised in the text the way 'gd' recognises a declaration.

// A file with more declarations than this has more than anybody scrolls
// through, and the filter is what finds the one being looked for anyway.
const maxSymbols = 2000

func OpenSymbols(e *state.Editor) {
	if lsp.Ready(e.SourceFile) {
		askSymbols(e)

		return
	}

	symbolsInText(e)
}

func askSymbols(e *state.Editor) {
	token, from := ask(e)
	path := e.SourceFile
	lsp.DocumentSymbols(path, func(found []lsp.Symbol, err error) {
		if !stale(e, token, from) {
			listSymbols(e, path, found, err)
		}
	})
}

// A file a server has nothing to say about is a file with no declarations in
// it, which is what the message says: reading the text after that would only
// turn up lines a parser has already decided were not declarations.
func listSymbols(e *state.Editor, path string, found []lsp.Symbol, err error) {
	if err != nil {
		symbolsInText(e)

		return
	}

	entries := symbolRows(path, found[:min(len(found), maxSymbols)])
	if len(entries) == 0 {
		e.StatusMsg = "no symbols in " + filepath.Base(path)

		return
	}

	showPicker(e, "Symbols in "+filepath.Base(path), entries)
}

func symbolsInText(e *state.Editor) {
	entries := bufferSymbols(e)
	if len(entries) == 0 {
		e.StatusMsg = "no symbols in " + filepath.Base(e.SourceFile)
		return
	}

	showPicker(e, "Symbols in "+filepath.Base(e.SourceFile), entries)
}

// '<leader>sS' is the same listing widened to the project: the file being
// edited first, then the ones of the same kind beside it, which is as far as
// 'gd' and 'gr' reach too.
func OpenWorkspaceSymbols(e *state.Editor) {
	entries := bufferSymbols(e)
	entries = append(entries, symbolsInFiles(e, maxSymbols-len(entries))...)
	if len(entries) == 0 {
		e.StatusMsg = "no symbols under " + filepath.Base(project.Root(e.SourceFile))
		return
	}

	// which file a symbol is in matters once there is more than one of them,
	// and the fuzzy match then narrows on the name and the file alike
	for at, entry := range entries {
		entries[at].Label = fmt.Sprintf("%s  %s:%d", entry.Label, filepath.Base(entry.Path), entry.Row+1)
	}

	showPicker(e, "Symbols under "+filepath.Base(project.Root(e.SourceFile)), entries)
}

func bufferSymbols(e *state.Editor) []picker.Entry {
	return symbolEntries(e.SourceFile, decl.Symbols(bufLines(e), maxSymbols))
}

func symbolsInFiles(e *state.Editor, limit int) []picker.Entry {
	entries := make([]picker.Entry, 0, 64)
	for path, content := range project.Siblings(e.SourceFile) {
		entries = append(entries, symbolEntries(path, decl.Symbols(decl.Of(content), limit-len(entries)))...)
		if len(entries) >= limit {
			break
		}
	}

	return entries
}

func symbolEntries(path string, symbols []decl.Symbol) []picker.Entry {
	entries := make([]picker.Entry, 0, len(symbols))
	for _, one := range symbols {
		entries = append(entries, picker.Entry{
			Label: symbolLabel(one.Kind, one.Name),
			Path:  path,
			Row:   one.Row,
			Col:   one.Col,
		})
	}

	return entries
}
