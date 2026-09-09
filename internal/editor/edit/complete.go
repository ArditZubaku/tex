package edit

import (
	"slices"
	"strings"

	"github.com/ArditZubaku/tex/internal/editor/complete"
	"github.com/ArditZubaku/tex/internal/editor/state"
)

// Accept is a candidate settled on: what has been typed of the word is replaced
// by what the server offered, and the edits it named elsewhere in the file go
// in with it. Those are what an import is — gopls offers a name out of a package
// the file does not import yet, and without the line it asks for alongside, the
// name it just wrote is a compile error.
func Accept(e *state.Editor, item complete.Item) {
	row := e.Row
	from := min(max(item.From, 0), e.Buf.RuneLen(row))
	to := min(max(e.Col, from), e.Buf.RuneLen(row))

	// The edits are applied furthest first so that the rows of the ones still
	// to come are the rows they were named in. Only what is above the word
	// moves the cursor, and only by the lines it added or took away.
	before, after := split(item.Extra, row)
	for i := len(after) - 1; i >= 0; i-- {
		replace(e, after[i])
	}

	text := []rune(item.Text)
	e.TouchLine(row)
	e.Buf.SetLine(row, slices.Replace(slices.Clone(e.Buf.Line(row)), from, to, text...))

	shift := 0
	for i := len(before) - 1; i >= 0; i-- {
		shift += replace(e, before[i])
	}

	e.Row, e.Col = row+shift, from+len(text)
	e.Modified = true
}

// An edit touching the row being completed on is dropped: what goes on that row
// is the candidate itself, and a server asking for both is a server whose two
// answers would land on top of each other.
func split(edits []complete.Edit, row int) (before, after []complete.Edit) {
	ordered := slices.SortedFunc(slices.Values(edits), func(a, b complete.Edit) int {
		if a.Row != b.Row {
			return a.Row - b.Row
		}

		return a.Col - b.Col
	})

	for _, one := range ordered {
		switch {
		case one.EndRow < row:
			before = append(before, one)
		case one.Row > row:
			after = append(after, one)
		}
	}

	return before, after
}

// replace puts text where a stretch of the file was, and answers with how many
// lines the file gained or lost by it.
func replace(e *state.Editor, at complete.Edit) int {
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
		e.Buf.SetLine(row, append(append(head, []rune(parts[0])...), tail...))

		return row - endRow
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

	return (last - row) - (endRow - row)
}

func colIn(e *state.Editor, row, col int) int {
	return min(max(col, 0), e.Buf.RuneLen(row))
}
