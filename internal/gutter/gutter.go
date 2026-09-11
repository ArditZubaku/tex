// Package gutter is the column of line numbers down the left of a window.
package gutter

import "strconv"

const minGutterWidth = 4

// padLeft sets the whole gutter in from the window's left edge. The current
// line's number is ranged left where every other is ranged right — VIM's own
// arrangement — so without it that one number alone sits against the frame.
const padLeft = 1

// Width is how many columns the gutter takes for a buffer of that many lines.
func Width(lineCount int) int {
	digits := 1
	for n := max(lineCount, 1); n >= 10; n /= 10 {
		digits++
	}

	return padLeft + max(digits+1, minGutterWidth)
}

// A screen shows a few dozen distances and redraws all of them on every
// keystroke, so the labels are formatted once and kept. The bound is well past
// any terminal's height; a distance beyond it is formatted afresh.
const maxCached = 512

var (
	relative      []string
	relativeWidth int
)

// Label is VIM's `number` + `relativenumber` pair: the cursor's own line shows its
// absolute number, every other line its distance from the cursor.
func Label(row, cursorRow, width int) string {
	width -= padLeft
	if row == cursorRow {
		return pad(row+1, width, true, "")
	}

	distance := row - cursorRow
	if distance < 0 {
		distance = -distance
	}
	if distance >= maxCached {
		return pad(distance, width-1, false, " ")
	}

	if width != relativeWidth {
		relative, relativeWidth = relative[:0], width
	}
	for len(relative) <= distance {
		relative = append(relative, pad(len(relative), width-1, false, " "))
	}

	return relative[distance]
}

// pad is fmt's "%-*d" and "%*d" without the reflection: n in width columns,
// against the left or the right of them, with suffix after it and the gutter's
// own indent before it.
func pad(n, width int, left bool, suffix string) string {
	var numBuf [20]byte
	num := strconv.AppendInt(numBuf[:0], int64(n), 10)

	var outBuf [32]byte
	out := appendSpaces(outBuf[:0], padLeft)
	spaces := max(width-len(num), 0)
	if !left {
		out = appendSpaces(out, spaces)
	}
	out = append(out, num...)
	if left {
		out = appendSpaces(out, spaces)
	}

	return string(append(out, suffix...))
}

func appendSpaces(dst []byte, n int) []byte {
	for range n {
		dst = append(dst, ' ')
	}

	return dst
}
