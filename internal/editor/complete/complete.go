// Package complete is the list of candidates under the word being typed: what a
// language server offers for it, narrowed to what has been typed of it so far,
// with one of them settled on by Tab. It is the menu and nothing else — where
// the candidates came from and what putting one in costs the buffer are both
// somebody else's, which is what lets a second source of them arrive without
// touching any of this.
package complete

import (
	"encoding/json"
	"slices"
	"strings"

	"github.com/ArditZubaku/tex/internal/editor/screen"
	"github.com/ArditZubaku/tex/internal/fuzzy"
	"github.com/ArditZubaku/tex/internal/layout"
	"github.com/ArditZubaku/tex/internal/theme"
)

// The menu is drawn at these at most and shrinks to whatever the screen has,
// with the listing scrolling under the selection. Under minCols there is no
// room to draw a candidate at all; under minDetail there is nothing left of a
// signature worth the columns it costs, so the detail goes and the name stays.
const (
	maxCols   = 62
	maxRows   = 10
	minCols   = 12
	minDetail = 6
)

// An Item is one candidate. Text is what goes into the file and From the rune
// column on the row it starts replacing at, which is the server's own idea of
// what the typing so far covered rather than this package's.
//
// Call is a candidate that is invoked rather than named — a function, a method,
// a constructor — which is written in with the parentheses it needs. It is said
// here as what it means for the text rather than as the kind it came from, so
// that the menu stays ignorant of whose kinds those are.
//
// Ask is a candidate whose edits elsewhere are not here yet, because the server
// held them back until it knew which candidate was being settled on. Data is
// the bookmark it will want quoting back to find them again.
type Item struct {
	Label  string
	Detail string
	Text   string
	From   int
	Extra  []Edit
	Call   bool
	Ask    bool
	Data   json.RawMessage
}

// An Edit is a stretch of the file replaced by some text, in the editor's own
// coordinates, with the end exclusive. What completion carries them for is the
// import line a name needs adding before it will compile.
type Edit struct {
	Row, Col       int
	EndRow, EndCol int
	Text           string
}

type match struct {
	at    int
	score int
}

// A Menu is the candidates and what has been typed against them. The slices are
// kept between openings rather than freed: a menu is refiltered on every
// keystroke and reopened on every word, and neither should reach the allocator.
type Menu struct {
	open    bool
	items   []Item
	matches []match
	query   []rune
	sel     int
	offset  int

	// Where the word being completed is, which is what says whether an answer
	// still belongs to it and where the menu is drawn.
	row   int
	start int

	// incomplete is the server saying it answered from the prefix it was given
	// rather than exhaustively, and so that a longer one is worth asking again.
	incomplete bool
}

func (m *Menu) Open() bool { return m.open }

func (m *Menu) Row() int   { return m.row }
func (m *Menu) Start() int { return m.start }

func (m *Menu) Incomplete() bool { return m.incomplete }

// Show puts candidates up for the word starting at start on row, narrowed to
// query — what has been typed of it since the answer was asked for. A menu with
// nothing left after that is no menu at all, which is what the result says.
func (m *Menu) Show(items []Item, row, start int, query []rune, incomplete bool) bool {
	m.items = append(m.items[:0], items...)
	m.row, m.start, m.incomplete = row, start, incomplete
	m.open = true
	m.filter(query)

	if len(m.matches) == 0 {
		m.Close()
	}

	return m.open
}

// Retype is the query having changed under a menu already up: another rune
// typed, or one taken back. It narrows what is there rather than asking again,
// which is what keeps a keystroke from being a request.
func (m *Menu) Retype(query []rune) bool {
	if !m.open {
		return false
	}

	m.filter(query)
	if len(m.matches) == 0 {
		m.Close()
	}

	return m.open
}

// Query is what has been typed of the word since the candidates came, which is
// what decides whether asking again would say anything new.
func (m *Menu) Query() []rune { return m.query }

func (m *Menu) Close() {
	m.open, m.incomplete = false, false
	m.items, m.matches = m.items[:0], m.matches[:0]
	m.sel, m.offset = 0, 0
}

func (m *Menu) Selected() (Item, bool) {
	if !m.open || len(m.matches) == 0 {
		return Item{}, false
	}

	return m.items[m.matches[m.sel].at], true
}

// Count is how many candidates the query left, which is what a test reads the
// menu by.
func (m *Menu) Count() int { return len(m.matches) }

// Labels are the candidates as they are listed, for the same reason.
func (m *Menu) Labels() []string {
	out := make([]string, 0, len(m.matches))
	for _, one := range m.matches {
		out = append(out, m.items[one.at].Label)
	}

	return out
}

// Next and Prev wrap, the way VIM's own completion menu does: the list is short
// and the way back to the top is one more press either way.
func (m *Menu) Next() { m.moveTo(m.sel + 1) }
func (m *Menu) Prev() { m.moveTo(m.sel - 1) }

func (m *Menu) moveTo(sel int) {
	if len(m.matches) == 0 {
		return
	}

	m.sel = (sel + len(m.matches)) % len(m.matches)
	m.scroll()
}

func (m *Menu) scroll() {
	rows := min(len(m.matches), maxRows)
	switch {
	case m.sel < m.offset:
		m.offset = m.sel
	case m.sel >= m.offset+rows:
		m.offset = m.sel - rows + 1
	}
	m.offset = max(min(m.offset, len(m.matches)-rows), 0)
}

// The server's own ranking is what the candidates arrive in, and it stands
// until something is typed against it: gopls puts the field of the receiver
// above the package of the same first letter, which no local score can know.
func (m *Menu) filter(query []rune) {
	m.query = append(m.query[:0], query...)
	m.matches = m.matches[:0]

	for at, item := range m.items {
		if score, ok := fuzzy.Score(item.Label, m.query); ok {
			m.matches = append(m.matches, match{at: at, score: score})
		}
	}

	if len(m.query) > 0 {
		slices.SortStableFunc(m.matches, func(a, b match) int { return b.score - a.score })
	}

	m.sel, m.offset = 0, 0
}

// Draw puts the menu under the word being typed, or above it where there is no
// room below. cursorCol is the cell the cursor sits in: the menu is drawn from
// the start of the word rather than from there, so the candidates line up under
// what they would replace.
func (m *Menu) Draw(within layout.Rect, cursorRow, cursorCol int, palette *theme.Palette) {
	if !m.open || len(m.matches) == 0 {
		return
	}

	frame := m.frame(within, cursorRow, cursorCol)
	if frame.Cols < minCols {
		return
	}

	for i := range frame.Rows {
		at := m.offset + i
		fg, bg := palette.Plain, palette.StatusBg
		if at == m.sel {
			fg, bg = palette.MatchFg, palette.MatchBg
		}

		screen.Print(frame.Col, frame.Row+i, fg, bg, m.rowText(m.items[m.matches[at].at], frame.Cols))
	}
}

// A candidate is drawn against the width it has, with the detail that explains
// it — a signature, a type — pushed to the right and cut first: the name is
// what is being chosen between, and the detail only tells two of them apart.
func (m *Menu) rowText(item Item, cols int) string {
	label := screen.Truncate(item.Label, cols-2, false)
	room := cols - 3 - screen.Width(label)
	if item.Detail == "" || room < minDetail {
		return screen.Pad(" "+label, cols)
	}

	detail := screen.Truncate(item.Detail, room, false)
	gap := max(cols-2-screen.Width(label)-screen.Width(detail), 1)

	return screen.Pad(" "+label+strings.Repeat(" ", gap)+detail, cols)
}

func (m *Menu) frame(within layout.Rect, cursorRow, cursorCol int) layout.Rect {
	widest := 0
	for _, one := range m.matches {
		item := m.items[one.at]
		width := screen.Width(item.Label) + 2
		if item.Detail != "" {
			width += screen.Width(item.Detail) + 1
		}
		widest = max(widest, width)
	}

	rows := min(len(m.matches), maxRows)
	cols := min(min(widest, maxCols), within.Cols)
	row := cursorRow + 1
	if row+rows > within.Row+within.Rows {
		row = max(cursorRow-rows, within.Row)
	}

	return layout.Rect{
		Row:  row,
		Col:  max(min(cursorCol-len(m.query), within.Col+within.Cols-cols), within.Col),
		Rows: min(rows, within.Row+within.Rows-row),
		Cols: cols,
	}
}
