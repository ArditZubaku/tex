package motion

import "testing"

func TestInnerWordFrom(t *testing.T) {
	cases := []struct {
		name      string
		content   string
		row, col  int
		wantStart int
		wantEnd   int
	}{
		{"mid-word", "foo bar\n", 0, 1, 0, 2},
		{"at word start", "foo bar\n", 0, 0, 0, 2},
		{"at word end", "foo bar\n", 0, 2, 0, 2},
		{"a run of punctuation", "foo::bar\n", 0, 3, 3, 4},
		{"a run of spaces", "foo   bar\n", 0, 4, 3, 5},
		{"a single-space run", "foo bar\n", 0, 3, 3, 3},
		{"a one-rune line", "a\n", 0, 0, 0, 0},
		{"an empty line has nothing to select", "\n", 0, 0, 0, -1},
		{"a column past the end clamps to the last rune", "abc\n", 0, 9, 0, 2},
		{"reads the row it is given, not another one", "xxxxxxxx\nfoo\n", 1, 1, 0, 2},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			start, end := InnerWordFrom(opened(t, tc.content), tc.row, tc.col)
			if start != tc.wantStart || end != tc.wantEnd {
				t.Errorf("InnerWordFrom(%d, %d) = %d,%d, want %d,%d",
					tc.row, tc.col, start, end, tc.wantStart, tc.wantEnd)
			}
		})
	}
}
