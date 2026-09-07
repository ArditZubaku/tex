package edit

import (
	"slices"

	"github.com/ArditZubaku/tex/internal/editor/register"
	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/ArditZubaku/tex/internal/motion"
)

func YankLine(e *state.Editor) {
	YankLines(e, e.Row, e.Count())
}

func YankLines(e *state.Editor, row, n int) {
	n = min(n, e.Buf.LineCount()-row)

	lines := make([][]rune, 0, n)
	for i := range n {
		lines = append(lines, slices.Clone(e.Buf.Line(row+i)))
	}
	e.SetClip(register.Linewise(lines))
}

func yankChars(e *state.Editor, row, from, to int) {
	e.SetClip(register.Charwise([][]rune{slices.Clone(e.Buf.Line(row)[from:to])}))
}

// YankWord is 'yw', YankToWordEnd is 'ye' and YankToPrevWord is 'yb'. Like the
// delete operators they stop at the end of the line, and like VIM they leave
// the cursor at the start of what was yanked.
func YankWord(e *state.Editor) {
	row, col := motion.NextWordFrom(e.Buf, e.Row, e.Col)
	yankForwardTo(e, row, col)
}

func YankToWordEnd(e *state.Editor) {
	row, col := motion.EndOfWordFrom(e.Buf, e.Row, e.Col)
	yankForwardTo(e, row, col+1)
}

func yankForwardTo(e *state.Editor, row, col int) {
	if row != e.Row {
		col = e.Buf.RuneLen(e.Row)
	}
	if col <= e.Col {
		return
	}
	yankChars(e, e.Row, e.Col, col)
}

func YankToPrevWord(e *state.Editor) {
	row, col := motion.PrevWordFrom(e.Buf, e.Row, e.Col)
	if row != e.Row {
		col = 0
	}
	if col >= e.Col {
		return
	}

	yankChars(e, e.Row, col, e.Col)
	e.Col = col
}

func PasteAfter(e *state.Editor)  { paste(e, true) }
func PasteBefore(e *state.Editor) { paste(e, false) }

func paste(e *state.Editor, after bool) {
	if e.Clip.Empty() {
		return
	}

	if e.Clip.IsLinewise() {
		pasteLines(e, after)
		return
	}
	pasteChars(e, after)
}

func pasteLines(e *state.Editor, after bool) {
	row := e.Row
	if after {
		row++
	}

	at := row
	for range e.Count() {
		for _, line := range e.Clip.Content() {
			e.TouchInsertLine(at)
			e.Buf.InsertLine(at)
			e.Buf.SetLine(at, slices.Clone(line))
			at++
		}
	}

	e.Row, e.Col = row, 0
	e.Modified = true
}

func pasteChars(e *state.Editor, after bool) {
	text := e.Clip.Repeated(e.Count())
	line := slices.Clone(e.Buf.Line(e.Row))

	col := e.Col
	if after && len(line) > 0 {
		col++
	}
	col = min(col, len(line))

	e.TouchLine(e.Row)

	if len(text) == 1 {
		e.Buf.SetLine(e.Row, slices.Insert(line, col, text[0]...))
		e.Col = max(col+len(text[0])-1, 0)
		e.Modified = true
		return
	}

	// A register holding a run that spanned lines splits the line it is put
	// into: its first line joins what was before the cursor, its last one what
	// came after.
	tail := slices.Clone(line[col:])
	e.Buf.SetLine(e.Row, append(line[:col], text[0]...))

	row := e.Row
	for _, l := range text[1:] {
		row++
		e.TouchInsertLine(row)
		e.Buf.InsertLine(row)
		e.Buf.SetLine(row, slices.Clone(l))
	}
	e.Buf.SetLine(row, append(e.Buf.Line(row), tail...))

	e.Col = col
	e.Modified = true
}
