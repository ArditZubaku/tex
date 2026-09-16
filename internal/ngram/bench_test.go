package ngram

import (
	"testing"

	"github.com/ArditZubaku/tex/internal/buffer"
	"github.com/ArditZubaku/tex/internal/fixture"
)

// Build is redone once per word asked about — the same cadence a language
// server would be asked at — rather than kept in step with every edit, so
// this is the cost that cadence pays.
func BenchmarkBuildPlainText(b *testing.B) {
	buf := buffer.Open(fixture.PlainText(b, 5_000))
	defer buf.Close()

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		Build(buf)
	}
}

func BenchmarkBuildGoSource(b *testing.B) {
	buf := buffer.Open(fixture.GoSource(b, 5_000))
	defer buf.Close()

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		Build(buf)
	}
}

func BenchmarkRank(b *testing.B) {
	buf := buffer.Open(fixture.PlainText(b, 5_000))
	defer buf.Close()
	m := Build(buf)

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		m.Rank("abcdefghij", "abc", 50)
	}
}
