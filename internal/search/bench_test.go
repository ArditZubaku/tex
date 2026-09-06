package search

import (
	"testing"

	"github.com/ArditZubaku/tex/internal/buffer"
	"github.com/ArditZubaku/tex/internal/fixture"
)

const benchLines = 200_000

func benchBuffer(b *testing.B) *buffer.Buffer {
	b.Helper()

	buf := buffer.Open(fixture.PlainText(b, benchLines))
	b.Cleanup(buf.Close)

	return buf
}

// The renderer asks for the hits of every visible line on every frame, so a
// screenful is the number a lit search costs per keystroke.
func BenchmarkMatchesInScreenful(b *testing.B) {
	buf := benchBuffer(b)
	pat := New([]rune("hij"))

	b.ReportAllocs()
	b.ResetTimer()

	var cols []int
	row := 0
	for b.Loop() {
		for i := range 48 {
			cols = pat.MatchesIn(buf, row+i, cols[:0])
		}
		row = (row + 1) % (benchLines - 48)
	}
}

// An edited line is matched over runes rather than bytes, which is the other
// half of the same call.
func BenchmarkMatchesInEdited(b *testing.B) {
	buf := benchBuffer(b)
	pat := New([]rune("hij"))
	for i := range 48 {
		buf.SetLine(i, buf.Line(i))
	}

	b.ReportAllocs()
	b.ResetTimer()

	var cols []int
	for b.Loop() {
		for i := range 48 {
			cols = pat.MatchesIn(buf, i, cols[:0])
		}
	}
}

// Find walks lines out from the cursor; a pattern that only matches far away
// is what makes it walk.
func BenchmarkFindFar(b *testing.B) {
	buf := benchBuffer(b)
	pat := New([]rune("0190000 abc"))

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		if _, _, ok := Find(buf, pat, 0, 0, false); !ok {
			b.Fatal("pattern not found")
		}
	}
}

func BenchmarkFindNear(b *testing.B) {
	buf := benchBuffer(b)
	pat := New([]rune("hij abcde"))

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		if _, _, ok := Find(buf, pat, 1000, 0, false); !ok {
			b.Fatal("pattern not found")
		}
	}
}

func BenchmarkFindMissing(b *testing.B) {
	buf := buffer.Open(fixture.PlainText(b, 20_000))
	b.Cleanup(buf.Close)
	pat := New([]rune("zzzzzzzz"))

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		if _, _, ok := Find(buf, pat, 0, 0, false); ok {
			b.Fatal("pattern found")
		}
	}
}
