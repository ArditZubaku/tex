package edit

import (
	"slices"
	"strings"

	"github.com/ArditZubaku/tex/internal/editor/complete"
	"github.com/ArditZubaku/tex/internal/editor/state"
)

// Accept is a candidate settled on: what has been typed of the word is replaced
// by what the server offered, and the edits it named elsewhere in the file go
// in with it. Those are what an import is — a server offers a name out of a
// package the file does not import yet, and without the line it asks for
// alongside, the name it just wrote is a compile error.
func Accept(e *state.Editor, item complete.Item) {
	row := e.Row
	from := min(max(item.From, 0), e.Buf.RuneLen(row))
	to := min(max(e.Col, from), e.Buf.RuneLen(row))

	text := []rune(item.Text)
	e.TouchLine(row)
	e.Buf.SetLine(row, slices.Replace(slices.Clone(e.Buf.Line(row)), from, to, text...))
	e.Row, e.Col = row, from+len(text)
	e.Modified = true

	Elsewhere(e, item.Extra)
}

// Elsewhere is the edits a candidate needs away from the word itself. It is its
// own entry because a server may hold them back until the candidate is settled
// on and answer with them a frame or two after the name has already gone in.
//
// They are applied furthest first, so that the rows of the ones still to come
// are the rows they were named in, and the cursor is carried through each of
// them: an import written above the line being typed on moves that line down,
// and one written in front of the cursor on its own line moves it along.
func Elsewhere(e *state.Editor, edits []complete.Edit) {
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
		apply(e, ordered[i])
	}
	e.Modified = true
}

func apply(e *state.Editor, at complete.Edit) {
	row, col := e.Row, e.Col
	ends := reaches(at.EndRow, at.EndCol, row, col)

	// An edit that starts at or before the cursor and ends past it is the
	// server rewriting the very text just settled on, which its own candidate
	// has already written. Two answers landing on top of each other is worse
	// than one of them being dropped.
	if !ends && reaches(at.Row, at.Col, row, col) {
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
