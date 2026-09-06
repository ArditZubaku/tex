package gutter

import "testing"

// Width is asked once per window per frame, Label once per visible row.
func BenchmarkWidth(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		_ = Width(200_000)
	}
}

func BenchmarkLabelScreenful(b *testing.B) {
	width := Width(200_000)

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		for row := range 48 {
			_ = Label(100_000+row, 100_024, width)
		}
	}
}
