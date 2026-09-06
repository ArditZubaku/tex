package fuzzy

import (
	"testing"

	"github.com/ArditZubaku/tex/internal/fixture"
)

// The picker rescores every path on every keystroke, so the listing size is
// what one typed character costs.
func BenchmarkScoreListing(b *testing.B) {
	paths := fixture.Paths(20_000)
	query := []rune("rendr")

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		hits := 0
		for _, path := range paths {
			if _, ok := Score(path, query); ok {
				hits++
			}
		}
		if hits == 0 {
			b.Fatal("nothing matched")
		}
	}
}

// A query that fails on its first character is the common case while typing,
// and the one an early exit would help most.
func BenchmarkScoreNoMatch(b *testing.B) {
	paths := fixture.Paths(20_000)
	query := []rune("qqqq")

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		for _, path := range paths {
			_, _ = Score(path, query)
		}
	}
}

func BenchmarkScoreEmptyQuery(b *testing.B) {
	paths := fixture.Paths(20_000)

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		for _, path := range paths {
			_, _ = Score(path, nil)
		}
	}
}
