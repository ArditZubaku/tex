package screen

import (
	"testing"

	"github.com/nsf/termbox-go"
)

// fakeEvents replaces pollEvent and interrupt with a channel-backed pair that
// never touches a real terminal: interrupt pushes the same EventInterrupt a
// real termbox.Interrupt eventually surfaces through PollEvent, onto the very
// channel Next is reading, exactly as the real pair does.
func fakeEvents(t *testing.T, events ...termbox.Event) {
	t.Helper()

	ch := make(chan termbox.Event, len(events)+4)
	for _, e := range events {
		ch <- e
	}

	pollEvent = func() termbox.Event { return <-ch }
	interrupt = func() { ch <- termbox.Event{Type: termbox.EventInterrupt} }

	t.Cleanup(func() {
		pollEvent, interrupt = termbox.PollEvent, termbox.Interrupt
		pending = nil
	})
}

func key(ch rune) termbox.Event { return termbox.Event{Type: termbox.EventKey, Ch: ch} }

func specialKey(k termbox.Key) termbox.Event { return termbox.Event{Type: termbox.EventKey, Key: k} }

func openMarker() []termbox.Event {
	return []termbox.Event{specialKey(termbox.KeyEsc), key('['), key('2'), key('0'), key('0'), key('~')}
}

func closeMarker() []termbox.Event {
	return []termbox.Event{specialKey(termbox.KeyEsc), key('['), key('2'), key('0'), key('1'), key('~')}
}

func TestNextPassesThroughAnOrdinaryKeyUntouched(t *testing.T) {
	fakeEvents(t, key('x'))

	event, isKey, _, isPaste := Next()

	if isPaste || !isKey || event.Ch != 'x' {
		t.Errorf("event = %+v, isKey = %v, isPaste = %v, want the plain key back", event, isKey, isPaste)
	}
}

// With nothing behind it, a bare Esc has to fall back to being exactly that
// — the whole reason Next only ever makes it wait, never hides it entirely.
func TestNextFallsBackToAPlainEscWhenNothingFollows(t *testing.T) {
	fakeEvents(t, specialKey(termbox.KeyEsc))

	event, isKey, _, isPaste := Next()

	if isPaste {
		t.Fatal("read as a paste, want a plain Esc")
	}
	if !isKey || event.Key != termbox.KeyEsc {
		t.Errorf("event = %+v, isKey = %v, want a lone KeyEsc", event, isKey)
	}
}

func TestNextCapturesAMultiLinePasteBetweenTheMarkers(t *testing.T) {
	events := openMarker()
	events = append(events, key('h'), key('i'), specialKey(termbox.KeyEnter), key('x'))
	events = append(events, closeMarker()...)
	fakeEvents(t, events...)

	_, isKey, pasted, isPaste := Next()

	if !isPaste || isKey {
		t.Fatalf("isPaste = %v, isKey = %v, want a bare paste result", isPaste, isKey)
	}
	if want := "hi\nx"; pasted != want {
		t.Errorf("pasted = %q, want %q", pasted, want)
	}
}

// An Esc that only looks like the start of the open marker has to give back
// every byte it took a peek at, in order, rather than eat a real keypress.
func TestNextRollsBackAnEscThatIsNotAPasteMarker(t *testing.T) {
	fakeEvents(t, specialKey(termbox.KeyEsc), key('['), key('a'))

	event, isKey, _, isPaste := Next()
	if isPaste || !isKey || event.Key != termbox.KeyEsc {
		t.Fatalf("1st = %+v, isKey = %v, isPaste = %v, want the Esc back", event, isKey, isPaste)
	}

	event, isKey, _, isPaste = Next()
	if isPaste || !isKey || event.Ch != '[' {
		t.Fatalf("2nd = %+v, isKey = %v, isPaste = %v, want '['", event, isKey, isPaste)
	}

	event, isKey, _, isPaste = Next()
	if isPaste || !isKey || event.Ch != 'a' {
		t.Fatalf("3rd = %+v, isKey = %v, isPaste = %v, want 'a'", event, isKey, isPaste)
	}
}

// A paste of styled terminal output can carry a raw Esc of its own; only the
// one that actually completes the close marker may end the capture.
func TestNextKeepsARawEscapeInsideAPasteThatIsNotTheCloseMarker(t *testing.T) {
	events := openMarker()
	events = append(events, specialKey(termbox.KeyEsc), key('3'), key('1'), key('m'), key('!'))
	events = append(events, closeMarker()...)
	fakeEvents(t, events...)

	_, _, pasted, isPaste := Next()

	if !isPaste {
		t.Fatal("want a paste")
	}
	if want := "\x1b31m!"; pasted != want {
		t.Errorf("pasted = %q, want %q", pasted, want)
	}
}
