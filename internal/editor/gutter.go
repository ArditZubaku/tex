package editor

import "fmt"

const minGutterWidth = 4

func gutterWidth(lineCount int) int {
	digits := len(fmt.Sprintf("%d", max(lineCount, 1)))
	return max(digits+1, minGutterWidth)
}

// VIM's `number` + `relativenumber` pair: the cursor's own line shows its
// absolute number, every other line its distance from the cursor.
func lineNumberLabel(row, cursorRow, width int) string {
	if row == cursorRow {
		return fmt.Sprintf("%-*d", width, row+1)
	}

	distance := row - cursorRow
	if distance < 0 {
		distance = -distance
	}

	return fmt.Sprintf("%*d ", width-1, distance)
}
