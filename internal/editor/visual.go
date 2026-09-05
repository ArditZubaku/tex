package editor

import (
	"slices"

	"github.com/ArditZubaku/tex/internal/editor/register"
)

// Visual mode marks a run of text for the operator that follows it: 'v' marks
// runes and 'V' whole lines, from the anchor the mode was entered at to
// wherever the cursor has moved since. Both ends are part of the selection, the
// way VIM's own 'selection=inclusive' has them.
var (
	visualLine           bool
	anchorRow, anchorCol int
)

type selection struct {
	startRow, startCol int
	endRow, endCol     int
	linewise           bool
	active             bool
}

func startVisualChar() { startVisual(false) }
func startVisualLine() { startVisual(true) }

func startVisual(linewise bool) {
	mode, visualLine = VisualMode, linewise
	anchorRow, anchorCol = currentRow, currentCol
}

// Inside Visual mode 'v' and 'V' switch between the two shapes of selection,
// and leave the mode when they name the shape it already has.
func toggleVisualChar() {
	switch {
	case mode != VisualMode:
		return
	case visualLine:
		visualLine = false
	default:
		exitVisual()
	}
}

func toggleVisualLine() {
	switch {
	case mode != VisualMode:
		return
	case !visualLine:
		visualLine = true
	default:
		exitVisual()
	}
}

func exitVisual() {
	mode = ReadMode
	clampCol()
}

// swapVisualEnds is 'o': the cursor takes the anchor's place, so the end that a
// motion drags along becomes the other one.
func swapVisualEnds() {
	if mode != VisualMode {
		return
	}
	anchorRow, anchorCol, currentRow, currentCol = currentRow, currentCol, anchorRow, anchorCol
	clampCol()
}

// visualSelection puts the two ends in buffer order. Outside Visual mode it
// reports an inactive selection, which is what leaves the operators below
// no-ops once one of them has run: a count typed before an operator repeats it,
// and repeating it must not eat the text that followed the selection.
func visualSelection() selection {
	if mode != VisualMode {
		return selection{}
	}

	s := selection{
		startRow: anchorRow, startCol: anchorCol,
		endRow: currentRow, endCol: currentCol,
		linewise: visualLine,
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
	currentRow = s.startRow

	if s.linewise {
		deleteLines(s.endRow - s.startRow + 1)
	} else {
		deleteSpan(s)
	}
	clampCol()
}

func yankSelection() {
	s := visualSelection()
	if !s.active {
		return
	}
	exitVisual()

	yankSpan(s)
	currentRow = s.startRow
	if !s.linewise {
		currentCol = s.startCol
	}
	clampCol()
}

// changeSelection is 'c': the selection goes into the register the way 'd'
// takes it, and typing carries on where it was.
func changeSelection() {
	s := visualSelection()
	if !s.active {
		return
	}
	exitVisual()
	currentRow = s.startRow

	if s.linewise {
		clearLines(s.endRow - s.startRow + 1)
	} else {
		deleteSpan(s)
	}
	enterEditMode()
}

// clearLines is what a linewise change deletes: the lines after the first go,
// and the first is emptied rather than dropped, so there is a line to type on.
func clearLines(n int) {
	n = min(n, buf.LineCount()-currentRow)
	yankLines(currentRow, n)

	for range n - 1 {
		touchDeleteLine(currentRow + 1)
		buf.DeleteLine(currentRow + 1)
	}
	touchLine(currentRow)
	buf.DeleteRunes(currentRow, 0, buf.RuneLen(currentRow))

	currentCol = 0
	modified = true
}

// deleteSpan takes the runes a charwise selection covers out, joining what is
// left of its last line onto its first when it spans more than one.
func deleteSpan(s selection) {
	yankSpan(s)
	currentRow, currentCol = s.startRow, s.startCol

	if s.startRow == s.endRow {
		touchLine(s.startRow)
		buf.DeleteRunes(s.startRow, s.startCol, s.endCol+1)
		modified = true
		return
	}

	// The lines between the two ends go first: dropping a line renumbers the
	// rows the undo log is keyed by, so it has to happen before the ends are
	// recorded, not between them.
	for range s.endRow - s.startRow - 1 {
		touchDeleteLine(s.startRow + 1)
		buf.DeleteLine(s.startRow + 1)
	}

	touchLine(s.startRow)
	buf.DeleteRunes(s.startRow, s.startCol, buf.RuneLen(s.startRow))
	touchLine(s.startRow + 1)
	buf.DeleteRunes(s.startRow+1, 0, s.endCol+1)
	touchDeleteLine(s.startRow + 1)
	buf.JoinLine(s.startRow)

	modified = true
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
		clipboard = register.Charwise([][]rune{runeSpan(s.startRow, s.startCol, s.endCol+1)})
		return
	}

	lines := make([][]rune, 0, s.endRow-s.startRow+1)
	lines = append(lines, runeSpan(s.startRow, s.startCol, buf.RuneLen(s.startRow)))
	for row := s.startRow + 1; row < s.endRow; row++ {
		lines = append(lines, slices.Clone(buf.Line(row)))
	}
	lines = append(lines, runeSpan(s.endRow, 0, s.endCol+1))

	clipboard = register.Charwise(lines)
}

func runeSpan(row, from, to int) []rune {
	line := buf.Line(row)
	from, to = min(max(from, 0), len(line)), min(to, len(line))
	if from >= to {
		return nil
	}

	return slices.Clone(line[from:to])
}
