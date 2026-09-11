package edit_test

import (
	"testing"

	"github.com/nsf/termbox-go"

	"github.com/ArditZubaku/tex/internal/editor/edtest"
	"github.com/ArditZubaku/tex/internal/editor/state"
)

func TestAutoPairs(t *testing.T) {
	e := state.New()

	cases := []struct {
		name    string
		content string
		col     int
		typed   string
		want    string
		wantCol int
	}{
		{"( closes itself", "", 0, "(", "()", 1},
		{"[ closes itself", "", 0, "[", "[]", 1},
		{"{ closes itself", "", 0, "{", "{}", 1},
		{"nested openers nest their closers", "", 0, "([{", "([{}])", 3},
		{"the closer goes in ahead of what follows", "foo", 0, "(", "()foo", 1},
		{"typing the closer steps over it", "", 0, "()", "()", 2},
		{"typing a closer with nothing to skip inserts it", "", 0, ")", ")", 1},
		{"a closer only skips its own kind", "]", 0, ")", ")]", 1},
		{"text between the pair keeps the closer", "", 0, "(ab", "(ab)", 3},
		{"a quote closes itself", "", 0, `"`, `""`, 1},
		{"typing the closing quote steps over it", "", 0, `""`, `""`, 2},
		{"a quote pairs inside brackets", "", 0, `("`, `("")`, 2},
		{"text between the quotes keeps the closer", "", 0, `"hi`, `"hi"`, 3},
		{"backspace between a pair takes both", "", 0, "(\b", "", 0},
		{"backspace between quotes takes both", "", 0, "\"\b", "", 0},
		{"backspace deletes one rune when the pair is not adjacent", "", 0, "(x\b", "()", 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := edtest.InReadMode(t, e, tc.content+"\n", 0, tc.col)
			e.Mode = state.EditMode

			edtest.Press(t, e, tc.typed)
			edtest.WantLines(t, b, tc.want)
			if e.Col != tc.wantCol {
				t.Errorf("cursor at col %d, want %d", e.Col, tc.wantCol)
			}
		})
	}
}

func TestAutoPairUndoesInOneStep(t *testing.T) {
	e := state.New()
	b := edtest.InReadMode(t, e, "\n", 0, 0)

	edtest.Press(t, e, "i(")
	edtest.PressKey(t, e, termbox.KeyEsc)
	e.Undo()

	edtest.WantLines(t, b, "")
}
