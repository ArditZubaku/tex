package rename

import (
	"regexp"

	"github.com/ArditZubaku/tex/internal/buffer"
	"github.com/ArditZubaku/tex/internal/chars"
	"github.com/ArditZubaku/tex/internal/decl"
	"github.com/ArditZubaku/tex/internal/syntax"
)

// A pass is one file's rename: the mentions the matcher takes, in the rows it is
// allowed. What the lexer reads as a comment or a string is left alone — prose
// naming a function is not a call of it — bar two: the doc comment over the
// declaration itself, which names what it documents and would be left saying
// the wrong thing, and the mention the cursor is on, which is what was pointed
// at.
type pass struct {
	b     *buffer.Buffer
	lang  *syntax.Syntax
	touch func(row int)
	m     matcher
	rows  [2]int
	at    [2]int // where the cursor's own mention starts; -1 in a file it is not in
}

func (p pass) run() (changes, lines int) {
	forms := p.declForms()
	inBlock, comment := false, -1
	var code []bool

	for row := range min(p.rows[1], p.b.LineCount()) {
		line := p.b.Line(row)
		code, inBlock = p.lang.Code(line, inBlock, code[:0])
		prose := commentOnly(line, code)

		if row < p.rows[0] {
			comment = runStart(comment, row, prose)
			continue
		}

		hits := p.replaceLine(row, line, code)
		if hits > 0 {
			changes, lines = changes+hits, lines+1
		}

		// the comment run this line closes documents it, if this is where the
		// name is declared
		if hits > 0 && comment >= 0 && !prose && declares(forms, line) {
			docChanges, docLines := p.replaceDoc(max(comment, p.rows[0]), row)
			changes, lines = changes+docChanges, lines+docLines
		}
		comment = runStart(comment, row, prose)
	}

	return changes, lines
}

func runStart(comment, row int, prose bool) int {
	switch {
	case !prose:
		return -1
	case comment < 0:
		return row
	default:
		return comment
	}
}

func (p pass) replaceLine(row int, line []rune, code []bool) int {
	if row == p.at[0] && code != nil && p.at[1] < len(code) {
		code[p.at[1]] = true
	}

	updated, hits := p.m.replaced(line, code)
	if hits == 0 {
		return 0
	}

	p.touch(row)
	p.b.SetLine(row, updated)

	return hits
}

// replaceDoc takes the comment lines above a declaration, where the whole line
// is prose and every mention in it is one of the declaration's own.
func (p pass) replaceDoc(from, to int) (changes, lines int) {
	for row := from; row < to; row++ {
		if hits := p.replaceLine(row, p.b.Line(row), nil); hits > 0 {
			changes, lines = changes+hits, lines+1
		}
	}

	return changes, lines
}

// declForms are the shapes the name is declared in, which only the files of its
// own package are looked at for: elsewhere a line declaring the same name
// declares something else of the same spelling.
func (p pass) declForms() []*regexp.Regexp {
	if !p.m.home {
		return nil
	}

	forms, err := decl.Forms(string(p.m.name))
	if err != nil {
		return nil
	}

	return forms
}

func declares(forms []*regexp.Regexp, line []rune) bool {
	if forms == nil {
		return false
	}

	raw := []byte(string(line))
	for rank := range len(forms) - 1 { // a mention on its own declares nothing
		if forms[rank].Match(raw) {
			return true
		}
	}

	return false
}

// commentOnly says whether the line is prose and nothing else, which is what a
// doc comment above a declaration is made of.
func commentOnly(line []rune, code []bool) bool {
	if code == nil {
		return false
	}

	written := false
	for i, ch := range line {
		if chars.IsSpace(ch) {
			continue
		}
		if code[i] {
			return false
		}
		written = true
	}

	return written
}
