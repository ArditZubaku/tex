package gutter

import (
	"fmt"
	"strings"
	"testing"
)

func TestGutterWidth(t *testing.T) {
	cases := []struct {
		name  string
		lines int
		want  int
	}{
		{"empty buffer", 0, padLeft + minGutterWidth},
		{"one line", 1, padLeft + minGutterWidth},
		{"under the minimum", 999, padLeft + minGutterWidth},
		{"four digits", 1000, padLeft + 5},
		{"six digits", 202000, padLeft + 7},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Width(tc.lines); got != tc.want {
				t.Errorf("Width(%d) = %d, want %d", tc.lines, got, tc.want)
			}
		})
	}
}

func TestLineNumberLabel(t *testing.T) {
	cases := []struct {
		name           string
		row, cursorRow int
		width          int
		want           string
	}{
		{"the cursor's line is absolute and left-aligned", 41, 41, 5, " 42  "},
		{"a line above counts up", 39, 41, 5, "   2 "},
		{"a line below counts up too", 44, 41, 5, "   3 "},
		{"the first line, cursor on it", 0, 0, 5, " 1   "},
		{"a wider gutter pads further", 8, 41, 8, "     33 "},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Label(tc.row, tc.cursorRow, tc.width)
			if got != tc.want {
				t.Errorf("Label(%d, %d, %d) = %q, want %q", tc.row, tc.cursorRow, tc.width, got, tc.want)
			}
		})
	}
}

func TestLineNumberLabelFillsTheGutter(t *testing.T) {
	for _, lines := range []int{1, 9, 10, 999, 1000, 202000} {
		width := Width(lines)

		for _, row := range []int{0, lines / 2, lines - 1} {
			for _, cursorRow := range []int{0, lines / 2, lines - 1} {
				if got := len(Label(row, cursorRow, width)); got != width {
					t.Errorf("len(Label(%d, %d, %d)) = %d, want %d", row, cursorRow, width, got, width)
				}
			}
		}
	}
}

func referenceLabel(row, cursorRow, width int) string {
	indent := strings.Repeat(" ", padLeft)
	width -= padLeft
	if row == cursorRow {
		return indent + fmt.Sprintf("%-*d", width, row+1)
	}

	distance := row - cursorRow
	if distance < 0 {
		distance = -distance
	}

	return indent + fmt.Sprintf("%*d ", width-1, distance)
}

func TestLabelMatchesTheFormatItReplaced(t *testing.T) {
	for _, width := range []int{4, 5, 7, 9, 4} {
		for _, cursorRow := range []int{0, 7, 5000, 199999} {
			rows := []int{
				0, 1, 6, 8,
				cursorRow, cursorRow + 1,
				cursorRow + maxCached - 1, cursorRow + maxCached, cursorRow + maxCached + 3,
				1234567,
			}
			for _, row := range rows {
				want := referenceLabel(row, cursorRow, width)
				if got := Label(row, cursorRow, width); got != want {
					t.Errorf("Label(%d, %d, %d) = %q, want %q", row, cursorRow, width, got, want)
				}
			}
		}
	}
}

func TestWidthMatchesTheDigitCount(t *testing.T) {
	for lines := range 2000 {
		want := padLeft + max(len(fmt.Sprintf("%d", max(lines, 1)))+1, minGutterWidth)
		if got := Width(lines); got != want {
			t.Fatalf("Width(%d) = %d, want %d", lines, got, want)
		}
	}
	for _, lines := range []int{9999, 10000, 999999, 1000000, 1 << 40} {
		want := padLeft + max(len(fmt.Sprintf("%d", lines))+1, minGutterWidth)
		if got := Width(lines); got != want {
			t.Errorf("Width(%d) = %d, want %d", lines, got, want)
		}
	}
}

func TestEveryLabelIsSetInFromTheLeftEdge(t *testing.T) {
	indent := strings.Repeat(" ", padLeft)

	for _, lines := range []int{1, 9, 10, 999, 1000, 202000} {
		width := Width(lines)

		for _, row := range []int{0, lines / 2, lines - 1} {
			for _, cursorRow := range []int{0, lines / 2, lines - 1} {
				if got := Label(row, cursorRow, width); !strings.HasPrefix(got, indent) {
					t.Errorf("Label(%d, %d, %d) = %q, want it set in by %d", row, cursorRow, width, got, padLeft)
				}
			}
		}
	}
}
