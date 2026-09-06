package buffer

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/ArditZubaku/tex/internal/fixture"
)

const benchLines = 200_000

func benchBuffer(b *testing.B, path string) *Buffer {
	b.Helper()

	buf := Open(path)
	b.Cleanup(buf.Close)

	return buf
}

func BenchmarkOpen(b *testing.B) {
	path := fixture.PlainText(b, benchLines)
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		buf := Open(path)
		buf.Close()
	}
}

func BenchmarkBuildIndex(b *testing.B) {
	path := fixture.PlainText(b, benchLines)
	f, err := os.Open(path)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = f.Close() })
	info, err := f.Stat()
	if err != nil {
		b.Fatal(err)
	}

	b.SetBytes(info.Size())
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		if got := buildIndex(f, info.Size()); len(got) != benchLines {
			b.Fatalf("indexed %d lines, want %d", len(got), benchLines)
		}
	}
}

// A screenful read the way a redraw reads one: consecutive lines, all inside
// the window, decoded to runes.
func BenchmarkLineScreenful(b *testing.B) {
	buf := benchBuffer(b, fixture.PlainText(b, benchLines))
	b.ReportAllocs()
	b.ResetTimer()

	row := 0
	for b.Loop() {
		for i := range 48 {
			_ = buf.Line(row + i)
		}
		row = (row + 1) % (benchLines - 48)
	}
}

// Reading raw bytes is what search and the save path do, and what the renderer
// would do if it did not need runes.
func BenchmarkRawScreenful(b *testing.B) {
	buf := benchBuffer(b, fixture.PlainText(b, benchLines))
	b.ReportAllocs()
	b.ResetTimer()

	row := 0
	for b.Loop() {
		for i := range 48 {
			_ = buf.Raw(row + i)
		}
		row = (row + 1) % (benchLines - 48)
	}
}

func BenchmarkRuneLenScreenful(b *testing.B) {
	buf := benchBuffer(b, fixture.PlainText(b, benchLines))
	b.ReportAllocs()
	b.ResetTimer()

	row := 0
	for b.Loop() {
		for i := range 48 {
			_ = buf.RuneLen(row + i)
		}
		row = (row + 1) % (benchLines - 48)
	}
}

// Jumping past the window every time is the refill cost on its own.
func BenchmarkLineRandomAccess(b *testing.B) {
	buf := benchBuffer(b, fixture.PlainText(b, benchLines))
	b.ReportAllocs()
	b.ResetTimer()

	row := 0
	for b.Loop() {
		_ = buf.Line(row)
		row = (row + 4001) % benchLines
	}
}

// Typing a run of characters into the middle of a big buffer: the first insert
// copies the line out of the window, the rest grow it in the overlay.
func BenchmarkInsertRune(b *testing.B) {
	buf := benchBuffer(b, fixture.PlainText(b, benchLines))
	b.ReportAllocs()
	b.ResetTimer()

	row, col := benchLines/2, 40
	for b.Loop() {
		buf.InsertRune(row, col, 'x')
		if buf.RuneLen(row) > 4096 {
			buf.SetLine(row, nil)
		}
	}
}

func BenchmarkDeleteRunes(b *testing.B) {
	buf := benchBuffer(b, fixture.PlainText(b, benchLines))
	b.ReportAllocs()
	b.ResetTimer()

	row := benchLines / 2
	line := []rune(strings.Repeat("abcdefghij ", 40))
	buf.SetLine(row, slices.Clone(line))

	for b.Loop() {
		if buf.RuneLen(row) < 8 {
			buf.SetLine(row, slices.Clone(line))
		}
		buf.DeleteRunes(row, 3, 4)
	}
}

// InsertLine and DeleteLine both move the index and re-key the overlay, which
// is the cost that scales with the file rather than with the line. They are
// measured as a pair so that the line count — and with it the size of the move
// — is the same on every iteration, whatever -benchtime asks for.
func BenchmarkInsertDeleteLine(b *testing.B) {
	buf := benchBuffer(b, fixture.PlainText(b, benchLines))
	at := benchLines / 2

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		buf.InsertLine(at)
		buf.DeleteLine(at)
	}
	if buf.LineCount() != benchLines {
		b.Fatalf("line count drifted to %d", buf.LineCount())
	}
}

// The same pair with an overlay big enough to make the re-keying show, which
// is what a long editing session leaves behind.
func BenchmarkInsertDeleteLineEdited(b *testing.B) {
	buf := benchBuffer(b, fixture.PlainText(b, benchLines))
	at := benchLines / 2
	edit := []rune("an edited line")
	for i := range 2000 {
		buf.SetLine(i*97, edit)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		buf.InsertLine(at)
		buf.DeleteLine(at)
	}
	if buf.LineCount() != benchLines {
		b.Fatalf("line count drifted to %d", buf.LineCount())
	}
}

// Enter and the Backspace that takes it back, which is the same index move
// with a line's runes copied either way.
func BenchmarkSplitJoinLine(b *testing.B) {
	buf := benchBuffer(b, fixture.PlainText(b, benchLines))
	at := benchLines / 2

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		buf.SplitLine(at, 40)
		buf.JoinLine(at)
	}
	if buf.LineCount() != benchLines {
		b.Fatalf("line count drifted to %d", buf.LineCount())
	}
}

// Saving walks the whole index and writes every line, so it is the one
// operation whose cost is the file rather than the screen.
func BenchmarkSave(b *testing.B) {
	src := fixture.PlainText(b, benchLines/10)
	dst := b.TempDir() + "/out.txt"
	if err := copyFile(b, src, dst); err != nil {
		b.Fatal(err)
	}

	buf := benchBuffer(b, dst)
	edit := []rune("an edited line that the overlay has to write as runes")

	b.ReportAllocs()
	b.ResetTimer()

	// Save reopens the file, which drops the overlay, so the edits are put
	// back each time: 200 map writes against a save that costs milliseconds.
	for b.Loop() {
		for i := range 200 {
			buf.SetLine(i*17, edit)
		}
		if err := buf.Save(dst); err != nil {
			b.Fatal(err)
		}
	}
}

func copyFile(b *testing.B, src, dst string) error {
	b.Helper()

	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	return os.WriteFile(dst, data, 0o600)
}
