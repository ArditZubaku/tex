package motion

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ArditZubaku/tex/internal/buffer"
)

func opened(t *testing.T, content string) *buffer.Buffer {
	t.Helper()

	path := filepath.Join(t.TempDir(), "match.txt")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	b := buffer.Open(path)
	t.Cleanup(b.Close)

	return b
}

func TestMatchFrom(t *testing.T) {
	cases := []struct {
		name      string
		content   string
		row, col  int
		wantRow   int
		wantCol   int
		wantFound bool
	}{
		{"an opener finds its closer", "(a)\n", 0, 0, 0, 2, true},
		{"a closer finds its opener", "(a)\n", 0, 2, 0, 0, true},
		{"nesting is counted", "((a))\n", 0, 0, 0, 4, true},
		{"the inner pair from inside", "((a))\n", 0, 1, 0, 3, true},
		{"square brackets", "x[1]\n", 0, 1, 0, 3, true},
		{"braces", "f{}\n", 0, 1, 0, 2, true},
		{"the first bracket after the cursor is the one used", "ab(c)\n", 0, 0, 0, 4, true},
		{"a line with no bracket matches nothing", "abc\n", 0, 0, 0, 0, false},
		{"nothing past the cursor on the line", "(a) b\n", 0, 4, 0, 0, false},
		{"across lines", "f(\n\ta,\n)\n", 0, 1, 2, 0, true},
		{"back across lines", "f(\n\ta,\n)\n", 2, 0, 0, 1, true},
		{"an unmatched opener finds nothing", "(a\nb\n", 0, 0, 0, 0, false},
		{"an unmatched closer finds nothing", "a\nb)\n", 1, 1, 0, 0, false},
		{"a different kind in between is not the match", "([)]\n", 0, 0, 0, 2, true},
		{"an empty line in between is stepped over", "{\n\n}\n", 0, 0, 2, 0, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			row, col, ok := MatchFrom(opened(t, tc.content), tc.row, tc.col)
			if ok != tc.wantFound {
				t.Fatalf("MatchFrom(%d, %d) found = %v, want %v", tc.row, tc.col, ok, tc.wantFound)
			}
			if ok && (row != tc.wantRow || col != tc.wantCol) {
				t.Errorf("MatchFrom(%d, %d) = %d,%d, want %d,%d",
					tc.row, tc.col, row, col, tc.wantRow, tc.wantCol)
			}
		})
	}
}
