// Package gutter is the column of line numbers down the left of a window.
package gutter

import "fmt"

const minGutterWidth = 4

// Width is how many columns the gutter takes for a buffer of that many lines.
func Width(lineCount int) int {
	digits := 1
	for n := max(lineCount, 1); n >= 10; n /= 10 {
		digits++
	}

	return max(digits+1, minGutterWidth)
}

// Label is VIM's `number` + `relativenumber` pair: the cursor's own line shows its
// absolute number, every other line its distance from the cursor.
func Label(row, cursorRow, width int) string {
	if row == cursorRow {
		return fmt.Sprintf("%-*d", width, row+1)
	}

	distance := row - cursorRow
	if distance < 0 {
		distance = -distance
	}

	return fmt.Sprintf("%*d ", width-1, distance)
}
