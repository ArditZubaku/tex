package decl

import (
	"regexp"
	"unicode/utf8"
)

// A symbol is a declaration recognised without being looked for: the same
// shapes 'gd' matches against one identifier, run over every line instead.
type Symbol struct {
	Kind string
	Name string
	Row  int
	Col  int
}

const ident = `[A-Za-z_][A-Za-z0-9_]*`

// A declaration starts its line, past whatever the language lets stand in
// front of one. Anchoring there is what keeps a list of a language's keywords
// inside a string literal from reading as a screenful of declarations.
const declares = `^[ \t]*(?:(?:export|pub(?:\([^)]*\))?|public|private|protected|internal|` +
	`static|async|abstract|final|override|inline|extern|unsafe|declare|open|sealed|suspend)\s+)*`

type symbolForm struct {
	kind string
	form *regexp.Regexp
}

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

func form(source string) *regexp.Regexp {
	return regexp.MustCompile(source)
}

func Symbols(src Lines, limit int) []Symbol {
	found := make([]Symbol, 0, 64)
	group := ""
	for row := range src.Count {
		line := src.At(row)

		switch {
		case group != "" && groupClose.Match(line):
			group = ""
		case group != "":
			found = append(found, groupSymbols(group, row, line)...)
		case groupOpen.Match(line):
			group = groupKinds[string(groupOpen.FindSubmatch(line)[1])]
		default:
			if one, ok := lineSymbol(row, line); ok {
				found = append(found, one)
			}
		}

		if len(found) >= limit {
			break
		}
	}

	return found
}

func lineSymbol(row int, line []byte) (Symbol, bool) {
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

		return Symbol{Kind: candidate.kind, Name: name, Row: row, Col: utf8.RuneCount(line[:at[2]])}, true
	}

	return Symbol{}, false
}

// One line of a group can declare several names ("ROWS, COLS int"), each of
// which is its own symbol at its own column.
func groupSymbols(kind string, row int, line []byte) []Symbol {
	at := groupMember.FindSubmatchIndex(line)
	if at == nil {
		return nil
	}

	found := make([]Symbol, 0, 2)
	for _, name := range identOnly.FindAllIndex(line[at[2]:at[3]], -1) {
		from := at[2] + name[0]
		found = append(found, Symbol{
			Kind: kind,
			Name: string(line[from : at[2]+name[1]]),
			Row:  row,
			Col:  utf8.RuneCount(line[:from]),
		})
	}

	return found
}
