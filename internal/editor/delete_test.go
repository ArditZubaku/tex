package editor

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ArditZubaku/tex/internal/buffer"
	"github.com/ArditZubaku/tex/internal/editor/state"
)

func lines(t *testing.T, b *buffer.Buffer) []string {
	t.Helper()

	out := make([]string, b.LineCount())
	for i := range out {
		out[i] = string(b.Line(i))
	}

	return out
}

func wantLines(t *testing.T, b *buffer.Buffer, want ...string) {
	t.Helper()

	got := lines(t, b)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("buffer = %q, want %q", got, want)
	}
}

func TestDeleteRunes(t *testing.T) {
	cases := []struct {
		name     string
		from, to int
		want     string
	}{
		{"head", 0, 3, "345"},
		{"middle", 2, 4, "0145"},
		{"tail", 4, 6, "0123"},
		{"whole line", 0, 6, ""},
		{"clamped past end", 4, 99, "0123"},
		{"clamped before start", -5, 2, "2345"},
		{"empty range", 3, 3, "012345"},
		{"reversed range", 4, 1, "012345"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := buffer.Open(writeTemp(t, "012345\nkeep\n"))
			defer b.Close()

			b.DeleteRunes(0, tc.from, tc.to)
			wantLines(t, b, tc.want, "keep")
		})
	}
}

func TestDeleteRunesRepeatedlyOnSameLine(t *testing.T) {
	b := buffer.Open(writeTemp(t, "héllo wörld\n"))
	defer b.Close()

	b.DeleteRunes(0, 0, 6) // "wörld", the second delete now works on the overlay
	b.DeleteRunes(0, 1, 3)
	wantLines(t, b, "wld")
}

func TestDeleteLine(t *testing.T) {
	cases := []struct {
		name    string
		content string
		row     int
		want    []string
	}{
		{"first", "a\nbb\nccc\n", 0, []string{"bb", "ccc"}},
		{"middle", "a\nbb\nccc\n", 1, []string{"a", "ccc"}},
		{"last", "a\nbb\nccc\n", 2, []string{"a", "bb"}},
		{"last, unterminated file", "a\nbb\nccc", 2, []string{"a", "bb"}},
		{"only line leaves an empty one", "solo\n", 0, []string{""}},
		{"out of range", "a\nbb\n", 7, []string{"a", "bb"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := buffer.Open(writeTemp(t, tc.content))
			defer b.Close()

			b.DeleteLine(tc.row)
			wantLines(t, b, tc.want...)
		})
	}
}

// Deleting a line renumbers every line after it, so edits held in the overlay
// have to move down with the lines they belong to.
func TestDeleteLineReKeysOverlay(t *testing.T) {
	b := buffer.Open(writeTemp(t, "a\nb\nc\nd\ne\n"))
	defer b.Close()

	b.SetLine(0, []rune("A"))
	b.SetLine(3, []rune("D"))
	b.SetLine(4, []rune("E"))

	b.DeleteLine(1)
	wantLines(t, b, "A", "c", "D", "E")

	b.DeleteLine(2) // the edited "D"
	wantLines(t, b, "A", "c", "E")
}

func TestDeleteLineThenReadFromFile(t *testing.T) {
	long := strings.Repeat("z", buffer.WindowBytes+7)
	b := buffer.Open(writeTemp(t, "a\n"+long+"\nc\nd\n"))
	defer b.Close()

	b.DeleteLine(0)
	// forces the window, keyed by line number, to be refilled after the shift
	if got := string(b.Line(0)); got != long {
		t.Fatalf("Line(0) len = %d, want %d", len(got), len(long))
	}
	wantLines(t, b, long, "c", "d")
}

func TestDeleteEveryLine(t *testing.T) {
	b := buffer.Open(writeTemp(t, "a\nbb\nccc\n"))
	defer b.Close()

	for range 5 {
		b.DeleteLine(0)
	}
	wantLines(t, b, "")
}

func TestDeleteLineOnEmptyBuffer(t *testing.T) {
	b := buffer.NewEmpty()

	b.DeleteLine(0)
	wantLines(t, b, "")
}

func TestInsertRune(t *testing.T) {
	cases := []struct {
		name string
		col  int
		want string
	}{
		{"head", 0, "Xabc"},
		{"middle", 2, "abXc"},
		{"tail", 3, "abcX"},
		{"clamped past end", 99, "abcX"},
		{"clamped before start", -1, "Xabc"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := buffer.Open(writeTemp(t, "abc\nkeep\n"))
			defer b.Close()

			b.InsertRune(0, tc.col, 'X')
			wantLines(t, b, tc.want, "keep")
		})
	}
}

// Typing must not copy the whole line on every keystroke: the first insert
// lifts the line into the overlay, the rest grow it amortized.
func TestTypingIntoAnEditedLineDoesNotAllocate(t *testing.T) {
	b := buffer.Open(writeTemp(t, "abc\n"))
	defer b.Close()

	b.InsertRune(0, 0, 'X')

	if allocs := testing.AllocsPerRun(1000, func() { b.InsertRune(0, 1, 'y') }); allocs > 0 {
		t.Errorf("typing allocated %v times per keystroke, want amortized 0", allocs)
	}
	// 3 + the first insert + 1000 runs + the warm-up run AllocsPerRun does
	if got := b.RuneLen(0); got != 1005 {
		t.Errorf("RuneLen = %d, want 1005", got)
	}
}

// Deleting scattered lines from a big file exercises the index shift, the
// overlay re-key and the window invalidation together, against a full decode.
func TestDeleteLinesAcrossBigFile(t *testing.T) {
	path := bigFile(t, 5000)
	want := fullDecode(t, path)

	b := buffer.Open(path)
	defer b.Close()

	del := func(row int) {
		t.Helper()
		b.DeleteLine(row)
		want = slices.Delete(want, row, row+1)
	}

	// row 41 is longer than the window, row 0 and 97 are empty
	for _, row := range []int{4999, 2345, 2345, 42, 41, 40, 97, 1, 0} {
		del(row)
	}

	// an edit must survive the deletion of the line above it
	b.SetLine(500, []rune("EDITED"))
	want[500] = []rune("EDITED")
	del(499)

	if b.LineCount() != len(want) {
		t.Fatalf("LineCount = %d, want %d", b.LineCount(), len(want))
	}
	for i := range want {
		if got := string(b.Line(i)); got != string(want[i]) {
			t.Fatalf("Line(%d) = %.40q, want %.40q", i, got, string(want[i]))
		}
	}
}

// The operators run off the cursor globals, so drive them the way the key
// handler does rather than through the Buffer directly.
func writeTemp(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "f.txt")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	return path
}

func atCursor(t *testing.T, content string, row, col int) *buffer.Buffer {
	t.Helper()

	path := writeTemp(t, content)
	b := buffer.Open(path)
	t.Cleanup(b.Close)
	ed.Buf, ed.SourceFile, ed.Row, ed.Col, ed.Modified = b, path, row, col, false

	return b
}

func TestOperators(t *testing.T) {
	const content = "foo bar baz\nlast\n"

	cases := []struct {
		name string
		col  int
		op   func()
		want string
	}{
		{"x deletes under the cursor", 1, deleteRune, "fo bar baz"},
		{"x at end of line", 10, deleteRune, "foo bar ba"},
		{"x past end of line", 11, deleteRune, "foo bar baz"},
		{"dw from a word start", 0, deleteWord, "bar baz"},
		{"dw mid-word", 5, deleteWord, "foo bbaz"},
		{"dw on the last word stops at end of line", 8, deleteWord, "foo bar "},
		{"de from a word start", 4, deleteToWordEnd, "foo  baz"},
		{"de mid-word", 5, deleteToWordEnd, "foo b baz"},
		{"de on the last word stops at end of line", 8, deleteToWordEnd, "foo bar "},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := atCursor(t, content, 0, tc.col)

			tc.op()
			wantLines(t, b, tc.want, "last")
			if ed.Modified != (tc.want != "foo bar baz") {
				t.Errorf("modified = %v after a %s", ed.Modified, tc.name)
			}
		})
	}
}

// db deletes behind the cursor, so unlike dw/de the cursor moves with it.
func TestDeleteToPrevWord(t *testing.T) {
	cases := []struct {
		name string
		col  int
		want string
		col2 int
	}{
		{"from a word start", 8, "foo baz", 4},
		{"mid-word", 6, "foo r baz", 4},
		{"from end of line", 11, "foo bar ", 8},
		{"at start of line does nothing", 0, "foo bar baz", 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := atCursor(t, "foo bar baz\nlast\n", 0, tc.col)

			deleteToPrevWord()
			wantLines(t, b, tc.want, "last")
			if ed.Col != tc.col2 {
				t.Errorf("currentCol = %d, want %d", ed.Col, tc.col2)
			}
		})
	}
}

// db on the first column would walk onto the line above; operators stop there.
func TestDeleteToPrevWordStopsAtTheLineStart(t *testing.T) {
	b := atCursor(t, "foo\nbar\n", 1, 0)

	deleteToPrevWord()
	wantLines(t, b, "foo", "bar")
	if ed.Modified {
		t.Error("db at the start of a line reported a modification")
	}
}

func TestJoinLine(t *testing.T) {
	b := buffer.Open(writeTemp(t, "a\nbb\nccc\n"))
	defer b.Close()

	b.JoinLine(0)
	wantLines(t, b, "abb", "ccc")

	b.JoinLine(0)
	wantLines(t, b, "abbccc")

	b.JoinLine(0) // no line below to join
	wantLines(t, b, "abbccc")
}

func TestBackspace(t *testing.T) {
	b := atCursor(t, "foo\nbar\n", 0, 2)
	ed.Mode = state.EditMode

	backspace()
	wantLines(t, b, "fo", "bar")
	if ed.Col != 1 {
		t.Fatalf("currentCol = %d, want 1", ed.Col)
	}
}

func TestBackspaceAtColumnZeroJoins(t *testing.T) {
	b := atCursor(t, "foo\nbar\nbaz\n", 1, 0)
	ed.Mode = state.EditMode

	backspace()
	wantLines(t, b, "foobar", "baz")
	if ed.Row != 0 || ed.Col != 3 {
		t.Fatalf("cursor at %d,%d, want 0,3", ed.Row, ed.Col)
	}

	// typing must continue where the join left off
	ed.Buf.InsertRune(ed.Row, ed.Col, 'X')
	wantLines(t, b, "fooXbar", "baz")
}

func TestBackspaceAtTheStartOfTheBuffer(t *testing.T) {
	b := atCursor(t, "foo\n", 0, 0)
	ed.Mode = state.EditMode

	backspace()
	wantLines(t, b, "foo")
	if ed.Row != 0 || ed.Col != 0 || ed.Modified {
		t.Errorf("cursor at %d,%d, modified = %v", ed.Row, ed.Col, ed.Modified)
	}
}

func TestBackspaceInReadModeJustMoves(t *testing.T) {
	b := atCursor(t, "foo\n", 0, 2)
	ed.Mode = state.ReadMode

	backspace()
	wantLines(t, b, "foo")
	if ed.Col != 1 || ed.Modified {
		t.Errorf("currentCol = %d, modified = %v", ed.Col, ed.Modified)
	}
}

func TestDeleteLineKeepsCursorInBuffer(t *testing.T) {
	b := atCursor(t, "a\nbb\nccc\n", 2, 2)

	deleteLine()
	if ed.Row != 1 {
		t.Errorf("currentRow = %d, want 1", ed.Row)
	}
	wantLines(t, b, "a", "bb")
}
