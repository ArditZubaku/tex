package edit_test

import (
	"testing"

	"github.com/ArditZubaku/tex/internal/editor/edtest"
	"github.com/ArditZubaku/tex/internal/editor/state"
)

func TestReplaceRune(t *testing.T) {
	e := state.New()

	cases := []struct {
		name    string
		content string
		col     int
		typed   string
		want    string
		wantCol int
	}{
		{"the rune under the cursor", "abc", 0, "rx", "xbc", 0},
		{"further along the line", "abc", 1, "rx", "axc", 1},
		{"the last rune of the line", "abc", 2, "rx", "abx", 2},
		{"a count replaces that many and lands on the last", "abcd", 0, "3rx", "xxxd", 2},
		{"a count reaching past the end replaces nothing", "abc", 0, "5rx", "abc", 0},
		{"an empty line has nothing to replace", "", 0, "rx", "", 0},
		{"a space is a rune like any other", "abc", 0, "r ", " bc", 0},
		{"a digit is the replacement rather than a count", "abc", 0, "r5", "5bc", 0},
		{"Esc cancels", "abc", 0, "r\x1b", "abc", 0},
		{"Enter has no rune to put in and cancels", "abc", 0, "r\n", "abc", 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := edtest.InReadMode(t, e, tc.content+"\n", 0, tc.col)

			edtest.Press(t, e, tc.typed)
			edtest.WantLines(t, b, tc.want)
			edtest.WantCursor(t, e, 0, tc.wantCol)
		})
	}
}

func TestReplaceUndoesInOneStep(t *testing.T) {
	e := state.New()
	b := edtest.InReadMode(t, e, "abcd\n", 0, 0)

	edtest.Press(t, e, "3rx")
	edtest.WantLines(t, b, "xxxd")

	e.Undo()

	edtest.WantLines(t, b, "abcd")
}

func TestReplaceLeavesTheKeysAfterItAlone(t *testing.T) {
	e := state.New()
	b := edtest.InReadMode(t, e, "abcd\n", 0, 0)

	edtest.Press(t, e, "3rxx")

	edtest.WantLines(t, b, "xxd")
	edtest.WantCursor(t, e, 0, 2)
}
