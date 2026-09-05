package find

import (
	"fmt"
	"path/filepath"

	"github.com/ArditZubaku/tex/internal/decl"
	"github.com/ArditZubaku/tex/internal/editor/picker"
	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/ArditZubaku/tex/internal/project"
)

// '<leader>ss' is LazyVim's document symbols with no language server behind it:
// the declarations of the file being edited, recognised in the text the way
// 'gd' recognises one, listed in the popup in the order they appear.

// A file with more declarations than this has more than anybody scrolls
// through, and the filter is what finds the one being looked for anyway.
const maxSymbols = 2000

func OpenSymbols(e *state.Editor) {
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
	return symbolEntries(e, e.SourceFile, decl.Symbols(bufLines(e), maxSymbols))
}

func symbolsInFiles(e *state.Editor, limit int) []picker.Entry {
	entries := make([]picker.Entry, 0, 64)
	for path, content := range project.Siblings(e.SourceFile) {
		entries = append(entries, symbolEntries(e, path, decl.Symbols(decl.Of(content), limit-len(entries)))...)
		if len(entries) >= limit {
			break
		}
	}

	return entries
}

func symbolEntries(e *state.Editor, path string, symbols []decl.Symbol) []picker.Entry {
	entries := make([]picker.Entry, 0, len(symbols))
	for _, one := range symbols {
		entries = append(entries, picker.Entry{
			Label: fmt.Sprintf("%-9s %s", one.Kind, one.Name),
			Path:  path,
			Row:   one.Row,
			Col:   one.Col,
		})
	}

	return entries
}
