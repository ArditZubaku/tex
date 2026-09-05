package editor

import (
	"bytes"
	"slices"
	"unicode/utf8"
)

// pattern is one search string held in both shapes the buffer keeps lines in:
// raw bytes for the lines still on disk, runes for the ones the overlay holds.
type pattern struct {
	raw   []byte
	runes []rune
}

func newPattern(runes []rune) pattern {
	return pattern{raw: []byte(string(runes)), runes: runes}
}

func (p pattern) empty() bool {
	return len(p.runes) == 0
}

var (
	searchPat  pattern // the pattern n and N repeat
	searchBack bool    // the direction it was last run in
	hlSearch   bool
	hitCols    []int // scratch for the matches drawn on one line
)

// matchesIn appends the rune column of every occurrence of p in line row, in
// ascending order and overlapping matches included. An unedited line is matched
// over its raw bytes, so a search decodes nothing it does not have to:
// bytes.Index walks the line whole, and only the text in front of a hit is
// counted, to turn its byte offset into the column the cursor moves to.
func (p pattern) matchesIn(row int, out []int) []int {
	if p.empty() || row < 0 || row >= buf.LineCount() {
		return out
	}

	if line, ok := buf.EditedLine(row); ok {
		for i := 0; i+len(p.runes) <= len(line); i++ {
			if slices.Equal(line[i:i+len(p.runes)], p.runes) {
				out = append(out, i)
			}
		}

		return out
	}

	raw := buf.Raw(row)
	col := 0
	for at := 0; at+len(p.raw) <= len(raw); {
		i := bytes.Index(raw[at:], p.raw)
		if i < 0 {
			break
		}

		col += utf8.RuneCount(raw[at : at+i])
		out = append(out, col)

		_, size := utf8.DecodeRune(raw[at+i:])
		at, col = at+i+size, col+1
	}

	return out
}

// findMatch walks out from the cursor a line at a time and wraps around the end
// of the buffer the way VIM does, which is why the line the search started on
// is visited twice: once for what lies past the cursor, and once, after the
// wrap, for what lies before it.
func findMatch(p pattern, row, col int, back bool) (int, int, bool) {
	lines := buf.LineCount()
	var cols []int

	for step := 0; step <= lines; step++ {
		at := row + step
		if back {
			at = row - step
		}
		at = ((at % lines) + lines) % lines

		cols = p.matchesIn(at, cols[:0])
		if back {
			slices.Reverse(cols)
		}

		for _, c := range cols {
			if step == 0 && !beyond(c, col, back) {
				continue
			}
			if step == lines && beyond(c, col, back) {
				continue
			}

			return at, c, true
		}
	}

	return 0, 0, false
}

func beyond(c, col int, back bool) bool {
	if back {
		return c < col
	}

	return c > col
}

func startSearchForward()  { startPrompt('/') }
func startSearchBackward() { startPrompt('?') }

// commitSearch keeps the last pattern when nothing was typed, which is how VIM
// repeats a search from the prompt; the delimiter still says which way to go.
func commitSearch(input []rune, back bool) {
	if len(input) > 0 {
		searchPat = newPattern(input)
	}
	searchBack = back

	jumpToMatch(searchBack)
}

func nextMatch() { jumpToMatch(searchBack) }
func prevMatch() { jumpToMatch(!searchBack) }

func jumpToMatch(back bool) {
	if searchPat.empty() {
		return
	}
	hlSearch = true

	row, col, ok := findMatch(searchPat, currentRow, currentCol, back)
	if !ok {
		statusMsg = "Pattern not found: " + string(searchPat.runes)
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
	if !hlSearch || searchPat.empty() {
		return hitScan{}
	}
	hitCols = searchPat.matchesIn(row, hitCols[:0])

	return hitScan{cols: hitCols, width: len(searchPat.runes)}
}

func (h *hitScan) covers(col int) bool {
	for h.next < len(h.cols) && h.cols[h.next]+h.width <= col {
		h.next++
	}

	return h.next < len(h.cols) && col >= h.cols[h.next]
}
