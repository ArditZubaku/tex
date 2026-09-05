package editor

import (
	"testing"

	"github.com/ArditZubaku/tex/internal/buffer"
	"github.com/ArditZubaku/tex/internal/editor/state"
)

func TestInsertLine(t *testing.T) {
	cases := []struct {
		name string
		at   int
		want []string
	}{
		{"at the top", 0, []string{"", "a", "bb", "ccc"}},
		{"in the middle", 1, []string{"a", "", "bb", "ccc"}},
		{"at the end", 3, []string{"a", "bb", "ccc", ""}},
		{"past the end", 4, []string{"a", "bb", "ccc"}},
		{"before the start", -1, []string{"a", "bb", "ccc"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := buffer.Open(writeTemp(t, "a\nbb\nccc\n"))
			defer b.Close()

			b.InsertLine(tc.at)
			wantLines(t, b, tc.want...)
		})
	}
}

// The inserted line borrows the offset of the line below it, so the lines
// either side of it must still read back from the file untouched.
func TestInsertLineKeepsNeighboursReadable(t *testing.T) {
	b := buffer.Open(writeTemp(t, "a\nbb\nccc\ndddd\n"))
	defer b.Close()

	b.InsertLine(2)
	b.SetLine(2, []rune("NEW"))
	wantLines(t, b, "a", "bb", "NEW", "ccc", "dddd")

	// again, now that the index already holds a duplicated offset
	b.InsertLine(4)
	wantLines(t, b, "a", "bb", "NEW", "ccc", "", "dddd")
}

// A file with no trailing newline ends its last line at size rather than at a
// terminator, so appending a line must not eat that line's final character.
func TestInsertLineAtTheEndOfAnUnterminatedFile(t *testing.T) {
	for _, tc := range []struct{ name, content string }{
		{"terminated", "a\nbb\n"},
		{"unterminated", "a\nbb"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := buffer.Open(writeTemp(t, tc.content))
			defer b.Close()

			b.InsertLine(2)
			wantLines(t, b, "a", "bb", "")
		})
	}
}

func TestInsertLineShiftsOverlay(t *testing.T) {
	b := buffer.Open(writeTemp(t, "a\nb\nc\nd\n"))
	defer b.Close()

	b.SetLine(1, []rune("B"))
	b.SetLine(3, []rune("D"))

	b.InsertLine(1)
	wantLines(t, b, "a", "", "B", "c", "D")
}

func TestInsertLineOnAnEmptyBuffer(t *testing.T) {
	b := buffer.NewEmpty()

	b.InsertLine(1)
	b.InsertRune(1, 0, 'x')
	wantLines(t, b, "", "x")

	b.DeleteLine(0)
	wantLines(t, b, "x")
}

func TestInsertLineSaves(t *testing.T) {
	path := writeTemp(t, "a\nbb\n")
	b := buffer.Open(path)
	defer b.Close()

	b.InsertLine(1)
	b.InsertRune(1, 0, 'X')
	if err := b.Save(path); err != nil {
		t.Fatal(err)
	}

	if got := readBack(t, path); got != "a\nX\nbb\n" {
		t.Fatalf("file = %q, want %q", got, "a\nX\nbb\n")
	}
	wantLines(t, b, "a", "X", "bb")
}

func TestInsertLineAcrossBigFile(t *testing.T) {
	path := bigFile(t, 5000)
	want := fullDecode(t, path)

	b := buffer.Open(path)
	defer b.Close()

	// 41 is longer than the window, 97 is empty
	for _, row := range []int{4999, 2345, 98, 42, 41, 1, 0} {
		b.InsertLine(row)
		want = append(want[:row], append([][]rune{{}}, want[row:]...)...)
	}

	if b.LineCount() != len(want) {
		t.Fatalf("LineCount = %d, want %d", b.LineCount(), len(want))
	}
	for i := range want {
		if got := string(b.Line(i)); got != string(want[i]) {
			t.Fatalf("Line(%d) = %.40q, want %.40q", i, got, string(want[i]))
		}
	}
}

func TestSplitLine(t *testing.T) {
	cases := []struct {
		name string
		col  int
		want []string
	}{
		{"mid-line", 2, []string{"a", "bb", "cc", "c", "dddd"}},
		{"at the start", 0, []string{"a", "bb", "", "ccc", "dddd"}},
		{"at the end", 3, []string{"a", "bb", "ccc", "", "dddd"}},
		{"clamped past the end", 99, []string{"a", "bb", "ccc", "", "dddd"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := buffer.Open(writeTemp(t, "a\nbb\nccc\ndddd\n"))
			defer b.Close()

			b.SplitLine(2, tc.col)
			wantLines(t, b, tc.want...)
		})
	}
}

// Split then join is a round trip, and both halves have to stay editable.
func TestSplitLineThenEdit(t *testing.T) {
	b := buffer.Open(writeTemp(t, "héllo wörld\nlast\n"))
	defer b.Close()

	b.SplitLine(0, 6)
	wantLines(t, b, "héllo ", "wörld", "last")

	b.InsertRune(0, 6, '-')
	b.InsertRune(1, 5, '!')
	wantLines(t, b, "héllo -", "wörld!", "last")

	b.DeleteRunes(0, 6, 7)
	b.JoinLine(0)
	wantLines(t, b, "héllo wörld!", "last")
}

func TestSplitLineSaves(t *testing.T) {
	path := writeTemp(t, "abcd\nlast\n")
	b := buffer.Open(path)
	defer b.Close()

	b.SplitLine(0, 2)
	if err := b.Save(path); err != nil {
		t.Fatal(err)
	}

	if got := readBack(t, path); got != "ab\ncd\nlast\n" {
		t.Fatalf("file = %q, want %q", got, "ab\ncd\nlast\n")
	}
	wantLines(t, b, "ab", "cd", "last")
}

func TestSplitLineAcrossBigFile(t *testing.T) {
	path := bigFile(t, 5000)
	want := fullDecode(t, path)

	b := buffer.Open(path)
	defer b.Close()

	// 41 is longer than the window, 97 is empty
	for _, row := range []int{4999, 2345, 97, 41, 0} {
		b.SplitLine(row, 1)
		head, tail := want[row][:min(1, len(want[row]))], want[row][min(1, len(want[row])):]
		want = append(want[:row], append([][]rune{head, tail}, want[row+1:]...)...)
	}

	if b.LineCount() != len(want) {
		t.Fatalf("LineCount = %d, want %d", b.LineCount(), len(want))
	}
	for i := range want {
		if got := string(b.Line(i)); got != string(want[i]) {
			t.Fatalf("Line(%d) = %.40q, want %.40q", i, got, string(want[i]))
		}
	}
}

func TestEnter(t *testing.T) {
	t.Run("splits in Edit mode", func(t *testing.T) {
		b := atCursor(t, "abcd\nlast\n", 0, 2)
		ed.Mode = state.EditMode

		enter()
		wantLines(t, b, "ab", "cd", "last")
		if ed.Row != 1 || ed.Col != 0 || !ed.Modified {
			t.Errorf("cursor at %d,%d, modified %v", ed.Row, ed.Col, ed.Modified)
		}
	})

	t.Run("moves down in Read mode", func(t *testing.T) {
		b := atCursor(t, "abcd\nlast\n", 0, 2)
		ed.Mode = state.ReadMode

		enter()
		wantLines(t, b, "abcd", "last")
		if ed.Row != 1 || ed.Modified {
			t.Errorf("currentRow = %d, modified %v", ed.Row, ed.Modified)
		}
	})

	t.Run("split then backspace is a round trip", func(t *testing.T) {
		b := atCursor(t, "abcd\n", 0, 2)
		ed.Mode = state.EditMode

		enter()
		backspace()
		wantLines(t, b, "abcd")
		if ed.Row != 0 || ed.Col != 2 {
			t.Errorf("cursor at %d,%d, want 0,2", ed.Row, ed.Col)
		}
	})
}

func TestOpenLineOperators(t *testing.T) {
	t.Run("o opens below", func(t *testing.T) {
		b := atCursor(t, "a\nbb\n", 0, 1)
		ed.Mode = state.ReadMode

		openLineBelow()
		wantLines(t, b, "a", "", "bb")
		if ed.Row != 1 || ed.Col != 0 || ed.Mode != state.EditMode || !ed.Modified {
			t.Errorf("cursor at %d,%d, mode %v, modified %v", ed.Row, ed.Col, ed.Mode, ed.Modified)
		}
	})

	t.Run("O opens above", func(t *testing.T) {
		b := atCursor(t, "a\nbb\n", 1, 2)
		ed.Mode = state.ReadMode

		openLineAbove()
		wantLines(t, b, "a", "", "bb")
		if ed.Row != 1 || ed.Col != 0 || ed.Mode != state.EditMode {
			t.Errorf("cursor at %d,%d, mode %v", ed.Row, ed.Col, ed.Mode)
		}
	})

	t.Run("o on the last line", func(t *testing.T) {
		b := atCursor(t, "a\nbb\n", 1, 0)
		ed.Mode = state.ReadMode

		openLineBelow()
		wantLines(t, b, "a", "bb", "")
		if ed.Row != 2 {
			t.Errorf("currentRow = %d, want 2", ed.Row)
		}
	})
}
