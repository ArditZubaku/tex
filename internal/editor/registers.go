package editor

import (
	"slices"

	"github.com/ArditZubaku/tex/internal/editor/register"
	"github.com/ArditZubaku/tex/internal/motion"
)

func yankLine() {
	yankLines(ed.Row, ed.Count())
}

func yankLines(row, n int) {
	n = min(n, ed.Buf.LineCount()-row)

	lines := make([][]rune, 0, n)
	for i := range n {
		lines = append(lines, slices.Clone(ed.Buf.Line(row+i)))
	}
	ed.Clip = register.Linewise(lines)
}

func yankChars(row, from, to int) {
	ed.Clip = register.Charwise([][]rune{slices.Clone(ed.Buf.Line(row)[from:to])})
}

// yankWord is 'yw', yankToWordEnd is 'ye' and yankToPrevWord is 'yb'. Like the
// delete operators they stop at the end of the line, and like VIM they leave
// the cursor at the start of what was yanked.
func yankWord() {
	row, col := motion.NextWordFrom(ed.Buf, ed.Row, ed.Col)
	yankForwardTo(row, col)
}

func yankToWordEnd() {
	row, col := motion.EndOfWordFrom(ed.Buf, ed.Row, ed.Col)
	yankForwardTo(row, col+1)
}

func yankForwardTo(row, col int) {
	if row != ed.Row {
		col = ed.Buf.RuneLen(ed.Row)
	}
	if col <= ed.Col {
		return
	}
	yankChars(ed.Row, ed.Col, col)
}

func yankToPrevWord() {
	row, col := motion.PrevWordFrom(ed.Buf, ed.Row, ed.Col)
	if row != ed.Row {
		col = 0
	}
	if col >= ed.Col {
		return
	}

	yankChars(ed.Row, col, ed.Col)
	ed.Col = col
}

func pasteAfter()  { paste(true) }
func pasteBefore() { paste(false) }

func paste(after bool) {
	if ed.Clip.Empty() {
		return
	}

	if ed.Clip.IsLinewise() {
		pasteLines(after)
		return
	}
	pasteChars(after)
}

func pasteLines(after bool) {
	row := ed.Row
	if after {
		row++
	}

	at := row
	for range ed.Count() {
		for _, line := range ed.Clip.Content() {
			ed.TouchInsertLine(at)
			ed.Buf.InsertLine(at)
			ed.Buf.SetLine(at, slices.Clone(line))
			at++
		}
	}

	ed.Row, ed.Col = row, 0
	ed.Modified = true
}

func pasteChars(after bool) {
	text := ed.Clip.Repeated(ed.Count())
	line := slices.Clone(ed.Buf.Line(ed.Row))

	col := ed.Col
	if after && len(line) > 0 {
		col++
	}
	col = min(col, len(line))

	ed.TouchLine(ed.Row)

	if len(text) == 1 {
		ed.Buf.SetLine(ed.Row, slices.Insert(line, col, text[0]...))
		ed.Col = max(col+len(text[0])-1, 0)
		ed.Modified = true
		return
	}

	// A register holding a run that spanned lines splits the line it is put
	// into: its first line joins what was before the cursor, its last one what
	// came after.
	tail := slices.Clone(line[col:])
	ed.Buf.SetLine(ed.Row, append(line[:col], text[0]...))

	row := ed.Row
	for _, l := range text[1:] {
		row++
		ed.TouchInsertLine(row)
		ed.Buf.InsertLine(row)
		ed.Buf.SetLine(row, slices.Clone(l))
	}
	ed.Buf.SetLine(row, append(ed.Buf.Line(row), tail...))

	ed.Col = col
	ed.Modified = true
}
