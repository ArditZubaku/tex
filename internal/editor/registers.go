package editor

import (
	"slices"

	"github.com/ArditZubaku/tex/internal/motion"
)

// clipboard is VIM's unnamed register: whatever was last yanked or deleted,
// held either as whole lines (yy, dd, V) or as a run of runes (yw, x, v), which
// spans more than one line only when a Visual selection did.
type register struct {
	lines    [][]rune
	linewise bool
}

var clipboard register

func (r register) empty() bool {
	return len(r.lines) == 0
}

func yankLine() {
	yankLines(currentRow, count())
}

func yankLines(row, n int) {
	n = min(n, buf.LineCount()-row)

	lines := make([][]rune, 0, n)
	for i := range n {
		lines = append(lines, slices.Clone(buf.Line(row+i)))
	}
	clipboard = register{lines: lines, linewise: true}
}

func yankChars(row, from, to int) {
	clipboard = register{lines: [][]rune{slices.Clone(buf.Line(row)[from:to])}}
}

// yankWord is 'yw', yankToWordEnd is 'ye' and yankToPrevWord is 'yb'. Like the
// delete operators they stop at the end of the line, and like VIM they leave
// the cursor at the start of what was yanked.
func yankWord() {
	row, col := motion.NextWordFrom(buf, currentRow, currentCol)
	yankForwardTo(row, col)
}

func yankToWordEnd() {
	row, col := motion.EndOfWordFrom(buf, currentRow, currentCol)
	yankForwardTo(row, col+1)
}

func yankForwardTo(row, col int) {
	if row != currentRow {
		col = buf.RuneLen(currentRow)
	}
	if col <= currentCol {
		return
	}
	yankChars(currentRow, currentCol, col)
}

func yankToPrevWord() {
	row, col := motion.PrevWordFrom(buf, currentRow, currentCol)
	if row != currentRow {
		col = 0
	}
	if col >= currentCol {
		return
	}

	yankChars(currentRow, col, currentCol)
	currentCol = col
}

func pasteAfter()  { paste(true) }
func pasteBefore() { paste(false) }

func paste(after bool) {
	if clipboard.empty() {
		return
	}

	if clipboard.linewise {
		pasteLines(after)
		return
	}
	pasteChars(after)
}

func pasteLines(after bool) {
	row := currentRow
	if after {
		row++
	}

	at := row
	for range count() {
		for _, line := range clipboard.lines {
			touchInsertLine(at)
			buf.InsertLine(at)
			buf.SetLine(at, slices.Clone(line))
			at++
		}
	}

	currentRow, currentCol = row, 0
	modified = true
}

func pasteChars(after bool) {
	text := repeatChars(clipboard.lines, count())
	line := slices.Clone(buf.Line(currentRow))

	col := currentCol
	if after && len(line) > 0 {
		col++
	}
	col = min(col, len(line))

	touchLine(currentRow)

	if len(text) == 1 {
		buf.SetLine(currentRow, slices.Insert(line, col, text[0]...))
		currentCol = max(col+len(text[0])-1, 0)
		modified = true
		return
	}

	// A register holding a run that spanned lines splits the line it is put
	// into: its first line joins what was before the cursor, its last one what
	// came after.
	tail := slices.Clone(line[col:])
	buf.SetLine(currentRow, append(line[:col], text[0]...))

	row := currentRow
	for _, l := range text[1:] {
		row++
		touchInsertLine(row)
		buf.InsertLine(row)
		buf.SetLine(row, slices.Clone(l))
	}
	buf.SetLine(row, append(buf.Line(row), tail...))

	currentCol = col
	modified = true
}

// repeatChars is a counted put of a charwise register: the copies run into each
// other, so putting a two-line register twice leaves three lines, not four.
func repeatChars(lines [][]rune, n int) [][]rune {
	if n <= 1 {
		return lines
	}

	out := make([][]rune, 0, (len(lines)-1)*n+1)
	for range n {
		if len(out) == 0 {
			out = append(out, slices.Clone(lines[0]))
		} else {
			out[len(out)-1] = append(out[len(out)-1], lines[0]...)
		}
		for _, l := range lines[1:] {
			out = append(out, slices.Clone(l))
		}
	}

	return out
}
