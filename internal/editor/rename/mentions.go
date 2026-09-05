package rename

import (
	"slices"

	"github.com/ArditZubaku/tex/internal/chars"
)

// A matcher is which mentions of the name one file's rename touches: the bare
// ones, the ones the name's own package qualifies, or either of them when the
// name is reached through a value rather than declared.
type matcher struct {
	name []rune
	to   []rune
	pkg  []rune // when set, only a mention this package name qualifies
	bare bool   // when set, never a mention reached through a '.'
	home bool   // the file is one of the name's own package
}

// needle is the shortest text a file has to hold for a mention to be in it,
// which is what a rename decides on before opening the file.
func (m matcher) needle() string {
	if len(m.pkg) > 0 {
		return string(m.pkg) + "." + string(m.name)
	}

	return string(m.name)
}

// at is where the name next stands on its own in line, at or after from: the
// whole-word match 'gr' lists, narrowed to the mentions this rename means. The
// code mask, when there is one, is which columns are not comment or string.
func (m matcher) at(line []rune, code []bool, from int) int {
	for at := from; at+len(m.name) <= len(line); at++ {
		if !slices.Equal(line[at:at+len(m.name)], m.name) || (at > 0 && chars.IsWord(line[at-1])) {
			continue
		}
		if code != nil && at < len(code) && !code[at] {
			continue
		}
		if end := at + len(m.name); end < len(line) && chars.IsWord(line[end]) {
			continue
		}
		if !m.reached(line, at) {
			continue
		}

		return at
	}

	return -1
}

// reached says whether the mention is come at the way this rename means it: a
// package-level name bare inside its own package and through its package name
// outside it, and a name reached through a value either way, since nothing in
// the text says which values are of the type that holds it.
func (m matcher) reached(line []rune, at int) bool {
	switch {
	case len(m.pkg) > 0:
		start := at - len(m.pkg) - 1

		return start >= 0 && line[at-1] == '.' && slices.Equal(line[start:at-1], m.pkg) &&
			(start == 0 || (!chars.IsWord(line[start-1]) && line[start-1] != '.'))
	case m.bare:
		return at == 0 || line[at-1] != '.'
	default:
		return true
	}
}

// replaced rewrites every mention in line and says how many there were. A line
// holding none is handed back as it stands, array and all, since most lines of
// a file mention the name nowhere.
func (m matcher) replaced(line []rune, code []bool) ([]rune, int) {
	at := m.at(line, code, 0)
	if at < 0 {
		return line, 0
	}

	out := make([]rune, 0, len(line)+len(m.to)-len(m.name))
	copied, count := 0, 0
	for ; at >= 0; at = m.at(line, code, copied) {
		out = append(out, line[copied:at]...)
		out = append(out, m.to...)
		copied, count = at+len(m.name), count+1
	}

	return append(out, line[copied:]...), count
}

// mentionAt is the mention the cursor sits on and how many stand before it on
// the line, which is what the cursor is put back onto: a name that changed
// length carries everything after it along the line.
func (m matcher) mentionAt(line []rune, col int) (at, before int) {
	for start := m.at(line, nil, 0); start >= 0; start = m.at(line, nil, start+len(m.name)) {
		if col < start+len(m.name) {
			return start, before
		}
		before++
	}

	return col, 0
}
