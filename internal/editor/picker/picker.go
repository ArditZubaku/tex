// Package picker is the popup over whatever is on screen: a list of places to
// go, narrowed to what is typed, with Enter settling on one. The files under
// the project root are one such list ('<leader><leader>'), the references to an
// identifier another ('gr').
package picker

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/mattn/go-runewidth"
	"github.com/nsf/termbox-go"

	"github.com/ArditZubaku/tex/internal/editor/screen"
	"github.com/ArditZubaku/tex/internal/fuzzy"
	"github.com/ArditZubaku/tex/internal/layout"
	"github.com/ArditZubaku/tex/internal/theme"
)

// An Entry is a row of the popup: what is drawn and matched, and where Enter
// goes. A negative row means the file itself, wherever the cursor was last left
// in it.
type Entry struct {
	Label string
	Path  string
	Row   int
	Col   int
}

type match struct {
	at    int
	score int
}

type Picker struct {
	open    bool
	title   string
	entries []Entry
	matches []match
	query   []rune
	sel     int
	offset  int
}

// What a key did, which is all the caller has to act on: everything else the
// popup handles itself.
type Action int

const (
	Handled Action = iota
	Closed
	Chosen
)

// The popup is drawn at these at most, and shrinks to whatever the screen has.
const (
	maxCols = 80
	maxRows = 15
)

// The frame takes two rows for its own edges, one for the query and one for the
// rule under it, which is what the listing has to fit around.
const chromeRows = 4

func (p *Picker) Open() bool { return p.open }

// Show on a popup that is already up keeps what has been typed into it, so that
// a listing refreshed under the query — a server asked again as it is typed —
// does not throw the typing away.
func (p *Picker) Show(title string, entries []Entry) {
	if !p.open {
		p.query = p.query[:0]
	}
	p.open, p.title, p.entries = true, title, entries
	p.filter()
}

func (p *Picker) Close() {
	p.open = false
	p.entries, p.matches = nil, nil
}

// Matched is what the query left, in the order the popup lists it.
func (p *Picker) Matched() []Entry {
	left := make([]Entry, 0, len(p.matches))
	for _, one := range p.matches {
		left = append(left, p.entries[one.at])
	}

	return left
}

func (p *Picker) Selected() (Entry, bool) {
	if len(p.matches) == 0 {
		return Entry{}, false
	}

	return p.entries[p.matches[p.sel].at], true
}

func (p *Picker) filter() {
	p.matches = p.matches[:0]
	for at, entry := range p.entries {
		if score, ok := fuzzy.Score(entry.Label, p.query); ok {
			p.matches = append(p.matches, match{at: at, score: score})
		}
	}

	// a list already in the order it means something in — the references down a
	// file — keeps it until something is typed
	if len(p.query) > 0 {
		slices.SortStableFunc(p.matches, func(a, b match) int {
			if a.score != b.score {
				return b.score - a.score
			}

			return cmp.Compare(p.entries[a.at].Label, p.entries[b.at].Label)
		})
	}
	p.sel, p.offset = 0, 0
}

func (p *Picker) move(delta int) {
	p.sel = max(min(p.sel+delta, len(p.matches)-1), 0)
}

var moves = map[termbox.Key]int{
	termbox.KeyArrowDown: 1,
	termbox.KeyArrowUp:   -1,
	termbox.KeyCtrlN:     1,
	termbox.KeyCtrlP:     -1,
	termbox.KeyCtrlJ:     1,
	termbox.KeyCtrlK:     -1,
}

func (p *Picker) Key(event termbox.Event) Action {
	if delta, ok := moves[event.Key]; ok {
		p.move(delta)
		return Handled
	}

	switch {
	case event.Key == termbox.KeyEsc:
		return Closed
	case event.Key == termbox.KeyEnter:
		return Chosen
	case event.Key == termbox.KeyBackspace, event.Key == termbox.KeyBackspace2:
		// backspacing off an empty query leaves the popup, as it does the prompt
		if len(p.query) == 0 {
			return Closed
		}
		p.query = p.query[:len(p.query)-1]
	case event.Key == termbox.KeyCtrlU:
		p.query = p.query[:0]
	case event.Key == termbox.KeySpace:
		p.query = append(p.query, ' ')
	case event.Ch != 0:
		p.query = append(p.query, event.Ch)
	default:
		return Handled
	}
	p.filter()

	return Handled
}

// The popup is drawn over the windows rather than inside one: it belongs to the
// screen the way the status line does, so it stays where it is however the
// screen is split.
func (p *Picker) Draw(within layout.Rect, palette *theme.Palette) {
	if !p.open {
		return
	}

	frame := p.frame(within)
	listRows := frame.Rows - chromeRows

	p.scroll(listRows)
	p.drawFrame(frame, palette)

	screen.Print(frame.Col+1, frame.Row+1, palette.Plain, palette.Background,
		screen.Pad(" > "+string(p.query), frame.Cols-2))

	for i := range listRows {
		at := p.offset + i
		if at >= len(p.matches) {
			break
		}

		background := palette.Background
		if at == p.sel {
			background = palette.VisualBg
		}
		entry := p.entries[p.matches[at].at]
		screen.Print(frame.Col+1, frame.Row+3+i, palette.Plain, background,
			screen.Pad(" "+screen.Truncate(entry.Label, frame.Cols-3, entry.Row < 0), frame.Cols-2))
	}
}

// CursorRow and CursorCol are where the terminal's own cursor sits while the
// popup is up: on the query line, after what has been typed.
func (p *Picker) CursorRow(within layout.Rect) int {
	return p.frame(within).Row + 1
}

func (p *Picker) CursorCol(within layout.Rect) int {
	return p.frame(within).Col + 4 + runewidth.StringWidth(string(p.query))
}

func (p *Picker) frame(within layout.Rect) layout.Rect {
	cols := min(maxCols, within.Cols-4)
	rows := min(maxRows, within.Rows-2)

	return layout.Rect{
		Row:  within.Row + (within.Rows-rows)/2,
		Col:  within.Col + (within.Cols-cols)/2,
		Rows: rows,
		Cols: cols,
	}
}

func (p *Picker) scroll(listRows int) {
	p.sel = min(p.sel, max(len(p.matches)-1, 0))
	p.offset = min(p.offset, p.sel)
	if p.sel >= p.offset+listRows {
		p.offset = p.sel - listRows + 1
	}
}

func (p *Picker) drawFrame(frame layout.Rect, palette *theme.Palette) {
	row, col, rows, cols := frame.Row, frame.Col, frame.Rows, frame.Cols
	title := " " + cmp.Or(p.title, "Files") + " "

	top := []rune("┌" + strings.Repeat("─", cols-2) + "┐")
	copy(top[2:], []rune(title))
	count := fmt.Sprintf(" %d/%d ", len(p.matches), len(p.entries))
	copy(top[cols-2-len([]rune(count)):], []rune(count))

	screen.Print(col, row, palette.Separator, palette.Background, string(top))
	screen.Print(col, row+2, palette.Separator, palette.Background,
		"├"+strings.Repeat("─", cols-2)+"┤")
	screen.Print(col, row+rows-1, palette.Separator, palette.Background,
		"└"+strings.Repeat("─", cols-2)+"┘")

	for i := 1; i < rows-1; i++ {
		if i == 2 {
			continue
		}
		screen.Print(col, row+i, palette.Separator, palette.Background, "│")
		screen.Print(col+1, row+i, palette.Plain, palette.Background, strings.Repeat(" ", cols-2))
		screen.Print(col+cols-1, row+i, palette.Separator, palette.Background, "│")
	}
}
