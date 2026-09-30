package screen

import (
	"strings"
	"time"

	"github.com/nsf/termbox-go"
)

// A bracketed paste is marked out by the terminal wrapping it in these two
// sequences; termbox has no table entry for either, so left to itself it
// reports the opening one as a bare Esc followed by five ordinary-looking
// characters. escPasteWait is how long Next lingers after such an Esc to let
// the rest of the marker, if it's coming, catch up — long enough that a
// marker's own bytes, already queued behind it, are never missed, short
// enough that a real Esc key is never felt to lag over it. Once the opening
// marker is confirmed, pasteCaptureBackstop replaces it as a much longer
// backstop against a paste whose closing marker never arrives, rather than
// blocking the editor on it forever.
const (
	pasteOpen  = "\x1b[200~"
	pasteClose = "\x1b[201~"

	escPasteWait         = 25 * time.Millisecond
	pasteCaptureBackstop = 2 * time.Second
)

// pollEvent and interrupt are termbox.PollEvent and termbox.Interrupt, swapped
// out in tests for a fake source that never touches a real terminal.
var (
	pollEvent = termbox.PollEvent
	interrupt = termbox.Interrupt
)

// pending is what a marker match gave up partway through — events already
// taken off termbox that turned out not to belong to one — waiting to be
// handed back through Next in the order they arrived.
var pending []termbox.Event

// Next is Key, but for the one thing termbox cannot tell apart on its own:
// a bracketed paste's opening marker arrives as a bare Esc followed by
// ordinary-looking characters, indistinguishable at the first byte from a
// real Esc keypress that happens to be followed by the same few keys.
// Between a confirmed opening marker and its closing one, everything read
// off termbox is taken as the pasted text itself rather than as key presses.
func Next() (event termbox.Event, isKey bool, pasted string, isPaste bool) {
	if len(pending) > 0 {
		event, pending = pending[0], pending[1:]
		return event, event.Type == termbox.EventKey, "", false
	}

	event = poll()
	if event.Type != termbox.EventKey || event.Key != termbox.KeyEsc || event.Ch != 0 {
		return event, event.Type == termbox.EventKey, "", false
	}

	if ok, consumed := tryMarker(pasteOpen, event, escPasteWait); !ok {
		event, pending = consumed[0], consumed[1:]
		return event, true, "", false
	}

	return termbox.Event{}, false, capture(), true
}

func poll() termbox.Event {
	event := pollEvent()
	if event.Type == termbox.EventError {
		panic(event.Err) // TODO: Will think of something better in such a case
	}

	return event
}

// pollTimeout is poll with a bound: it reports back false, having disturbed
// nothing observable, if nothing arrives within d.
func pollTimeout(d time.Duration) (termbox.Event, bool) {
	timer := time.AfterFunc(d, interrupt)
	event := poll()
	timer.Stop()

	if event.Type == termbox.EventInterrupt {
		return termbox.Event{}, false
	}

	return event, true
}

// tryMarker is marker's tail — everything after the Esc byte esc already is —
// matched one event at a time, each given up to d to arrive. Failing partway
// through hands back every event it took off termbox, esc included, so the
// caller can treat them as what they actually were.
func tryMarker(marker string, esc termbox.Event, d time.Duration) (bool, []termbox.Event) {
	consumed := []termbox.Event{esc}
	for _, want := range marker[1:] {
		event, ok := pollTimeout(d)
		if !ok {
			return false, consumed
		}
		consumed = append(consumed, event)
		if event.Type != termbox.EventKey || event.Ch != want {
			return false, consumed
		}
	}

	return true, nil
}

// capture reads a confirmed paste through to its closing marker, translating
// every event in between into the text it stands for.
func capture() string {
	var text strings.Builder
	sawCR := false

	for {
		event, ok := pollTimeout(pasteCaptureBackstop)
		if !ok {
			return text.String() // a close marker that never came; keep what's in hand rather than hang on it
		}
		if event.Type != termbox.EventKey {
			continue // a resize or a mouse move mid-paste is not part of it
		}

		if event.Key == termbox.KeyEsc && event.Ch == 0 {
			closed, consumed := tryMarker(pasteClose, event, pasteCaptureBackstop)
			if closed {
				return text.String()
			}
			for _, e := range consumed {
				writeEvent(&text, e, &sawCR)
			}
			continue
		}

		writeEvent(&text, event, &sawCR)
	}
}

// writeEvent is one captured event as the byte a paste actually carries: a
// rune as itself, Enter and the bare LF a Unix line ending pastes as
// collapsed to one '\n' the same way a typed CRLF already is, and anything
// else termbox gives a literal byte value for — Tab, Esc, a raw control
// character, which styled terminal output copied out can genuinely contain —
// as that byte.
func writeEvent(text *strings.Builder, event termbox.Event, sawCR *bool) {
	wasCR := *sawCR
	*sawCR = event.Ch == 0 && event.Key == termbox.KeyEnter

	if wasCR && event.Ch == 0 && event.Key == termbox.KeyCtrlJ {
		return
	}

	switch {
	case event.Ch != 0:
		text.WriteRune(event.Ch)
	case event.Key == termbox.KeyEnter, event.Key == termbox.KeyCtrlJ:
		text.WriteByte('\n')
	case event.Key == termbox.KeySpace:
		text.WriteByte(' ')
	default:
		text.WriteRune(rune(event.Key))
	}
}
