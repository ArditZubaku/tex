package editor

import (
	"strings"
	"testing"

	"github.com/ArditZubaku/tex/internal/buffer"
	"github.com/ArditZubaku/tex/internal/syntax"
)

func TestLineColorsWithoutSyntax(t *testing.T) {
	ed.Lang = nil
	defer func() { ed.Lang = nil }()

	got, open := lineColors([]rune("var x int"), true)
	if got != nil || open {
		t.Errorf("lineColors with no lang = %v, %v, want nil, false", got, open)
	}
}

func TestBlockStateBefore(t *testing.T) {
	ed.Lang = syntax.Detect("x.go")
	defer func() { ed.Lang = nil }()

	ed.Buf = buffer.Open(writeTemp(t, "a\n/* open\nstill\n*/ b\nc\n"))
	defer ed.Buf.Close()

	cases := []struct {
		name string
		row  int
		want bool
	}{
		{"the first line starts outside", 0, false},
		{"the line after the opener is inside", 2, true},
		{"the closing line is still inside", 3, true},
		{"the line after the closer is outside", 4, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := blockStateBefore(tc.row); got != tc.want {
				t.Errorf("blockStateBefore(%d) = %v, want %v", tc.row, got, tc.want)
			}
		})
	}
}

// Past the look-back bound the window is assumed to start outside a comment,
// which is what keeps a redraw from lexing the whole file.
func TestBlockStateBeforeIsBounded(t *testing.T) {
	ed.Lang = syntax.Detect("x.go")
	defer func() { ed.Lang = nil }()

	lines := "/* open\n" + strings.Repeat("still\n", blockLookback+10)
	ed.Buf = buffer.Open(writeTemp(t, lines))
	defer ed.Buf.Close()

	if !blockStateBefore(blockLookback) {
		t.Error("blockStateBefore within the bound = false, want true")
	}
	if blockStateBefore(blockLookback + 2) {
		t.Error("blockStateBefore past the bound = true, want false")
	}
}
