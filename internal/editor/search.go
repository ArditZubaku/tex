package editor

import "github.com/ArditZubaku/tex/internal/search"

var hitCols []int // scratch for the matches drawn on one line

func startSearchForward()  { startPrompt('/') }
func startSearchBackward() { startPrompt('?') }

// commitSearch keeps the last search.Pattern when nothing was typed, which is how VIM
// repeats a search from the prompt; the delimiter still says which way to go.
func commitSearch(input []rune, back bool) {
	if len(input) > 0 {
		ed.SearchPat = search.New(input)
	}
	ed.SearchBack = back

	jumpToMatch(ed.SearchBack)
}

func nextMatch() { jumpToMatch(ed.SearchBack) }
func prevMatch() { jumpToMatch(!ed.SearchBack) }

func jumpToMatch(back bool) {
	if ed.SearchPat.Empty() {
		return
	}
	ed.HlSearch = true

	row, col, ok := search.Find(ed.Buf, ed.SearchPat, ed.Row, ed.Col, back)
	if !ok {
		ed.StatusMsg = "Pattern not found: " + string(ed.SearchPat.Runes())
		return
	}

	ed.Row, ed.Col = row, col
}

// hitScan tells the renderer which columns of a line fall inside a match.
// Columns are drawn left to right, so one index walking the hits covers a whole
// line, whatever it is scrolled to.
type hitScan struct {
	cols  []int
	width int
	next  int
}

func lineHits(row int) hitScan {
	if !ed.HlSearch || ed.SearchPat.Empty() {
		return hitScan{}
	}
	hitCols = ed.SearchPat.MatchesIn(ed.Buf, row, hitCols[:0])

	return hitScan{cols: hitCols, width: len(ed.SearchPat.Runes())}
}

func (h *hitScan) covers(col int) bool {
	for h.next < len(h.cols) && h.cols[h.next]+h.width <= col {
		h.next++
	}

	return h.next < len(h.cols) && col >= h.cols[h.next]
}
