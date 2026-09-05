package editor

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ArditZubaku/tex/internal/decl"
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
		statusMsg = "no symbols in " + filepath.Base(sourceFile)
		return
	}

	showPicker("Symbols in "+filepath.Base(sourceFile), entries)
}

// '<leader>sS' is the same listing widened to the project: the file being
// edited first, then the ones of the same kind beside it, which is as far as
// 'gd' and 'gr' reach too.
func openWorkspaceSymbols() {
	entries := bufferSymbols()
	entries = append(entries, symbolsInFiles(maxSymbols-len(entries))...)
	if len(entries) == 0 {
		statusMsg = "no symbols under " + filepath.Base(project.Root(sourceFile))
		return
	}

	// which file a symbol is in matters once there is more than one of them,
	// and the fuzzy match then narrows on the name and the file alike
	for at, entry := range entries {
		entries[at].label = fmt.Sprintf("%s  %s:%d", entry.label, filepath.Base(entry.path), entry.row+1)
	}

	showPicker("Symbols under "+filepath.Base(project.Root(sourceFile)), entries)
}

func bufferSymbols() []pickerEntry {
	return symbolEntries(sourceFile, decl.Symbols(bufLines(), maxSymbols))
}

func symbolsInFiles(limit int) []pickerEntry {
	root := project.Root(sourceFile)
	files, err := project.List(root)
	if err != nil {
		return nil
	}

	ext := filepath.Ext(sourceFile)
	entries := make([]pickerEntry, 0, 64)
	for _, rel := range files {
		path := filepath.Join(root, rel)
		if filepath.Ext(path) != ext || project.Same(path, sourceFile) {
			continue
		}

		info, err := os.Stat(path)
		if err != nil || info.Size() > maxDefinitionFileSize {
			continue
		}
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		entries = append(entries, symbolEntries(path, decl.Symbols(decl.Of(content), limit-len(entries)))...)
		if len(entries) >= limit {
			break
		}
	}

	return entries
}

func symbolEntries(path string, symbols []decl.Symbol) []pickerEntry {
	entries := make([]pickerEntry, 0, len(symbols))
	for _, one := range symbols {
		entries = append(entries, pickerEntry{
			label: fmt.Sprintf("%-9s %s", one.Kind, one.Name),
			path:  path,
			row:   one.Row,
			col:   one.Col,
		})
	}

	return entries
}
