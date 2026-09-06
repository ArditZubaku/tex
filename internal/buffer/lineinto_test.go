package buffer

import (
	"slices"
	"testing"
)

func TestLineIntoMatchesLine(t *testing.T) {
	path := bigFile(t, 20000)

	want := fullDecode(t, path)
	b := Open(path)
	defer b.Close()

	var scratch []rune
	for i := range want {
		scratch = b.LineInto(i, scratch)
		if string(scratch) != string(want[i]) {
			t.Fatalf("LineInto(%d) mismatch (len %d vs %d)", i, len(scratch), len(want[i]))
		}
	}

	for i := range slices.Backward(want) {
		scratch = b.LineInto(i, scratch)
		if string(scratch) != string(want[i]) {
			t.Fatalf("LineInto(%d) backwards mismatch", i)
		}
	}
}

func TestLineIntoReadsTheOverlayWithoutAliasingIt(t *testing.T) {
	b := Open(writeTemp(t, "one\ntwo\nthree\n"))
	defer b.Close()

	edited := []rune("edited")
	b.SetLine(1, edited)

	scratch := b.LineInto(1, nil)
	if string(scratch) != "edited" {
		t.Fatalf("LineInto(1) = %q, want %q", string(scratch), "edited")
	}

	scratch = b.LineInto(2, scratch)
	if string(scratch) != "three" {
		t.Fatalf("LineInto(2) = %q, want %q", string(scratch), "three")
	}
	if string(edited) != "edited" {
		t.Errorf("the overlay's line became %q", string(edited))
	}
	if got := string(b.Line(1)); got != "edited" {
		t.Errorf("Line(1) = %q, want %q", got, "edited")
	}
}

func TestLineIntoOutOfRange(t *testing.T) {
	b := Open(writeTemp(t, "one\ntwo\n"))
	defer b.Close()

	scratch := b.LineInto(0, nil)
	for _, row := range []int{-1, 2, 99} {
		if got := b.LineInto(row, scratch); len(got) != 0 {
			t.Errorf("LineInto(%d) = %q, want empty", row, string(got))
		}
	}
}

func TestLineIntoReusesItsScratch(t *testing.T) {
	b := Open(writeTemp(t, "aaaaaaaaaaaaaaaa\nbb\ncccc\n"))
	defer b.Close()

	scratch := b.LineInto(0, nil)
	grown := scratch[:cap(scratch)]

	for _, row := range []int{1, 2, 0, 2, 1} {
		next := b.LineInto(row, scratch)
		if &next[:cap(next)][0] != &grown[0] {
			t.Fatalf("LineInto(%d) allocated a new array", row)
		}
		scratch = next
	}
}
