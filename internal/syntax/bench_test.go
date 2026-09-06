package syntax

import (
	"bufio"
	"os"
	"testing"

	"github.com/ArditZubaku/tex/internal/fixture"
	"github.com/ArditZubaku/tex/internal/theme"
)

func sourceLines(tb testing.TB, count int) [][]rune {
	tb.Helper()

	f, err := os.Open(fixture.GoSource(tb, count))
	if err != nil {
		tb.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	var lines [][]rune
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		lines = append(lines, []rune(sc.Text()))
	}
	if err := sc.Err(); err != nil {
		tb.Fatal(err)
	}

	return lines
}

// A screenful is what one redraw colours, and the number every keystroke pays.
func BenchmarkLineColorsScreenful(b *testing.B) {
	lines := sourceLines(b, 48)
	palette := theme.Default()

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		inBlock := false
		for _, line := range lines {
			_, inBlock = goSyntax.LineColors(line, inBlock, &palette)
		}
	}
}

// The whole file is what a command that reads the lexer's verdict over the
// buffer costs, and what the block-comment look-back approximates.
func BenchmarkHighlightFile(b *testing.B) {
	lines := sourceLines(b, 20_000)
	palette := theme.Default()
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		inBlock := false
		for _, line := range lines {
			inBlock = goSyntax.Highlight(line, inBlock, nil, &palette)
		}
	}
}

// Code is the same pass with the colours thrown away, run by every command
// that has to skip comments and strings.
func BenchmarkCodeScreenful(b *testing.B) {
	lines := sourceLines(b, 48)

	b.ReportAllocs()
	b.ResetTimer()

	var mask []bool
	for b.Loop() {
		inBlock := false
		for _, line := range lines {
			mask, inBlock = goSyntax.Code(line, inBlock, mask[:0])
		}
	}
}

func BenchmarkDetect(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		_ = Detect("internal/editor/render/render.go")
	}
}
