// Package find is what the editor looks things up with: the '/' search, the
// jumps to a declaration and its references, and the popup those and the file
// picker are all listed in.
package find

import (
	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/ArditZubaku/tex/internal/search"
)

var hitCols []int // scratch for the matches drawn on one line

// CommitSearch keeps the last pattern when nothing was typed, which is how VIM
// repeats a search from the prompt; the delimiter still says which way to go.
func CommitSearch(e *state.Editor, input []rune, back bool) {
	if len(input) > 0 {
		e.SearchPat = search.New(input)
	}
	e.SearchBack = back

	jumpToMatch(e, e.SearchBack)
}

func NextMatch(e *state.Editor) { jumpToMatch(e, e.SearchBack) }
func PrevMatch(e *state.Editor) { jumpToMatch(e, !e.SearchBack) }

func jumpToMatch(e *state.Editor, back bool) {
	if e.SearchPat.Empty() {
		return
	}
	e.HlSearch = true

	row, col, ok := search.Find(e.Buf, e.SearchPat, e.Row, e.Col, back)
	if !ok {
		e.StatusMsg = "Pattern not found: " + string(e.SearchPat.Runes())
		return
	}

	e.Row, e.Col = row, col
}

// HitScan tells the renderer which columns of a line fall inside a match.
// Columns are drawn left to right, so one index walking the hits covers a whole
// line, whatever it is scrolled to.
type HitScan struct {
	cols  []int
	width int
	next  int
}

func LineHits(e *state.Editor, row int) HitScan {
	if !e.HlSearch || e.SearchPat.Empty() {
		return HitScan{}
	}
	hitCols = e.SearchPat.MatchesIn(e.Buf, row, hitCols[:0])

	return HitScan{cols: hitCols, width: len(e.SearchPat.Runes())}
}

func (h HitScan) Cols() []int { return h.cols }

func (h *HitScan) Covers(col int) bool {
	for h.next < len(h.cols) && h.cols[h.next]+h.width <= col {
		h.next++
	}

	return h.next < len(h.cols) && col >= h.cols[h.next]
}
