// Package decl is what a declaration looks like in text: the shapes 'gd', 'gr'
// and the symbol list recognise one by, with no language server behind them.
package decl

import (
	"bytes"
	"fmt"
	"regexp"
	"unicode/utf8"

	"github.com/ArditZubaku/tex/internal/chars"
)

// Lines is one file's lines, whether it is being edited or was read whole.
type Lines struct {
	Count int
	At    func(row int) []byte
}

// Of holds a file read whole, which is the other shape a caller has its lines
// in.
func Of(content []byte) Lines {
	lines := bytes.Split(content, []byte("\n"))

	return Lines{Count: len(lines), At: func(row int) []byte { return lines[row] }}
}

// forms are the shapes a declaration takes across the languages the editor
// highlights, strongest first; the last of them is a plain mention, which is
// what VIM's own 'gd' settles for when nothing stronger is there.
var forms = []string{
	`(?:func|fn|def|function|proc|sub)\s+(?:\([^)]*\)\s*)?(%s)\b`,
	`(?:type|class|struct|interface|enum|trait|record)\s+(%s)\b`,
	`(?:func|fn|def|function|proc|sub)\b[^(]*\((?:[^)]*[^\w.])?(%s)\b[^)]*\)`,
	`(?:var|let|const|val|static)\s+(%s)\b`,
	`(?:^|[^\w.])(%s)\s*(?::=|=[^=])`,
	`(?:^|[^\w.])(%s)\b`,
}

// ParamForm is where the parameter list sits in forms: a name declared there is
// local to the function whatever column that function's own line starts in.
const ParamForm = 2

// Site is where a form matched, and which one: a lower rank is a stronger
// claim to being the declaration.
type Site struct {
	Row  int
	Col  int
	Rank int
}

func Forms(word string) ([]*regexp.Regexp, error) {
	compiled := make([]*regexp.Regexp, 0, len(forms))
	for _, source := range forms {
		one, err := regexp.Compile(fmt.Sprintf(source, regexp.QuoteMeta(word)))
		if err != nil {
			return nil, err
		}
		compiled = append(compiled, one)
	}

	return compiled, nil
}

func Mentions(word string) (*regexp.Regexp, error) {
	return regexp.Compile(`\b` + regexp.QuoteMeta(word) + `\b`)
}

// Under is the identifier the column sits on, which is what 'gd' and 'gr' are
// asked about.
func Under(line []rune, col int) (string, bool) {
	if col >= len(line) || chars.ClassOf(line[col]) != chars.Word {
		return "", false
	}

	from := col
	for from > 0 && chars.ClassOf(line[from-1]) == chars.Word {
		from--
	}
	to := col
	for to < len(line) && chars.ClassOf(line[to]) == chars.Word {
		to++
	}

	return string(line[from:to]), true
}

// Above is what VIM's 'gd' is for: the declaration nearest above the cursor and
// inside the same top-level construct, which is the one the identifier under
// the cursor is bound to — the parameter it was passed as, or the local it was
// assigned from, rather than a field of the same name three hundred lines away.
func Above(src Lines, compiled []*regexp.Regexp, row, col int) (Site, bool) {
	for at := row; at >= BlockStart(src, row); at-- {
		line := src.At(at)
		for rank := range len(compiled) - 1 { // a mention on its own declares nothing
			found := compiled[rank].FindSubmatchIndex(line)
			if found == nil {
				continue
			}

			start := utf8.RuneCount(line[:found[2]])
			if at == row && start == col {
				break // the cursor is on it: whatever this is, it is not a jump
			}

			return Site{Row: at, Col: start, Rank: rank}, true
		}
	}

	return Site{}, false
}

// BlockStart is where the top-level construct the row sits in begins: the
// nearest line above it that starts in the first column, which is where every
// language the editor highlights puts one.
func BlockStart(src Lines, row int) int {
	for at := row - 1; at > 0; at-- {
		line := src.At(at)
		if len(line) > 0 && line[0] != ' ' && line[0] != '\t' {
			return at
		}
	}

	return 0
}

// BlockEnd is where the construct the row sits in gives out: the next line
// below it that starts in the first column, which is where the one after it —
// or the brace closing this one — begins.
func BlockEnd(src Lines, row int) int {
	for at := row + 1; at < src.Count; at++ {
		line := src.At(at)
		if len(line) > 0 && line[0] != ' ' && line[0] != '\t' {
			return at
		}
	}

	return src.Count
}

// First takes the strongest form any line matches, and the first line of that
// form: a declaration is what is wanted, and the file's own order breaks the
// tie between two of them. Ranks at or beyond weaker are not looked for, which
// is how a caller says it already holds something better.
func First(src Lines, compiled []*regexp.Regexp, weaker int) (Site, bool) {
	best := Site{Rank: weaker}
	found := false
	for row := range src.Count {
		line := src.At(row)
		for rank := range best.Rank {
			if at := compiled[rank].FindSubmatchIndex(line); at != nil {
				best = Site{Row: row, Col: utf8.RuneCount(line[:at[2]]), Rank: rank}
				found = true
				break
			}
		}
		if best.Rank == 0 {
			break
		}
	}

	return best, found
}
