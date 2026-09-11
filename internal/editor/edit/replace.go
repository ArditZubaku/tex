package edit

import (
	"slices"

	"github.com/nsf/termbox-go"

	"github.com/ArditZubaku/tex/internal/editor/state"
)

// StartReplace is 'r': the next key typed goes in place of the rune under the
// cursor, and of the count-1 runes after it. It is a state of its own rather
// than a chord because VIM waits for that key however long it takes, and a
// chord here times out.
func StartReplace(e *state.Editor) {
	e.PendingReplace = e.Count()
}

// Replace is the key 'r' was waiting for. Anything with no rune to put in the
// file — Esc, an arrow, Enter — cancels, which is VIM's own 'r': it takes one
// character and gives the keyboard back either way.
func Replace(e *state.Editor, event termbox.Event) {
	count := e.PendingReplace
	e.PendingReplace = 0

	ch, ok := replacement(event)
	if !ok {
		return
	}

	line := e.Buf.Line(e.Row)
	// A count reaching past the end of the line replaces nothing at all rather
	// than as much as it can, which is what VIM does with one.
	if count > len(line)-e.Col {
		return
	}

	put := slices.Clone(line)
	for i := range count {
		put[e.Col+i] = ch
	}

	e.BeginChange()
	e.TouchLine(e.Row)
	e.Buf.SetLine(e.Row, put)
	e.Col += count - 1
	e.Modified = true
	e.EndChange()
}

// Space and Tab arrive as keys of their own rather than as runes, and a Tab is
// the space this editor types everywhere else.
func replacement(event termbox.Event) (rune, bool) {
	switch {
	case event.Ch != 0:
		return event.Ch, true
	case event.Key == termbox.KeySpace, event.Key == termbox.KeyTab:
		return ' ', true
	}

	return 0, false
}
