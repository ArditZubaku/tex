package render

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ArditZubaku/tex/internal/buffer"
	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/ArditZubaku/tex/internal/syntax"
)

func writeTemp(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "f.txt")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	return path
}

func TestLineColorsWithoutSyntax(t *testing.T) {
	e := state.New()
	e.Lang = nil
	defer func() { e.Lang = nil }()

	got, open := lineColors(e, []rune("var x int"), true)
	if got != nil || open {
		t.Errorf("lineColors with no lang = %v, %v, want nil, false", got, open)
	}
}

func TestBlockStateBefore(t *testing.T) {
	e := state.New()
	e.Lang = syntax.Detect("x.go")
	defer func() { e.Lang = nil }()

	e.Buf = buffer.Open(writeTemp(t, "a\n/* open\nstill\n*/ b\nc\n"))
	defer e.Buf.Close()

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
			if got := blockStateBefore(e, tc.row); got != tc.want {
				t.Errorf("blockStateBefore(e, %d) = %v, want %v", tc.row, got, tc.want)
			}
		})
	}
}

// Past the look-back bound the window is assumed to start outside a comment,
// which is what keeps a redraw from lexing the whole file.
func TestBlockStateBeforeIsBounded(t *testing.T) {
	e := state.New()
	e.Lang = syntax.Detect("x.go")
	defer func() { e.Lang = nil }()

	lines := "/* open\n" + strings.Repeat("still\n", blockLookback+10)
	e.Buf = buffer.Open(writeTemp(t, lines))
	defer e.Buf.Close()

	if !blockStateBefore(e, blockLookback) {
		t.Error("blockStateBefore within the bound = false, want true")
	}
	if blockStateBefore(e, blockLookback+2) {
		t.Error("blockStateBefore past the bound = true, want false")
	}
}
