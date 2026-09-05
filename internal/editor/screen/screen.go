// Package screen is the terminal surface itself: putting text on it, and taking
// the next key off it.
package screen

import (
	"strings"

	"github.com/mattn/go-runewidth"
	"github.com/nsf/termbox-go"
)

func Print(col, row int, fg, bg termbox.Attribute, msg string) {
	for _, ch := range msg {
		termbox.SetCell(col, row, ch, fg, bg)
		col += runewidth.RuneWidth(ch)
	}
}

// Pad runs text out to a width so that what it is drawn on takes the whole
// width in its own colours, rather than however far the text happens to reach.
func Pad(txt string, width int) string {
	return txt + strings.Repeat(" ", max(width-runewidth.StringWidth(txt), 0))
}

// A path is cut at the front, since the name at its end is what is being looked
// for; a line of text is cut at its end, where a reference has already said
// which file and which row it is on.
func Truncate(txt string, width int, fromFront bool) string {
	runes := []rune(txt)
	if len(runes) <= width {
		return txt
	}
	if fromFront {
		return "…" + string(runes[len(runes)-width+1:])
	}

	return string(runes[:width-1]) + "…"
}

func Key() termbox.Event {
	var keyEvent termbox.Event

	switch event := termbox.PollEvent(); event.Type {
	case termbox.EventKey:
		keyEvent = event
	case termbox.EventError:
		panic(event.Err) // TODO: Will think of something better in such a case
	}

	return keyEvent
}
