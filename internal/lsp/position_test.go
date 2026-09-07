package lsp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ArditZubaku/tex/internal/buffer"
)

// "héllo ünïcödé 😀 x": the é and the ü are two bytes and one UTF-16 unit, the
// emoji is four bytes and two UTF-16 units, so no two of the three encodings
// agree past the first rune.
const mixed = "héllo ünïcödé 😀 x"

func TestACursorPastTheEndOfALineCountsAsItsEnd(t *testing.T) {
	line := []rune("abc")

	for _, enc := range []Encoding{UTF8, UTF16} {
		if got := enc.Character(line, 99); got != 3 {
			t.Errorf("%s: Character(col 99) = %d, want 3", enc, got)
		}
		if got := enc.Character(line, -1); got != 0 {
			t.Errorf("%s: Character(col -1) = %d, want 0", enc, got)
		}
	}
}

func TestEachEncodingCountsAMixedLineItsOwnWay(t *testing.T) {
	line := []rune(mixed)

	cases := []struct {
		col          int
		bytes, units int
	}{
		{col: 0, bytes: 0, units: 0},
		{col: 1, bytes: 1, units: 1},    // h
		{col: 2, bytes: 3, units: 2},    // é is two bytes
		{col: 14, bytes: 19, units: 14}, // past "héllo ünïcödé "
		{col: 15, bytes: 23, units: 16}, // the emoji: four bytes, a surrogate pair
		{col: 17, bytes: 25, units: 18},
	}

	for _, one := range cases {
		if got := UTF8.Character(line, one.col); got != one.bytes {
			t.Errorf("UTF8.Character(col %d) = %d, want %d", one.col, got, one.bytes)
		}
		if got := UTF16.Character(line, one.col); got != one.units {
			t.Errorf("UTF16.Character(col %d) = %d, want %d", one.col, got, one.units)
		}
	}
}

func TestEveryColumnOfAMixedLineSurvivesTheRoundTrip(t *testing.T) {
	line := []rune(mixed)

	for _, enc := range []Encoding{UTF8, UTF16} {
		for col := range len(line) + 1 {
			if got := enc.Column(line, enc.Character(line, col)); got != col {
				t.Errorf("%s: column %d came back as %d", enc, col, got)
			}
		}
	}
}

// A server counting the other way would land between two runes, which is no
// column the cursor can be at.
func TestAUnitInsideARuneNamesTheRuneItIsIn(t *testing.T) {
	line := []rune(mixed)

	if got := UTF8.Column(line, 2); got != 1 {
		t.Errorf("UTF8.Column(2) = %d, want 1: the second byte of é", got)
	}
	if got := UTF16.Column(line, 15); got != 14 {
		t.Errorf("UTF16.Column(15) = %d, want 14: the low half of the emoji", got)
	}
}

func TestAUnitPastTheEndOfALineNamesItsEnd(t *testing.T) {
	line := []rune("abc")

	for _, enc := range []Encoding{UTF8, UTF16} {
		if got := enc.Column(line, 99); got != 3 {
			t.Errorf("%s: Column(99) = %d, want 3", enc, got)
		}
		if got := enc.Column(line, -1); got != 0 {
			t.Errorf("%s: Column(-1) = %d, want 0", enc, got)
		}
	}
}

func TestAnEmptyLineIsColumnZeroWhicheverWayItIsCounted(t *testing.T) {
	for _, enc := range []Encoding{UTF8, UTF16} {
		if got := enc.Character(nil, 0); got != 0 {
			t.Errorf("%s: Character on an empty line = %d, want 0", enc, got)
		}
		if got := enc.Column(nil, 4); got != 0 {
			t.Errorf("%s: Column on an empty line = %d, want 0", enc, got)
		}
	}
}

func openTemp(t *testing.T, content string) *buffer.Buffer {
	t.Helper()

	path := filepath.Join(t.TempDir(), "f.go")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	b := buffer.Open(path)
	t.Cleanup(b.Close)

	return b
}

func TestAPositionOverABufferGoesBothWays(t *testing.T) {
	b := openTemp(t, "package main\n"+mixed+"\n")

	at := UTF8.Pos(b, 1, 15)
	if at.Line != 1 || at.Character != 23 {
		t.Fatalf("Pos = %+v, want line 1 character 23", at)
	}

	row, col := UTF8.RowCol(b, at)
	if row != 1 || col != 15 {
		t.Errorf("RowCol = %d,%d, want 1,15", row, col)
	}
}

// A server answering from text it was told about before the last change landed
// names a line the buffer no longer has.
func TestALineTheBufferNoLongerHasIsClampedOntoIt(t *testing.T) {
	b := openTemp(t, "a\nb\n")

	row, col := UTF16.RowCol(b, Position{Line: 99, Character: 40})
	if row != 1 || col != 1 {
		t.Errorf("RowCol = %d,%d, want the last line and its end (1,1)", row, col)
	}

	row, _ = UTF16.RowCol(b, Position{Line: -3})
	if row != 0 {
		t.Errorf("RowCol row = %d, want 0", row)
	}
}

func TestAServerNamingAnEncodingNeverOfferedIsRefused(t *testing.T) {
	if enc, ok := EncodingNamed("utf-8"); !ok || enc != UTF8 {
		t.Errorf("utf-8 = %v,%v", enc, ok)
	}
	if enc, ok := EncodingNamed("utf-16"); !ok || enc != UTF16 {
		t.Errorf("utf-16 = %v,%v", enc, ok)
	}
	if enc, ok := EncodingNamed(""); !ok || enc != UTF16 {
		t.Errorf("an unnamed encoding = %v,%v, want the protocol's own default", enc, ok)
	}
	if _, ok := EncodingNamed("utf-32"); ok {
		t.Error("utf-32 was accepted")
	}
}
