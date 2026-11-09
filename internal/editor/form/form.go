// Package form is the box more than one line is typed into: labelled fields
// stacked up, each in a box of its own, with Tab moving between them and Enter
// settling on the lot of them. '<leader>fr' is what asks for two — a pattern,
// and what to put in its place — where the prompt's single line would have had
// to ask for them one after the other.
package form

import (
	"cmp"
	"slices"
	"strings"

	"github.com/nsf/termbox-go"

	"github.com/ArditZubaku/tex/internal/editor/screen"
	"github.com/ArditZubaku/tex/internal/layout"
	"github.com/ArditZubaku/tex/internal/theme"
)

// What a key did, which is all the caller has to act on: everything else the
// box handles itself. Typed is a field's own text having changed, which is
// what anything showing what it matches redraws from.
type Action int

const (
	Handled Action = iota
	Typed
	Closed
	Submitted
)

// A Field is one box of the stack: what it is asking for, and what has been
// typed into it so far.
type Field struct {
	Label  string
	input  []rune
	cursor int
}

// NewField is a box asking for Label, opened on text already typed — the word
// the cursor was on, for the pattern '<leader>fr' is about to replace.
func NewField(label, input string) Field {
	runes := []rune(input)

	return Field{Label: label, input: runes, cursor: len(runes)}
}

type Form struct {
	open   bool
	title  string
	hint   string
	fields []Field
	focus  int
}

func (f *Form) Open() bool { return f.open }

func (f *Form) Show(title, hint string, fields ...Field) {
	f.open, f.title, f.hint, f.fields, f.focus = true, title, hint, fields, 0
}

func (f *Form) Close() {
	f.open, f.fields = false, nil
}

// Focus is which box the keys are going into, by the order they were given in.
func (f *Form) Focus() int { return f.focus }

// Value is what has been typed into one of the boxes. The slice is the field's
// own, so anything keeping it past the next key has to copy it.
func (f *Form) Value(at int) []rune {
	if at < 0 || at >= len(f.fields) {
		return nil
	}

	return f.fields[at].input
}

// Moving between the boxes is Tab and the arrows, which are what a stack of
// them reads as; Ctrl-N and Ctrl-P come along for the ride, since they already
// move through the picker and the completion menu. Shift-Tab is not among them:
// termbox has no key for it, and it arrives as an escape sequence instead.
var moves = map[termbox.Key]int{
	termbox.KeyTab:       1,
	termbox.KeyArrowDown: 1,
	termbox.KeyCtrlN:     1,
	termbox.KeyArrowUp:   -1,
	termbox.KeyCtrlP:     -1,
}

func (f *Form) Key(event termbox.Event) Action {
	if !f.open || len(f.fields) == 0 {
		return Handled
	}

	if delta, ok := moves[event.Key]; ok && event.Ch == 0 {
		f.focus = (f.focus + delta + len(f.fields)) % len(f.fields)

		return Handled
	}

	switch event.Key {
	case termbox.KeyEsc:
		return Closed
	case termbox.KeyEnter:
		return Submitted
	}

	return f.fields[f.focus].key(event)
}

// A box emptied by backspace stays open, unlike the prompt's own line and the
// picker's query: there is a second box the key would otherwise take the whole
// thing down from, and clearing one to retype it is ordinary.
func (field *Field) key(event termbox.Event) Action {
	switch event.Key {
	case termbox.KeyBackspace, termbox.KeyBackspace2:
		if field.cursor == 0 {
			return Handled
		}
		field.input = slices.Delete(field.input, field.cursor-1, field.cursor)
		field.cursor--
	case termbox.KeyCtrlU:
		if field.cursor == 0 {
			return Handled
		}
		field.input = slices.Delete(field.input, 0, field.cursor)
		field.cursor = 0
	case termbox.KeyArrowLeft:
		field.cursor = max(field.cursor-1, 0)

		return Handled
	case termbox.KeyArrowRight:
		field.cursor = min(field.cursor+1, len(field.input))

		return Handled
	case termbox.KeyHome:
		field.cursor = 0

		return Handled
	case termbox.KeyEnd:
		field.cursor = len(field.input)

		return Handled
	case termbox.KeySpace:
		field.insert(' ')
	default:
		if event.Ch == 0 {
			return Handled
		}
		field.insert(event.Ch)
	}

	return Typed
}

func (field *Field) insert(r rune) {
	field.input = slices.Insert(field.input, field.cursor, r)
	field.cursor++
}

// The box is drawn at this at most, and shrinks to whatever the screen has.
const maxCols = 60

// A field takes a box of its own: its two edges and the line typed into it. The
// frame around them takes its own two, plus the row the hint goes on.
const (
	fieldRows  = 3
	chromeRows = 3
)

func (f *Form) Draw(within layout.Rect, palette *theme.Palette) {
	if !f.open {
		return
	}

	frame := f.frame(within)
	if frame.Cols < 8 || frame.Rows < chromeRows {
		return
	}
	f.drawFrame(frame, palette)

	for at := range f.fields {
		f.drawField(frame, at, palette)
	}

	screen.Print(frame.Col+2, frame.Row+frame.Rows-2, palette.Comment, palette.Background,
		screen.Pad(screen.Truncate(f.hint, boxCols(frame), false), boxCols(frame)))
}

func (f *Form) frame(within layout.Rect) layout.Rect {
	cols := min(maxCols, within.Cols-4)
	rows := min(len(f.fields)*fieldRows+chromeRows, within.Rows)

	return layout.Rect{
		Row:  within.Row + max(within.Rows-rows, 0)/2,
		Col:  within.Col + max(within.Cols-cols, 0)/2,
		Rows: rows,
		Cols: cols,
	}
}

// boxCols is the width one field's box has inside the frame, and textCols what
// is left inside that box for what is typed into it.
func boxCols(frame layout.Rect) int  { return frame.Cols - 4 }
func textCols(frame layout.Rect) int { return boxCols(frame) - 3 }

func boxRow(frame layout.Rect, at int) int { return frame.Row + 1 + at*fieldRows }

func (f *Form) drawFrame(frame layout.Rect, palette *theme.Palette) {
	row, col, cols := frame.Row, frame.Col, frame.Cols
	title := []rune(" " + cmp.Or(f.title, "Form") + " ")

	top := []rune("┌" + strings.Repeat("─", cols-2) + "┐")
	if len(title) < cols-3 {
		copy(top[2:], title)
	}

	screen.Print(col, row, palette.Separator, palette.Background, string(top))
	screen.Print(col, row+frame.Rows-1, palette.Separator, palette.Background,
		"└"+strings.Repeat("─", cols-2)+"┘")

	for i := 1; i < frame.Rows-1; i++ {
		screen.Print(col, row+i, palette.Separator, palette.Background, "│")
		screen.Print(col+1, row+i, palette.Plain, palette.Background, strings.Repeat(" ", cols-2))
		screen.Print(col+cols-1, row+i, palette.Separator, palette.Background, "│")
	}
}

// drawField is one box of the stack. The box being typed into is drawn in the
// colour the buffer line marks the active buffer with, so which of them the
// keys are going into is read off the screen rather than remembered.
func (f *Form) drawField(frame layout.Rect, at int, palette *theme.Palette) {
	field, cols := &f.fields[at], boxCols(frame)
	col, row := frame.Col+2, boxRow(frame, at)
	if row+fieldRows > frame.Row+frame.Rows-1 || cols < 4 {
		return
	}

	border := palette.Separator
	if at == f.focus {
		border = palette.TabActiveFg
	}

	label := []rune(" " + field.Label + " ")
	top := []rune("┌" + strings.Repeat("─", cols-2) + "┐")
	if len(label) < cols-3 {
		copy(top[2:], label)
	}

	screen.Print(col, row, border, palette.Background, string(top))
	screen.Print(col, row+1, border, palette.Background, "│")
	screen.Print(col+cols-1, row+1, border, palette.Background, "│")
	screen.Print(col, row+2, border, palette.Background,
		"└"+strings.Repeat("─", cols-2)+"┘")

	shown, _ := field.view(textCols(frame))
	screen.Print(col+1, row+1, palette.Plain, palette.Background,
		screen.Pad(" "+shown, cols-2))
}

// view is as much of the field as its box has room for, together with how far
// into it the cursor sits. The window ends at the cursor, so a pattern longer
// than the box scrolls under it rather than running off the end.
func (field *Field) view(width int) (string, int) {
	if width <= 0 {
		return "", 0
	}

	from := 0
	for from < field.cursor && screen.Width(string(field.input[from:field.cursor])) >= width {
		from++
	}

	shown := field.input[from:]
	for len(shown) > 0 && screen.Width(string(shown)) > width {
		shown = shown[:len(shown)-1]
	}

	return string(shown), screen.Width(string(field.input[from:field.cursor]))
}

// CursorRow and CursorCol are where the terminal's own cursor goes: inside the
// box being typed into, however far along it what is typed reaches.
func (f *Form) CursorRow(within layout.Rect) int {
	return boxRow(f.frame(within), f.focus) + 1
}

func (f *Form) CursorCol(within layout.Rect) int {
	frame := f.frame(within)
	if f.focus >= len(f.fields) {
		return frame.Col
	}
	_, before := f.fields[f.focus].view(textCols(frame))

	return frame.Col + 4 + before
}
