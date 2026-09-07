package picker

import (
	"testing"

	"github.com/nsf/termbox-go"
)

func show(t *testing.T, labels ...string) *Picker {
	t.Helper()

	entries := make([]Entry, 0, len(labels))
	for _, label := range labels {
		entries = append(entries, Entry{Label: label, Path: label, Row: -1})
	}

	var p Picker
	p.Show("files", entries)

	return &p
}

func typeQuery(t *testing.T, p *Picker, query string) {
	t.Helper()

	for _, ch := range query {
		if action := p.Key(termbox.Event{Ch: ch}); action != Handled {
			t.Fatalf("typing %q = %v, want Handled", ch, action)
		}
	}
}

func press(t *testing.T, p *Picker, key termbox.Key, times int) {
	t.Helper()

	for range times {
		if action := p.Key(termbox.Event{Key: key}); action != Handled {
			t.Fatalf("key %v = %v, want Handled", key, action)
		}
	}
}

// The listing is in the scorer's order rather than the one it was given, so
// what a move should land on is read off the listing itself.
func row(t *testing.T, p *Picker, at int) string {
	t.Helper()

	matched := p.Matched()
	if at >= len(matched) {
		t.Fatalf("row %d of %d matched", at, len(matched))
	}

	return matched[at].Label
}

func selected(t *testing.T, p *Picker) string {
	t.Helper()

	entry, ok := p.Selected()
	if !ok {
		t.Fatal("nothing selected")
	}

	return entry.Label
}

func TestArrowsMoveThroughWhatTheQueryLeft(t *testing.T) {
	p := show(t, "buffer_test.go", "picker_test.go", "theme_test.go", "main.go")
	typeQuery(t, p, "test")

	if got := len(p.Matched()); got != 3 {
		t.Fatalf("matched %d entries, want 3", got)
	}

	for at := 1; at < 3; at++ {
		press(t, p, termbox.KeyArrowDown, 1)
		if got, want := selected(t, p), row(t, p, at); got != want {
			t.Fatalf("selected %q, want %q", got, want)
		}
	}

	press(t, p, termbox.KeyArrowUp, 1)
	if got, want := selected(t, p), row(t, p, 1); got != want {
		t.Fatalf("selected %q after ArrowUp, want %q", got, want)
	}
}

func TestCtrlNAndCtrlPMoveTheSelectionTheWayTheArrowsDo(t *testing.T) {
	p := show(t, "one_test.go", "two_test.go", "three_test.go")
	typeQuery(t, p, "test")

	press(t, p, termbox.KeyCtrlN, 2)
	if got, want := selected(t, p), row(t, p, 2); got != want {
		t.Fatalf("selected %q, want %q", got, want)
	}

	press(t, p, termbox.KeyCtrlP, 1)
	if got, want := selected(t, p), row(t, p, 1); got != want {
		t.Fatalf("selected %q, want %q", got, want)
	}
}

func TestTheSelectionStopsAtEitherEndOfTheListing(t *testing.T) {
	p := show(t, "one_test.go", "two_test.go")
	typeQuery(t, p, "test")

	press(t, p, termbox.KeyArrowUp, 5)
	if got, want := selected(t, p), row(t, p, 0); got != want {
		t.Fatalf("selected %q at the top, want %q", got, want)
	}

	press(t, p, termbox.KeyArrowDown, 5)
	if got, want := selected(t, p), row(t, p, 1); got != want {
		t.Fatalf("selected %q at the bottom, want %q", got, want)
	}
}

func TestNarrowingTheListingAgainSelectsItsFirstRow(t *testing.T) {
	p := show(t, "one_test.go", "two_test.go", "three_test.go")
	typeQuery(t, p, "test")
	press(t, p, termbox.KeyArrowDown, 2)

	typeQuery(t, p, "o")
	press(t, p, termbox.KeyBackspace, 1)

	if got, want := selected(t, p), row(t, p, 0); got != want {
		t.Fatalf("selected %q, want %q", got, want)
	}
}

func TestShowingAListingIntoAPopupAlreadyUpKeepsWhatWasTypedIntoIt(t *testing.T) {
	p := show(t, "Palette", "Default")
	typeQuery(t, p, "p")

	p.Show("Symbols", []Entry{{Label: "Palette"}, {Label: "Themes"}})

	if got := p.Matched(); len(got) != 1 || got[0].Label != "Palette" {
		t.Errorf("the refreshed listing is %v, want the query still narrowing it", got)
	}
}

func TestShowingAListingIntoAClosedPopupStartsWithNoQuery(t *testing.T) {
	p := show(t, "Palette", "Default")
	typeQuery(t, p, "p")
	p.Close()

	p.Show("Symbols", []Entry{{Label: "Palette"}, {Label: "Default"}})

	if got := len(p.Matched()); got != 2 {
		t.Errorf("the new listing shows %d rows, want both", got)
	}
}
