package diag

// A span longer than this is a server that could not place what it found —
// "expected declaration" over the rest of the file — and underlining all of it
// says nothing beyond that the file is broken.
const maxRows = 8

// toRowEnd is a span that reaches past every rune the row holds, which is what
// the middle rows of a multi-line one do.
const toRowEnd = -1

type span struct {
	from, to int
	severity Severity
}

// The row's spans are collected into a buffer that outlives the frame, the way
// the search's own hits are: a redraw asks about every visible row and keeps
// none of them. A Rows therefore holds only until the next one is taken.
var rowScratch []span

// Rows is what one row of a file has said about it, resolved once for the row
// rather than once for each of its cells.
type Rows struct {
	spans []span
	worst Severity
}

func (f File) Row(row int) Rows {
	spans, worst := rowScratch[:0], None
	for _, note := range f {
		from, to, ok := note.on(row)
		if !ok {
			continue
		}

		spans = append(spans, span{from: from, to: to, severity: note.Severity})
		worst = worse(note.Severity, worst)
	}
	rowScratch = spans

	return Rows{spans: spans, worst: worst}
}

// Worst is the worst thing said about the row, or None. It is what the line
// number is recoloured in, since the row is marked whether or not what was said
// about it reaches a column that is on screen.
func (r Rows) Worst() Severity { return r.worst }

// Under is the worst thing said about one column of the row.
func (r Rows) Under(col int) Severity {
	worst := None
	for _, at := range r.spans {
		if col < at.from || (at.to != toRowEnd && col >= at.to) {
			continue
		}
		worst = worse(at.severity, worst)
	}

	return worst
}

// on is where along a row one note reaches, if it reaches it at all: its own
// column on the row it starts, the whole of every row between, and up to its
// end on the row it ends.
func (n Note) on(row int) (from, to int, ok bool) {
	if row < n.Row || row > min(n.EndRow, n.Row+maxRows-1) {
		return 0, 0, false
	}
	// A range whose end is the start of a row ends where that row begins, so
	// none of it is on it.
	if row == n.EndRow && row != n.Row && n.EndCol == 0 {
		return 0, 0, false
	}

	from, to = 0, toRowEnd
	if row == n.Row {
		from = n.Col
	}
	if row == n.EndRow {
		to = n.EndCol
	}
	// A range of no width points at the rune it sits on, which is the only
	// thing there is to draw for it.
	if from == to {
		to = from + 1
	}

	return from, to, true
}
