package edit

import "testing"

func TestSelectionCovers(t *testing.T) {
	charwise := Span{startRow: 0, startCol: 1, endRow: 1, endCol: 2, Active: true}
	oneLine := Span{startRow: 0, startCol: 1, endRow: 0, endCol: 2, Active: true}
	byLine := Span{startRow: 0, startCol: 3, endRow: 0, endCol: 1, linewise: true, Active: true}

	cases := []struct {
		name     string
		sel      Span
		row, col int
		lineLen  int
		want     bool
	}{
		{"before the start", charwise, 0, 0, 3, false},
		{"at the start", charwise, 0, 1, 3, true},
		{"the line break it runs over", charwise, 0, 3, 3, true},
		{"past the line break", charwise, 0, 4, 3, false},
		{"the head of its last line", charwise, 1, 0, 3, true},
		{"at its end", charwise, 1, 2, 3, true},
		{"past its end", charwise, 1, 3, 3, false},
		{"a row below it", charwise, 2, 0, 3, false},
		{"a run inside one line", oneLine, 0, 2, 3, true},
		{"the line break it stops before", oneLine, 0, 3, 3, false},
		{"a line taken whole", byLine, 0, 0, 3, true},
		{"the break of a line taken whole", byLine, 0, 3, 3, true},
		{"an empty line taken whole", byLine, 0, 0, 0, true},
		{"no Span at all", Span{}, 0, 0, 3, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.sel.Covers(tc.row, tc.col, tc.lineLen); got != tc.want {
				t.Errorf("Covers(%d, %d, %d) = %v, want %v", tc.row, tc.col, tc.lineLen, got, tc.want)
			}
		})
	}
}
