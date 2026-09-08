package edit

import (
	"slices"

	"github.com/ArditZubaku/tex/internal/editor/register"
	"github.com/ArditZubaku/tex/internal/editor/state"
)

// Visual mode marks a run of text for the operator that follows it: 'v' marks
// runes and 'V' whole lines, from the anchor the mode was entered at to
// wherever the cursor has moved since. Both ends are part of the selection, the
// way VIM's own 'selection=inclusive' has them.
type Span struct {
	startRow, startCol int
	endRow, endCol     int
	linewise           bool
	Active             bool
}

func StartVisualChar(e *state.Editor) { startVisual(e, false) }
func StartVisualLine(e *state.Editor) { startVisual(e, true) }

func startVisual(e *state.Editor, linewise bool) {
	e.Mode, e.VisualLine = state.VisualMode, linewise
	e.AnchorRow, e.AnchorCol = e.Row, e.Col
}

// Inside Visual mode 'v' and 'V' switch between the two shapes of selection,
// and leave the mode when they name the shape it already has.
func ToggleVisualChar(e *state.Editor) {
	switch {
	case e.Mode != state.VisualMode:
		return
	case e.VisualLine:
		e.VisualLine = false
	default:
		ExitVisual(e)
	}
}

func ToggleVisualLine(e *state.Editor) {
	switch {
	case e.Mode != state.VisualMode:
		return
	case !e.VisualLine:
		e.VisualLine = true
	default:
		ExitVisual(e)
	}
}

func ExitVisual(e *state.Editor) {
	e.Mode = state.ReadMode
	e.ClampCol()
}

// SwapVisualEnds is 'o': the cursor takes the anchor's place, so the end that a
// motion drags along becomes the other one.
func SwapVisualEnds(e *state.Editor) {
	if e.Mode != state.VisualMode {
		return
	}
	e.AnchorRow, e.AnchorCol, e.Row, e.Col = e.Row, e.Col, e.AnchorRow, e.AnchorCol
	e.ClampCol()
}

// Selection puts the two ends in buffer order. Outside Visual mode it
// reports an inactive selection, which is what leaves the operators below
// no-ops once one of them has run: a count typed before an operator repeats it,
// and repeating it must not eat the text that followed the selection.
func Selection(e *state.Editor) Span {
	if e.Mode != state.VisualMode {
		return Span{}
	}

	s := Span{
		startRow: e.AnchorRow, startCol: e.AnchorCol,
		endRow: e.Row, endCol: e.Col,
		linewise: e.VisualLine,
		Active:   true,
	}
	if s.endRow < s.startRow || (s.endRow == s.startRow && s.endCol < s.startCol) {
		s.startRow, s.startCol, s.endRow, s.endCol = s.endRow, s.endCol, s.startRow, s.startCol
	}

	return s
}

// Covers says whether a rune of line row falls inside the selection. The column
// one past the last rune stands for the line break, which is painted whenever
// the selection carries on onto the line below, the way VIM paints it.
func (s Span) Covers(row, col, lineLen int) bool {
	if !s.Active || row < s.startRow || row > s.endRow || col > lineLen {
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

func DeleteSelection(e *state.Editor) {
	s := Selection(e)
	if !s.Active {
		return
	}
	ExitVisual(e)
	e.Row = s.startRow

	if s.linewise {
		DeleteLines(e, s.endRow-s.startRow+1)
	} else {
		deleteSpan(e, s)
	}
	e.ClampCol()
}

func YankSelection(e *state.Editor) {
	s := Selection(e)
	if !s.Active {
		return
	}
	ExitVisual(e)

	yankSpan(e, s)
	e.Row = s.startRow
	if !s.linewise {
		e.Col = s.startCol
	}
	e.ClampCol()
}

// PasteSelection is visual mode's 'p': the selection is removed like 'd'
// would remove it, and the register's previous contents are put in its place.
// The register itself is left holding the removed text, same as any other
// delete — pasting over a selection is VIM's own trade for that convenience.
func PasteSelection(e *state.Editor) {
	s := Selection(e)
	if !s.Active || e.Clip.Empty() {
		return
	}
	toInsert := e.Clip

	ExitVisual(e)
	e.Row = s.startRow

	if s.linewise {
		DeleteLines(e, s.endRow-s.startRow+1)
	} else {
		deleteSpan(e, s)
	}

	removed := e.Clip
	e.Clip = toInsert
	if s.linewise {
		pasteLines(e, false)
	} else {
		pasteChars(e, false)
	}
	e.Clip = removed

	e.ClampCol()
}

// ChangeSelection is 'c': the selection goes into the register the way 'd'
// takes it, and typing carries on where it was.
func ChangeSelection(e *state.Editor) {
	s := Selection(e)
	if !s.Active {
		return
	}
	ExitVisual(e)
	e.Row = s.startRow

	if s.linewise {
		clearLines(e, s.endRow-s.startRow+1)
	} else {
		deleteSpan(e, s)
	}
	e.EnterEditMode()
}

// clearLines is what a linewise change deletes: the lines after the first go,
// and the first is emptied rather than dropped, so there is a line to type on.
func clearLines(e *state.Editor, n int) {
	n = min(n, e.Buf.LineCount()-e.Row)
	YankLines(e, e.Row, n)

	for range n - 1 {
		e.TouchDeleteLine(e.Row + 1)
		e.Buf.DeleteLine(e.Row + 1)
	}
	e.TouchLine(e.Row)
	e.Buf.DeleteRunes(e.Row, 0, e.Buf.RuneLen(e.Row))

	e.Col = 0
	e.Modified = true
}

// deleteSpan takes the runes a charwise selection covers out, joining what is
// left of its last line onto its first when it spans more than one.
func deleteSpan(e *state.Editor, s Span) {
	yankSpan(e, s)
	e.Row, e.Col = s.startRow, s.startCol

	if s.startRow == s.endRow {
		e.TouchLine(s.startRow)
		e.Buf.DeleteRunes(s.startRow, s.startCol, s.endCol+1)
		e.Modified = true
		return
	}

	// The lines between the two ends go first: dropping a line renumbers the
	// rows the undo log is keyed by, so it has to happen before the ends are
	// recorded, not between them.
	for range s.endRow - s.startRow - 1 {
		e.TouchDeleteLine(s.startRow + 1)
		e.Buf.DeleteLine(s.startRow + 1)
	}

	e.TouchLine(s.startRow)
	e.Buf.DeleteRunes(s.startRow, s.startCol, e.Buf.RuneLen(s.startRow))
	e.TouchLine(s.startRow + 1)
	e.Buf.DeleteRunes(s.startRow+1, 0, s.endCol+1)
	e.TouchDeleteLine(s.startRow + 1)
	e.Buf.JoinLine(s.startRow)

	e.Modified = true
}

// yankSpan fills the register with what the selection covers, in the shape the
// selection has: whole lines for 'V', and for 'v' the run of runes between its
// ends, which spans several lines when the selection did.
func yankSpan(e *state.Editor, s Span) {
	if s.linewise {
		YankLines(e, s.startRow, s.endRow-s.startRow+1)
		return
	}

	if s.startRow == s.endRow {
		e.SetClip(register.Charwise([][]rune{runeSpan(e, s.startRow, s.startCol, s.endCol+1)}))
		return
	}

	lines := make([][]rune, 0, s.endRow-s.startRow+1)
	lines = append(lines, runeSpan(e, s.startRow, s.startCol, e.Buf.RuneLen(s.startRow)))
	for row := s.startRow + 1; row < s.endRow; row++ {
		lines = append(lines, slices.Clone(e.Buf.Line(row)))
	}
	lines = append(lines, runeSpan(e, s.endRow, 0, s.endCol+1))

	e.SetClip(register.Charwise(lines))
}

func runeSpan(e *state.Editor, row, from, to int) []rune {
	line := e.Buf.Line(row)
	from, to = min(max(from, 0), len(line)), min(to, len(line))
	if from >= to {
		return nil
	}

	return slices.Clone(line[from:to])
}
