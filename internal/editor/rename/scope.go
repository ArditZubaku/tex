package rename

import (
	"cmp"
	"path/filepath"
	"regexp"
	"unicode/utf8"

	"github.com/ArditZubaku/tex/internal/chars"
	"github.com/ArditZubaku/tex/internal/decl"
	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/ArditZubaku/tex/internal/project"
)

// A scope is how far a name reaches, which is what keeps a rename off the
// identifiers that merely spell the same. A local reaches the block it is
// declared in and no further; a package-level name reaches its own package bare
// and the rest of the project through its package name, so renaming
// 'command.Run' leaves 'editor.Run' alone; and a name reached through a value
// ('b.file') reaches its own package either way, since nothing in the text says
// which values are of the type that holds it.
type scope struct {
	own       string // the package the file being edited belongs to
	home      string // the package the name itself belongs to
	qualifier string // what the rest of the project mentions it through, if anything
	member    bool   // reached through a value rather than a package
	local     bool
	from, to  int // the rows a local is confined to
}

// header is what a file says it belongs to, wherever the doc comment above it
// ends. A language without one — most of them — is left to the directory, which
// is the same thing a package is.
var header = regexp.MustCompile(`(?m)^package\s+([A-Za-z_]\w*)`)

// The search for it is bounded either way it is made: a file that has a header
// puts it at the top, and one that has not should not be read through to find
// that out.
const (
	headerRows  = 64
	headerBytes = 4 << 10
)

func scopeOf(e *state.Editor, name string, col int) scope {
	src := bufLines(e)
	dir := filepath.Dir(e.SourceFile)
	own := cmp.Or(headerIn(src), dir)

	if qualifier, through := qualifierAt(e.Buf.Line(e.Row), col); through {
		if qualifier != "" && project.HasPackage(e.SourceFile, qualifier) {
			return scope{own: own, home: qualifier, qualifier: qualifier}
		}

		return scope{own: own, home: own, member: true}
	}

	if at, ok := localSite(src, name, e.Row, col); ok {
		from, to := blockAround(src, at.Row)

		return scope{own: own, home: own, local: true, from: from, to: to}
	}

	return scope{own: own, home: own, qualifier: cmp.Or(headerIn(src), filepath.Base(dir))}
}

// matcherFor is what a file of that package has renamed in it, and whether it is
// touched at all: a member's package is as far as a rename can honestly reach.
func (s scope) matcherFor(pkg string, from, to []rune) (matcher, bool) {
	if pkg == s.home {
		return matcher{name: from, to: to, bare: !s.member, home: true}, true
	}
	if s.qualifier == "" {
		return matcher{}, false
	}

	return matcher{name: from, to: to, pkg: []rune(s.qualifier)}, true
}

// homeOf is the package a file beside the one being edited belongs to, named
// the same way the editor's own is.
func homeOf(path string, content []byte) string {
	if at := header.FindSubmatch(content[:min(len(content), headerBytes)]); at != nil {
		return string(at[1])
	}

	return filepath.Dir(path)
}

func headerIn(src decl.Lines) string {
	for row := range min(src.Count, headerRows) {
		if at := header.FindSubmatch(src.At(row)); at != nil {
			return string(at[1])
		}
	}

	return ""
}

// qualifierAt is what the mention is reached through, and whether it is reached
// through anything at all. Only the first name of a chain can be a package, so
// the rest of one answers with the empty name that no package has.
func qualifierAt(line []rune, col int) (string, bool) {
	if col == 0 || line[col-1] != '.' {
		return "", false
	}

	from := col - 1
	for from > 0 && chars.IsWord(line[from-1]) {
		from--
	}
	if from == col-1 || (from > 0 && line[from-1] == '.') {
		return "", true
	}

	return string(line[from : col-1]), true
}

// localSite is where the name was declared when it was declared inside a block:
// a parameter, or anything indented under the construct that holds it.
func localSite(src decl.Lines, name string, row, col int) (decl.Site, bool) {
	forms, err := decl.Forms(name)
	if err != nil {
		return decl.Site{}, false
	}

	at, ok := siteOn(src, forms, row, col)
	if !ok {
		if at, ok = decl.Above(src, forms, row, col); !ok {
			return decl.Site{}, false
		}
	}

	line := src.At(at.Row)
	indented := len(line) > 0 && (line[0] == ' ' || line[0] == '\t')

	return at, at.Rank == decl.ParamForm || indented
}

// siteOn is the declaration the cursor is itself sitting on, which 'gd' steps
// over: it looks for what a mention was bound by, and a name is not declared by
// itself.
func siteOn(src decl.Lines, forms []*regexp.Regexp, row, col int) (decl.Site, bool) {
	line := src.At(row)
	for rank := range len(forms) - 1 { // a mention on its own declares nothing
		if at := forms[rank].FindSubmatchIndex(line); len(at) > 2 && utf8.RuneCount(line[:at[2]]) == col {
			return decl.Site{Row: row, Col: col, Rank: rank}, true
		}
	}

	return decl.Site{}, false
}

func blockAround(src decl.Lines, row int) (from, to int) {
	from = row
	if line := src.At(row); len(line) > 0 && (line[0] == ' ' || line[0] == '\t') {
		from = decl.BlockStart(src, row)
	}

	return from, decl.BlockEnd(src, row)
}

func wordStart(line []rune, col int) int {
	for col > 0 && chars.IsWord(line[col-1]) {
		col--
	}

	return col
}

func bufLines(e *state.Editor) decl.Lines {
	return decl.Lines{Count: e.Buf.LineCount(), At: e.LineBytes}
}
