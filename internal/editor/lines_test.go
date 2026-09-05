package editor

import (
	"testing"

	"github.com/ArditZubaku/tex/internal/editor/edit"
	"github.com/ArditZubaku/tex/internal/editor/edtest"
	"github.com/ArditZubaku/tex/internal/editor/state"
)

func TestEnter(t *testing.T) {
	t.Run("splits in Edit mode", func(t *testing.T) {
		b := edtest.AtCursor(t, ed, "abcd\nlast\n", 0, 2)
		ed.Mode = state.EditMode

		edit.Enter(ed)
		edtest.WantLines(t, b, "ab", "cd", "last")
		if ed.Row != 1 || ed.Col != 0 || !ed.Modified {
			t.Errorf("cursor at %d,%d, modified %v", ed.Row, ed.Col, ed.Modified)
		}
	})

	t.Run("moves down in Read mode", func(t *testing.T) {
		b := edtest.AtCursor(t, ed, "abcd\nlast\n", 0, 2)
		ed.Mode = state.ReadMode

		edit.Enter(ed)
		edtest.WantLines(t, b, "abcd", "last")
		if ed.Row != 1 || ed.Modified {
			t.Errorf("currentRow = %d, modified %v", ed.Row, ed.Modified)
		}
	})

	t.Run("split then backspace is a round trip", func(t *testing.T) {
		b := edtest.AtCursor(t, ed, "abcd\n", 0, 2)
		ed.Mode = state.EditMode

		edit.Enter(ed)
		edit.Backspace(ed)
		edtest.WantLines(t, b, "abcd")
		if ed.Row != 0 || ed.Col != 2 {
			t.Errorf("cursor at %d,%d, want 0,2", ed.Row, ed.Col)
		}
	})
}

func TestOpenLineOperators(t *testing.T) {
	t.Run("o opens below", func(t *testing.T) {
		b := edtest.AtCursor(t, ed, "a\nbb\n", 0, 1)
		ed.Mode = state.ReadMode

		edit.OpenLineBelow(ed)
		edtest.WantLines(t, b, "a", "", "bb")
		if ed.Row != 1 || ed.Col != 0 || ed.Mode != state.EditMode || !ed.Modified {
			t.Errorf("cursor at %d,%d, mode %v, modified %v", ed.Row, ed.Col, ed.Mode, ed.Modified)
		}
	})

	t.Run("O opens above", func(t *testing.T) {
		b := edtest.AtCursor(t, ed, "a\nbb\n", 1, 2)
		ed.Mode = state.ReadMode

		edit.OpenLineAbove(ed)
		edtest.WantLines(t, b, "a", "", "bb")
		if ed.Row != 1 || ed.Col != 0 || ed.Mode != state.EditMode {
			t.Errorf("cursor at %d,%d, mode %v", ed.Row, ed.Col, ed.Mode)
		}
	})

	t.Run("o on the last line", func(t *testing.T) {
		b := edtest.AtCursor(t, ed, "a\nbb\n", 1, 0)
		ed.Mode = state.ReadMode

		edit.OpenLineBelow(ed)
		edtest.WantLines(t, b, "a", "bb", "")
		if ed.Row != 2 {
			t.Errorf("currentRow = %d, want 2", ed.Row)
		}
	})
}
