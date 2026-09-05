package editor

import (
	"fmt"
	"path/filepath"

	"github.com/ArditZubaku/tex/internal/decl"
	"github.com/ArditZubaku/tex/internal/editor/picker"
	"github.com/ArditZubaku/tex/internal/project"
)

// '<leader>ss' is LazyVim's document symbols with no language server behind it:
// the declarations of the file being edited, recognised in the text the way
// 'gd' recognises one, listed in the popup in the order they appear.

// A file with more declarations than this has more than anybody scrolls
// through, and the filter is what finds the one being looked for anyway.
const maxSymbols = 2000

func openSymbols() {
	entries := bufferSymbols()
	if len(entries) == 0 {
		ed.StatusMsg = "no symbols in " + filepath.Base(ed.SourceFile)
		return
	}

	showPicker("Symbols in "+filepath.Base(ed.SourceFile), entries)
}

// '<leader>sS' is the same listing widened to the project: the file being
// edited first, then the ones of the same kind beside it, which is as far as
// 'gd' and 'gr' reach too.
func openWorkspaceSymbols() {
	entries := bufferSymbols()
	entries = append(entries, symbolsInFiles(maxSymbols-len(entries))...)
	if len(entries) == 0 {
		ed.StatusMsg = "no symbols under " + filepath.Base(project.Root(ed.SourceFile))
		return
	}

	// which file a symbol is in matters once there is more than one of them,
	// and the fuzzy match then narrows on the name and the file alike
	for at, entry := range entries {
		entries[at].Label = fmt.Sprintf("%s  %s:%d", entry.Label, filepath.Base(entry.Path), entry.Row+1)
	}

	showPicker("Symbols under "+filepath.Base(project.Root(ed.SourceFile)), entries)
}

func bufferSymbols() []picker.Entry {
	return symbolEntries(ed.SourceFile, decl.Symbols(bufLines(), maxSymbols))
}

func symbolsInFiles(limit int) []picker.Entry {
	entries := make([]picker.Entry, 0, 64)
	for path, content := range project.Siblings(ed.SourceFile) {
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
			Label: fmt.Sprintf("%-9s %s", one.Kind, one.Name),
			Path:  path,
			Row:   one.Row,
			Col:   one.Col,
		})
	}

	return entries
}
