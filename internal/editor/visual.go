package editor

import (
	"slices"

	"github.com/ArditZubaku/tex/internal/editor/register"
	"github.com/ArditZubaku/tex/internal/editor/state"
)

// Visual mode marks a run of text for the operator that follows it: 'v' marks
// runes and 'V' whole lines, from the anchor the mode was entered at to
// wherever the cursor has moved since. Both ends are part of the selection, the
// way VIM's own 'selection=inclusive' has them.
type selection struct {
	startRow, startCol int
	endRow, endCol     int
	linewise           bool
	active             bool
}

func startVisualChar() { startVisual(false) }
func startVisualLine() { startVisual(true) }

func startVisual(linewise bool) {
	ed.Mode, ed.VisualLine = state.VisualMode, linewise
	ed.AnchorRow, ed.AnchorCol = ed.Row, ed.Col
}

// Inside Visual mode 'v' and 'V' switch between the two shapes of selection,
// and leave the mode when they name the shape it already has.
func toggleVisualChar() {
	switch {
	case ed.Mode != state.VisualMode:
		return
	case ed.VisualLine:
		ed.VisualLine = false
	default:
		exitVisual()
	}
}

func toggleVisualLine() {
	switch {
	case ed.Mode != state.VisualMode:
		return
	case !ed.VisualLine:
		ed.VisualLine = true
	default:
		exitVisual()
	}
}

func exitVisual() {
	ed.Mode = state.ReadMode
	ed.ClampCol()
}

// swapVisualEnds is 'o': the cursor takes the anchor's place, so the end that a
// motion drags along becomes the other one.
func swapVisualEnds() {
	if ed.Mode != state.VisualMode {
		return
	}
	ed.AnchorRow, ed.AnchorCol, ed.Row, ed.Col = ed.Row, ed.Col, ed.AnchorRow, ed.AnchorCol
	ed.ClampCol()
}

// visualSelection puts the two ends in buffer order. Outside Visual mode it
// reports an inactive selection, which is what leaves the operators below
// no-ops once one of them has run: a count typed before an operator repeats it,
// and repeating it must not eat the text that followed the selection.
func visualSelection() selection {
	if ed.Mode != state.VisualMode {
		return selection{}
	}

	s := selection{
		startRow: ed.AnchorRow, startCol: ed.AnchorCol,
		endRow: ed.Row, endCol: ed.Col,
		linewise: ed.VisualLine,
		active:   true,
	}
	if s.endRow < s.startRow || (s.endRow == s.startRow && s.endCol < s.startCol) {
		s.startRow, s.startCol, s.endRow, s.endCol = s.endRow, s.endCol, s.startRow, s.startCol
	}

	return s
}

// covers says whether a rune of line row falls inside the selection. The column
// one past the last rune stands for the line break, which is painted whenever
// the selection carries on onto the line below, the way VIM paints it.
func (s selection) covers(row, col, lineLen int) bool {
	if !s.active || row < s.startRow || row > s.endRow || col > lineLen {
		return false
	}
	if col == lineLen && !s.linewise && row == s.endRow {
		return false
	}
	if s.linewise {
		return true
	}
	if row == s.startRow && col < s.startCol {
		return false
	}

	return row != s.endRow || col <= s.endCol
}

func deleteSelection() {
	s := visualSelection()
	if !s.active {
		return
	}
	exitVisual()
	ed.Row = s.startRow

	if s.linewise {
		deleteLines(s.endRow - s.startRow + 1)
	} else {
		deleteSpan(s)
	}
	ed.ClampCol()
}

func yankSelection() {
	s := visualSelection()
	if !s.active {
		return
	}
	exitVisual()

	yankSpan(s)
	ed.Row = s.startRow
	if !s.linewise {
		ed.Col = s.startCol
	}
	ed.ClampCol()
}

// changeSelection is 'c': the selection goes into the register the way 'd'
// takes it, and typing carries on where it was.
func changeSelection() {
	s := visualSelection()
	if !s.active {
		return
	}
	exitVisual()
	ed.Row = s.startRow

	if s.linewise {
		clearLines(s.endRow - s.startRow + 1)
	} else {
		deleteSpan(s)
	}
	ed.EnterEditMode()
}

// clearLines is what a linewise change deletes: the lines after the first go,
// and the first is emptied rather than dropped, so there is a line to type on.
func clearLines(n int) {
	n = min(n, ed.Buf.LineCount()-ed.Row)
	yankLines(ed.Row, n)

	for range n - 1 {
		ed.TouchDeleteLine(ed.Row + 1)
		ed.Buf.DeleteLine(ed.Row + 1)
	}
	ed.TouchLine(ed.Row)
	ed.Buf.DeleteRunes(ed.Row, 0, ed.Buf.RuneLen(ed.Row))

	ed.Col = 0
	ed.Modified = true
}

// deleteSpan takes the runes a charwise selection covers out, joining what is
// left of its last line onto its first when it spans more than one.
func deleteSpan(s selection) {
	yankSpan(s)
	ed.Row, ed.Col = s.startRow, s.startCol

	if s.startRow == s.endRow {
		ed.TouchLine(s.startRow)
		ed.Buf.DeleteRunes(s.startRow, s.startCol, s.endCol+1)
		ed.Modified = true
		return
	}

	// The lines between the two ends go first: dropping a line renumbers the
	// rows the undo log is keyed by, so it has to happen before the ends are
	// recorded, not between them.
	for range s.endRow - s.startRow - 1 {
		ed.TouchDeleteLine(s.startRow + 1)
		ed.Buf.DeleteLine(s.startRow + 1)
	}

	ed.TouchLine(s.startRow)
	ed.Buf.DeleteRunes(s.startRow, s.startCol, ed.Buf.RuneLen(s.startRow))
	ed.TouchLine(s.startRow + 1)
	ed.Buf.DeleteRunes(s.startRow+1, 0, s.endCol+1)
	ed.TouchDeleteLine(s.startRow + 1)
	ed.Buf.JoinLine(s.startRow)

	ed.Modified = true
}

// yankSpan fills the register with what the selection covers, in the shape the
// selection has: whole lines for 'V', and for 'v' the run of runes between its
// ends, which spans several lines when the selection did.
func yankSpan(s selection) {
	if s.linewise {
		yankLines(s.startRow, s.endRow-s.startRow+1)
		return
	}

	if s.startRow == s.endRow {
		ed.Clip = register.Charwise([][]rune{runeSpan(s.startRow, s.startCol, s.endCol+1)})
		return
	}

	lines := make([][]rune, 0, s.endRow-s.startRow+1)
	lines = append(lines, runeSpan(s.startRow, s.startCol, ed.Buf.RuneLen(s.startRow)))
	for row := s.startRow + 1; row < s.endRow; row++ {
		lines = append(lines, slices.Clone(ed.Buf.Line(row)))
	}
	lines = append(lines, runeSpan(s.endRow, 0, s.endCol+1))

	ed.Clip = register.Charwise(lines)
}

func runeSpan(row, from, to int) []rune {
	line := ed.Buf.Line(row)
	from, to = min(max(from, 0), len(line)), min(to, len(line))
	if from >= to {
		return nil
	}

	return slices.Clone(line[from:to])
}
