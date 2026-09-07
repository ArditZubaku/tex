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

// Key is the next event off the terminal, and whether it was a key at all.
// What comes off it that is not one — a resize, or the interrupt something off
// the loop's goroutine asks for a frame with — is a frame owed and nothing
// more: handing it to the dispatcher as a key clears a chord half typed, drops
// a count, and wipes what the last command reported.
func Key() (termbox.Event, bool) {
	event := termbox.PollEvent()
	if event.Type == termbox.EventError {
		panic(event.Err) // TODO: Will think of something better in such a case
	}

	return event, event.Type == termbox.EventKey
}

// StartWaker is how something off the loop's goroutine asks for a frame: it
// returns the function to call, which never blocks and never draws anything
// itself.
//
// termbox's own interrupt channel is unbuffered, so Interrupt blocks until a
// PollEvent takes it — which is why exactly one goroutine ever waits there, and
// why what asks for a frame goes through a channel of one instead. A burst of
// answers arriving is a single wake-up, and the loop draws once for all of them.
//
// It is returned rather than installed because the editor's own loop is the
// only thing that may call it: a test drives the dispatcher directly, never
// initialises termbox, and must therefore never have a waker at all.
func StartWaker() func() {
	asked := make(chan struct{}, 1)

	go func() {
		for range asked {
			termbox.Interrupt()
		}
	}()

	return func() {
		select {
		case asked <- struct{}{}:
		default:
		}
	}
}
