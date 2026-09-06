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
		// printable ASCII is one column wide, which is most of what the editor
		// draws and all of what the gutter does; the table lookup is for the rest
		if ch >= ' ' && ch < 0x7f {
			col++
			continue
		}
		col += runewidth.RuneWidth(ch)
	}
}

// Fill paints a run of cells in one colour, which is what a band under a row of
// text is: characters drawn over it afterwards keep the colours painted here.
func Fill(col, row, width int, fg, bg termbox.Attribute) {
	for i := range width {
		termbox.SetCell(col+i, row, ' ', fg, bg)
	}
}

// Pad runs text out to a width so that what it is drawn on takes the whole
// width in its own colours, rather than however far the text happens to reach.
func Pad(txt string, width int) string {
	spaces := max(width-Width(txt), 0)
	if spaces == 0 {
		return txt
	}

	var out strings.Builder
	out.Grow(len(txt) + spaces)
	out.WriteString(txt)
	for range spaces {
		out.WriteByte(' ')
	}

	return out.String()
}

// Width is how many columns txt takes. It is runewidth.StringWidth without the
// []rune it allocates on every call, and with the same ASCII shortcut Print has.
func Width(txt string) int {
	width := 0
	for _, ch := range txt {
		if ch >= ' ' && ch < 0x7f {
			width++
			continue
		}
		width += runewidth.RuneWidth(ch)
	}

	return width
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
