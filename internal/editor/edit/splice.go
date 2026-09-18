package edit

import (
	"slices"
	"strings"

	"github.com/ArditZubaku/tex/internal/editor/complete"
	"github.com/ArditZubaku/tex/internal/editor/state"
)

// Splice applies edits away from wherever the cursor is, furthest first, so
// that the rows of the ones still to come are the rows they were named in, and
// carries the cursor through each of them: one written above the line it is on
// moves that line down, and one written in front of it on its own line moves it
// along.
func Splice(e *state.Editor, edits []complete.Edit) { splice(e, edits, false) }

func splice(e *state.Editor, edits []complete.Edit, dedup bool) {
	if len(edits) == 0 {
		return
	}

	ordered := slices.SortedFunc(slices.Values(edits), func(a, b complete.Edit) int {
		if a.Row != b.Row {
			return a.Row - b.Row
		}

		return a.Col - b.Col
	})

	for i := len(ordered) - 1; i >= 0; i-- {
		apply(e, ordered[i], dedup)
	}
	e.Modified = true
}

func apply(e *state.Editor, at complete.Edit, dedup bool) {
	row, col := e.Row, e.Col
	ends := reaches(at.EndRow, at.EndCol, row, col)

	// dedup is completion's own guard against a server rewriting the very text
	// its candidate just settled on; nothing else asks for it.
	if dedup && !ends && reaches(at.Row, at.Col, row, col) {
		return
	}

	endRow, endCol := replace(e, at)
	if !ends {
		return
	}

	if at.EndRow < row {
		e.Row = row + endRow - at.EndRow

		return
	}
	e.Row, e.Col = endRow, endCol+col-at.EndCol
}

// reaches says whether one position is at or before another, which is how an
// edit is told from one the cursor sits in front of.
func reaches(row, col, atRow, atCol int) bool {
	return row < atRow || (row == atRow && col <= atCol)
}

// replace puts text where a stretch of the file was, and answers with where the
// end of that stretch has moved to.
func replace(e *state.Editor, at complete.Edit) (int, int) {
	row := min(max(at.Row, 0), e.Buf.LineCount()-1)
	endRow := min(max(at.EndRow, row), e.Buf.LineCount()-1)

	head := slices.Clone(e.Buf.Line(row)[:colIn(e, row, at.Col)])
	tail := slices.Clone(e.Buf.Line(endRow)[colIn(e, endRow, at.EndCol):])

	for gone := endRow; gone > row; gone-- {
		e.TouchDeleteLine(gone)
		e.Buf.DeleteLine(gone)
	}

	parts := strings.Split(at.Text, "\n")
	e.TouchLine(row)

	if len(parts) == 1 {
		put := []rune(parts[0])
		e.Buf.SetLine(row, append(append(head, put...), tail...))

		return row, len(head) + len(put)
	}
	e.Buf.SetLine(row, append(head, []rune(parts[0])...))

	last := row
	for i, part := range parts[1:] {
		last++
		e.TouchInsertLine(last)
		e.Buf.InsertLine(last)

		line := []rune(part)
		if i == len(parts)-2 {
			line = append(line, tail...)
		}
		e.Buf.SetLine(last, line)
	}

	return last, len([]rune(parts[len(parts)-1]))
}

func colIn(e *state.Editor, row, col int) int {
	return min(max(col, 0), e.Buf.RuneLen(row))
}
