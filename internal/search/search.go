// Package search is literal text matched over a buffer, in the two shapes the
// buffer keeps its lines in.
package search

import (
	"bytes"
	"slices"
	"unicode/utf8"

	"github.com/ArditZubaku/tex/internal/buffer"
)

// Pattern is one search string held in both shapes the buffer keeps lines in:
// raw bytes for the lines still on disk, runes for the ones the overlay holds.
type Pattern struct {
	raw   []byte
	runes []rune
}

// Runes is the pattern as it was typed, which the status line reports and the
// renderer measures a highlight by.
func (p Pattern) Runes() []rune { return p.runes }

func New(runes []rune) Pattern {
	return Pattern{raw: []byte(string(runes)), runes: runes}
}

func (p Pattern) Empty() bool {
	return len(p.runes) == 0
}

// MatchesIn appends the rune column of every occurrence of p in line row, in
// ascending order and overlapping matches included. An unedited line is matched
// over its raw bytes, so a search decodes nothing it does not have to:
// bytes.Index walks the line whole, and only the text in front of a hit is
// counted, to turn its byte offset into the column the cursor moves to.
func (p Pattern) MatchesIn(b *buffer.Buffer, row int, out []int) []int {
	if p.Empty() || row < 0 || row >= b.LineCount() {
		return out
	}

	if line, ok := b.EditedLine(row); ok {
		for i := 0; i+len(p.runes) <= len(line); i++ {
			if slices.Equal(line[i:i+len(p.runes)], p.runes) {
				out = append(out, i)
			}
		}

		return out
	}

	raw := b.Raw(row)
	col := 0
	for at := 0; at+len(p.raw) <= len(raw); {
		i := bytes.Index(raw[at:], p.raw)
		if i < 0 {
			break
		}

		col += utf8.RuneCount(raw[at : at+i])
		out = append(out, col)

		_, size := utf8.DecodeRune(raw[at+i:])
		at, col = at+i+size, col+1
	}

	return out
}

// Find walks out from the cursor a line at a time and wraps around the end
// of the buffer the way VIM does, which is why the line the search started on
// is visited twice: once for what lies past the cursor, and once, after the
// wrap, for what lies before it.
func Find(b *buffer.Buffer, p Pattern, row, col int, back bool) (int, int, bool) {
	lines := b.LineCount()
	var cols []int

	for step := 0; step <= lines; step++ {
		at := row + step
		if back {
			at = row - step
		}
		at = ((at % lines) + lines) % lines

		cols = p.MatchesIn(b, at, cols[:0])
		if back {
			slices.Reverse(cols)
		}

		for _, c := range cols {
			if step == 0 && !beyond(c, col, back) {
				continue
			}
			if step == lines && beyond(c, col, back) {
				continue
			}

			return at, c, true
		}
	}

	return 0, 0, false
}

func beyond(c, col int, back bool) bool {
	if back {
		return c < col
	}

	return c > col
}
