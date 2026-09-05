package editor

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"unicode/utf8"
)

// '<leader>ss' is LazyVim's document symbols with no language server behind it:
// the declarations of the file being edited, recognised in the text the way
// 'gd' recognises one, listed in the popup in the order they appear.
type symbolForm struct {
	kind string
	form *regexp.Regexp
}

const ident = `[A-Za-z_][A-Za-z0-9_]*`

// A declaration starts its line, past whatever the language lets stand in
// front of one. Anchoring there is what keeps a list of a language's keywords
// inside a string literal from reading as a screenful of declarations.
const declares = `^[ \t]*(?:(?:export|pub(?:\([^)]*\))?|public|private|protected|internal|` +
	`static|async|abstract|final|override|inline|extern|unsafe|declare|open|sealed|suspend)\s+)*`

// The first form a line matches names it, so the specific shapes come before
// the loose ones. A form's captures are joined into the name, which is what
// lets a method carry the receiver it hangs off.
var symbolForms = []symbolForm{
	{"Method", form(declares + `func\s+\(\s*` + ident + `\s+\**(` + ident + `)\s*\)\s*(` + ident + `)`)},
	{"Function", form(declares + `(?:func|fn|def|function|proc|sub)\s+(` + ident + `)\s*[([<]`)},
	{"Struct", form(declares + `type\s+(` + ident + `)(?:\[[^\]]*\])?\s+struct\b`)},
	{"Interface", form(declares + `type\s+(` + ident + `)(?:\[[^\]]*\])?\s+interface\b`)},
	{"Class", form(declares + `(?:class|record)\s+(` + ident + `)`)},
	{"Struct", form(declares + `struct\s+(` + ident + `)`)},
	{"Interface", form(declares + `interface\s+(` + ident + `)`)},
	{"Enum", form(declares + `enum\s+(` + ident + `)`)},
	{"Trait", form(declares + `trait\s+(` + ident + `)`)},
	{"Type", form(declares + `type\s+(` + ident + `)`)},
	{"Constant", form(declares + `const\s+(` + ident + `)`)},
	{"Variable", form(declares + `(?:var|let|val)\s+(?:mut\s+)?(` + ident + `)`)},
}

// Go declares most of its globals inside a 'var (' block, where the names are
// on the lines below the keyword rather than beside it: without this a file
// like globals.go would list no symbols at all.
var (
	groupOpen   = form(`^(var|const|type)\s*\(\s*$`)
	groupMember = form(`^\s+(` + ident + `(?:\s*,\s*` + ident + `)*)\b`)
	groupClose  = form(`^\s*\)`)
	identOnly   = form(ident)
)

var groupKinds = map[string]string{"var": "Variable", "const": "Constant", "type": "Type"}

// A file with more declarations than this has more than anybody scrolls
// through, and the filter is what finds the one being looked for anyway.
const maxSymbols = 2000

func form(source string) *regexp.Regexp {
	return regexp.MustCompile(source)
}

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
		statusMsg = "no symbols under " + filepath.Base(projectRoot())
		return
	}

	// which file a symbol is in matters once there is more than one of them,
	// and the fuzzy match then narrows on the name and the file alike
	for at, entry := range entries {
		entries[at].label = fmt.Sprintf("%s  %s:%d", entry.label, filepath.Base(entry.path), entry.row+1)
	}

	showPicker("Symbols under "+filepath.Base(projectRoot()), entries)
}

// A line source is the buffer or a file read whole, which is the only thing
// that differs between listing the symbols of one and of the other.
type lineSource struct {
	count int
	at    func(row int) []byte
}

func bufferSymbols() []pickerEntry {
	return symbolsIn(sourceFile, lineSource{count: buf.LineCount(), at: lineBytes}, maxSymbols)
}

func symbolsInFiles(limit int) []pickerEntry {
	root := projectRoot()
	files, err := listFiles(root)
	if err != nil {
		return nil
	}

	ext := filepath.Ext(sourceFile)
	entries := make([]pickerEntry, 0, 64)
	for _, rel := range files {
		path := filepath.Join(root, rel)
		if filepath.Ext(path) != ext || sameFile(path, sourceFile) {
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

		lines := bytes.Split(content, []byte("\n"))
		entries = append(entries, symbolsIn(path, lineSource{
			count: len(lines),
			at:    func(row int) []byte { return lines[row] },
		}, limit-len(entries))...)
		if len(entries) >= limit {
			break
		}
	}

	return entries
}

func symbolsIn(path string, src lineSource, limit int) []pickerEntry {
	entries := make([]pickerEntry, 0, 64)
	group := ""
	for row := range src.count {
		line := src.at(row)

		switch {
		case group != "" && groupClose.Match(line):
			group = ""
		case group != "":
			entries = append(entries, groupSymbols(path, group, row, line)...)
		case groupOpen.Match(line):
			group = groupKinds[string(groupOpen.FindSubmatch(line)[1])]
		default:
			if entry, ok := lineSymbol(path, row, line); ok {
				entries = append(entries, entry)
			}
		}

		if len(entries) >= limit {
			break
		}
	}

	return entries
}

func lineSymbol(path string, row int, line []byte) (pickerEntry, bool) {
	for _, candidate := range symbolForms {
		at := candidate.form.FindSubmatchIndex(line)
		if at == nil {
			continue
		}

		name := ""
		for group := 1; 2*group < len(at); group++ {
			if name != "" {
				name += "."
			}
			name += string(line[at[2*group]:at[2*group+1]])
		}

		return symbol(candidate.kind, name, path, row, utf8.RuneCount(line[:at[2]])), true
	}

	return pickerEntry{}, false
}

// One line of a group can declare several names ("ROWS, COLS int"), each of
// which is its own symbol at its own column.
func groupSymbols(path, kind string, row int, line []byte) []pickerEntry {
	at := groupMember.FindSubmatchIndex(line)
	if at == nil {
		return nil
	}

	entries := make([]pickerEntry, 0, 2)
	for _, name := range identOnly.FindAllIndex(line[at[2]:at[3]], -1) {
		from := at[2] + name[0]
		entries = append(entries, symbol(kind, string(line[from:at[2]+name[1]]), path, row, utf8.RuneCount(line[:from])))
	}

	return entries
}

func symbol(kind, name, path string, row, col int) pickerEntry {
	return pickerEntry{
		label: fmt.Sprintf("%-9s %s", kind, name),
		path:  path,
		row:   row,
		col:   col,
	}
}
