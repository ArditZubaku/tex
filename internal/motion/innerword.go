package motion

import (
	"github.com/ArditZubaku/tex/internal/buffer"
	"github.com/ArditZubaku/tex/internal/chars"
)

// InnerWordFrom is where VIM's 'iw' text object reaches: the run of
// same-class runes — word, punct or space — that col already sits inside, as
// a closed [start, end] range of columns on row. Unlike NextWordFrom and
// friends it never crosses a line break, since a word text object doesn't
// either; an empty line has nothing to select, reported as start > end.
func InnerWordFrom(b *buffer.Buffer, row, col int) (start, end int) {
	line := b.Line(row)
	if len(line) == 0 {
		return 0, -1
	}
	col = min(max(col, 0), len(line)-1)

	class := chars.ClassOf(line[col])
	start, end = col, col
	for start > 0 && chars.ClassOf(line[start-1]) == class {
		start--
	}
	for end < len(line)-1 && chars.ClassOf(line[end+1]) == class {
		end++
	}

	return start, end
}
