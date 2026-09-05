package editor

import (
	"testing"

	"github.com/ArditZubaku/tex/internal/editor/edit"
	"github.com/ArditZubaku/tex/internal/editor/edtest"
	"github.com/ArditZubaku/tex/internal/editor/state"
)

func TestOperators(t *testing.T) {
	const content = "foo bar baz\nlast\n"

	cases := []struct {
		name string
		col  int
		op   func(*state.Editor)
		want string
	}{
		{"x deletes under the cursor", 1, edit.DeleteRune, "fo bar baz"},
		{"x at end of line", 10, edit.DeleteRune, "foo bar ba"},
		{"x past end of line", 11, edit.DeleteRune, "foo bar baz"},
		{"dw from a word start", 0, edit.DeleteWord, "bar baz"},
		{"dw mid-word", 5, edit.DeleteWord, "foo bbaz"},
		{"dw on the last word stops at end of line", 8, edit.DeleteWord, "foo bar "},
		{"de from a word start", 4, edit.DeleteToWordEnd, "foo  baz"},
		{"de mid-word", 5, edit.DeleteToWordEnd, "foo b baz"},
		{"de on the last word stops at end of line", 8, edit.DeleteToWordEnd, "foo bar "},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := edtest.AtCursor(t, ed, content, 0, tc.col)

			tc.op(ed)
			edtest.WantLines(t, b, tc.want, "last")
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
			b := edtest.AtCursor(t, ed, "foo bar baz\nlast\n", 0, tc.col)

			edit.DeleteToPrevWord(ed)
			edtest.WantLines(t, b, tc.want, "last")
			if ed.Col != tc.col2 {
				t.Errorf("currentCol = %d, want %d", ed.Col, tc.col2)
			}
		})
	}
}

// db on the first column would walk onto the line above; operators stop there.
func TestDeleteToPrevWordStopsAtTheLineStart(t *testing.T) {
	b := edtest.AtCursor(t, ed, "foo\nbar\n", 1, 0)

	edit.DeleteToPrevWord(ed)
	edtest.WantLines(t, b, "foo", "bar")
	if ed.Modified {
		t.Error("db at the start of a line reported a modification")
	}
}

func TestBackspace(t *testing.T) {
	b := edtest.AtCursor(t, ed, "foo\nbar\n", 0, 2)
	ed.Mode = state.EditMode

	edit.Backspace(ed)
	edtest.WantLines(t, b, "fo", "bar")
	if ed.Col != 1 {
		t.Fatalf("currentCol = %d, want 1", ed.Col)
	}
}

func TestBackspaceAtColumnZeroJoins(t *testing.T) {
	b := edtest.AtCursor(t, ed, "foo\nbar\nbaz\n", 1, 0)
	ed.Mode = state.EditMode

	edit.Backspace(ed)
	edtest.WantLines(t, b, "foobar", "baz")
	if ed.Row != 0 || ed.Col != 3 {
		t.Fatalf("cursor at %d,%d, want 0,3", ed.Row, ed.Col)
	}

	// typing must continue where the join left off
	ed.Buf.InsertRune(ed.Row, ed.Col, 'X')
	edtest.WantLines(t, b, "fooXbar", "baz")
}

func TestBackspaceAtTheStartOfTheBuffer(t *testing.T) {
	b := edtest.AtCursor(t, ed, "foo\n", 0, 0)
	ed.Mode = state.EditMode

	edit.Backspace(ed)
	edtest.WantLines(t, b, "foo")
	if ed.Row != 0 || ed.Col != 0 || ed.Modified {
		t.Errorf("cursor at %d,%d, modified = %v", ed.Row, ed.Col, ed.Modified)
	}
}

func TestBackspaceInReadModeJustMoves(t *testing.T) {
	b := edtest.AtCursor(t, ed, "foo\n", 0, 2)
	ed.Mode = state.ReadMode

	edit.Backspace(ed)
	edtest.WantLines(t, b, "foo")
	if ed.Col != 1 || ed.Modified {
		t.Errorf("currentCol = %d, modified = %v", ed.Col, ed.Modified)
	}
}

func TestDeleteLineKeepsCursorInBuffer(t *testing.T) {
	b := edtest.AtCursor(t, ed, "a\nbb\nccc\n", 2, 2)

	edit.DeleteLine(ed)
	if ed.Row != 1 {
		t.Errorf("currentRow = %d, want 1", ed.Row)
	}
	edtest.WantLines(t, b, "a", "bb")
}
