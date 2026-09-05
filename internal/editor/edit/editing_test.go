// The tests sit outside the package because the harness they share with the
// rest of the editor imports it.
package edit_test

import (
	"testing"

	"github.com/ArditZubaku/tex/internal/editor/edit"
	"github.com/ArditZubaku/tex/internal/editor/edtest"
	"github.com/ArditZubaku/tex/internal/editor/state"
)

func TestOperators(t *testing.T) {
	e := state.New()

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
			b := edtest.AtCursor(t, e, content, 0, tc.col)

			tc.op(e)
			edtest.WantLines(t, b, tc.want, "last")
			if e.Modified != (tc.want != "foo bar baz") {
				t.Errorf("modified = %v after a %s", e.Modified, tc.name)
			}
		})
	}
}

// db deletes behind the cursor, so unlike dw/de the cursor moves with it.
func TestDeleteToPrevWord(t *testing.T) {
	e := state.New()

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
			b := edtest.AtCursor(t, e, "foo bar baz\nlast\n", 0, tc.col)

			edit.DeleteToPrevWord(e)
			edtest.WantLines(t, b, tc.want, "last")
			if e.Col != tc.col2 {
				t.Errorf("currentCol = %d, want %d", e.Col, tc.col2)
			}
		})
	}
}

// db on the first column would walk onto the line above; operators stop there.
func TestDeleteToPrevWordStopsAtTheLineStart(t *testing.T) {
	e := state.New()

	b := edtest.AtCursor(t, e, "foo\nbar\n", 1, 0)

	edit.DeleteToPrevWord(e)
	edtest.WantLines(t, b, "foo", "bar")
	if e.Modified {
		t.Error("db at the start of a line reported a modification")
	}
}

func TestBackspace(t *testing.T) {
	e := state.New()

	b := edtest.AtCursor(t, e, "foo\nbar\n", 0, 2)
	e.Mode = state.EditMode

	edit.Backspace(e)
	edtest.WantLines(t, b, "fo", "bar")
	if e.Col != 1 {
		t.Fatalf("currentCol = %d, want 1", e.Col)
	}
}

func TestBackspaceAtColumnZeroJoins(t *testing.T) {
	e := state.New()

	b := edtest.AtCursor(t, e, "foo\nbar\nbaz\n", 1, 0)
	e.Mode = state.EditMode

	edit.Backspace(e)
	edtest.WantLines(t, b, "foobar", "baz")
	if e.Row != 0 || e.Col != 3 {
		t.Fatalf("cursor at %d,%d, want 0,3", e.Row, e.Col)
	}

	// typing must continue where the join left off
	e.Buf.InsertRune(e.Row, e.Col, 'X')
	edtest.WantLines(t, b, "fooXbar", "baz")
}

func TestBackspaceAtTheStartOfTheBuffer(t *testing.T) {
	e := state.New()

	b := edtest.AtCursor(t, e, "foo\n", 0, 0)
	e.Mode = state.EditMode

	edit.Backspace(e)
	edtest.WantLines(t, b, "foo")
	if e.Row != 0 || e.Col != 0 || e.Modified {
		t.Errorf("cursor at %d,%d, modified = %v", e.Row, e.Col, e.Modified)
	}
}

func TestBackspaceInReadModeJustMoves(t *testing.T) {
	e := state.New()

	b := edtest.AtCursor(t, e, "foo\n", 0, 2)
	e.Mode = state.ReadMode

	edit.Backspace(e)
	edtest.WantLines(t, b, "foo")
	if e.Col != 1 || e.Modified {
		t.Errorf("currentCol = %d, modified = %v", e.Col, e.Modified)
	}
}

func TestDeleteLineKeepsCursorInBuffer(t *testing.T) {
	e := state.New()

	b := edtest.AtCursor(t, e, "a\nbb\nccc\n", 2, 2)

	edit.DeleteLine(e)
	if e.Row != 1 {
		t.Errorf("currentRow = %d, want 1", e.Row)
	}
	edtest.WantLines(t, b, "a", "bb")
}

func TestEnter(t *testing.T) {
	e := state.New()

	t.Run("splits in Edit mode", func(t *testing.T) {
		b := edtest.AtCursor(t, e, "abcd\nlast\n", 0, 2)
		e.Mode = state.EditMode

		edit.Enter(e)
		edtest.WantLines(t, b, "ab", "cd", "last")
		if e.Row != 1 || e.Col != 0 || !e.Modified {
			t.Errorf("cursor at %d,%d, modified %v", e.Row, e.Col, e.Modified)
		}
	})

	t.Run("moves down in Read mode", func(t *testing.T) {
		b := edtest.AtCursor(t, e, "abcd\nlast\n", 0, 2)
		e.Mode = state.ReadMode

		edit.Enter(e)
		edtest.WantLines(t, b, "abcd", "last")
		if e.Row != 1 || e.Modified {
			t.Errorf("currentRow = %d, modified %v", e.Row, e.Modified)
		}
	})

	t.Run("split then backspace is a round trip", func(t *testing.T) {
		b := edtest.AtCursor(t, e, "abcd\n", 0, 2)
		e.Mode = state.EditMode

		edit.Enter(e)
		edit.Backspace(e)
		edtest.WantLines(t, b, "abcd")
		if e.Row != 0 || e.Col != 2 {
			t.Errorf("cursor at %d,%d, want 0,2", e.Row, e.Col)
		}
	})
}

func TestOpenLineOperators(t *testing.T) {
	e := state.New()

	t.Run("o opens below", func(t *testing.T) {
		b := edtest.AtCursor(t, e, "a\nbb\n", 0, 1)
		e.Mode = state.ReadMode

		edit.OpenLineBelow(e)
		edtest.WantLines(t, b, "a", "", "bb")
		if e.Row != 1 || e.Col != 0 || e.Mode != state.EditMode || !e.Modified {
			t.Errorf("cursor at %d,%d, mode %v, modified %v", e.Row, e.Col, e.Mode, e.Modified)
		}
	})

	t.Run("O opens above", func(t *testing.T) {
		b := edtest.AtCursor(t, e, "a\nbb\n", 1, 2)
		e.Mode = state.ReadMode

		edit.OpenLineAbove(e)
		edtest.WantLines(t, b, "a", "", "bb")
		if e.Row != 1 || e.Col != 0 || e.Mode != state.EditMode {
			t.Errorf("cursor at %d,%d, mode %v", e.Row, e.Col, e.Mode)
		}
	})

	t.Run("o on the last line", func(t *testing.T) {
		b := edtest.AtCursor(t, e, "a\nbb\n", 1, 0)
		e.Mode = state.ReadMode

		edit.OpenLineBelow(e)
		edtest.WantLines(t, b, "a", "bb", "")
		if e.Row != 2 {
			t.Errorf("currentRow = %d, want 2", e.Row)
		}
	})
}

func TestDeletingTheLastRunePullsTheCursorBack(t *testing.T) {
	e := state.New()

	b := edtest.AtCursor(t, e, "abc\n", 0, 2)
	e.Mode = state.ReadMode

	edit.DeleteRune(e)
	e.ClampCol()

	edtest.WantLines(t, b, "ab")
	if e.Col != 1 {
		t.Errorf("currentCol = %d, want 1", e.Col)
	}
}
