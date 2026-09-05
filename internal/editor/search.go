package editor

import "github.com/ArditZubaku/tex/internal/search"

var (
	searchPat  search.Pattern // the search.Pattern n and N repeat
	searchBack bool           // the direction it was last run in
	hlSearch   bool
	hitCols    []int // scratch for the matches drawn on one line
)

func startSearchForward()  { startPrompt('/') }
func startSearchBackward() { startPrompt('?') }

// commitSearch keeps the last search.Pattern when nothing was typed, which is how VIM
// repeats a search from the prompt; the delimiter still says which way to go.
func commitSearch(input []rune, back bool) {
	if len(input) > 0 {
		searchPat = search.New(input)
	}
	searchBack = back

	jumpToMatch(searchBack)
}

func nextMatch() { jumpToMatch(searchBack) }
func prevMatch() { jumpToMatch(!searchBack) }

func jumpToMatch(back bool) {
	if searchPat.Empty() {
		return
	}
	hlSearch = true

	row, col, ok := search.Find(buf, searchPat, currentRow, currentCol, back)
	if !ok {
		statusMsg = "Pattern not found: " + string(searchPat.Runes())
		return
	}

	currentRow, currentCol = row, col
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
	if !hlSearch || searchPat.Empty() {
		return hitScan{}
	}
	hitCols = searchPat.MatchesIn(buf, row, hitCols[:0])

	return hitScan{cols: hitCols, width: len(searchPat.Runes())}
}

func (h *hitScan) covers(col int) bool {
	for h.next < len(h.cols) && h.cols[h.next]+h.width <= col {
		h.next++
	}

	return h.next < len(h.cols) && col >= h.cols[h.next]
}
