// Package motion is VIM's word motions: where w, e and b land, read off the
// buffer and the character classes either side of the cursor.
package motion

import (
	"github.com/ArditZubaku/tex/internal/buffer"
	"github.com/ArditZubaku/tex/internal/chars"
)

// at treats past-end-of-line as whitespace, so word motions see line
// breaks as word boundaries without special-casing them separately.
func at(b *buffer.Buffer, row, col int) rune {
	ch, ok := b.Rune(row, col)
	if !ok {
		return ' '
	}
	return ch
}

func forward(b *buffer.Buffer, row, col int) (int, int) {
	if col < b.RuneLen(row) {
		return row, col + 1
	}
	if row < b.LineCount()-1 {
		return row + 1, 0
	}
	return row, col
}

func NextWordFrom(b *buffer.Buffer, row, col int) (int, int) {
	startClass := chars.ClassOf(at(b, row, col))

	if startClass != chars.Space {
		for chars.ClassOf(at(b, row, col)) == startClass {
			nr, nc := forward(b, row, col)
			if nr == row && nc == col {
				break
			}
			row, col = nr, nc
		}
	}

	for chars.ClassOf(at(b, row, col)) == chars.Space {
		nr, nc := forward(b, row, col)
		if nr == row && nc == col {
			break
		}
		row, col = nr, nc
	}

	return row, col
}

func EndOfWordFrom(b *buffer.Buffer, row, col int) (int, int) {
	// always advance at least one position, so pressing 'e' at the end of
	// a word moves to the end of the next one instead of staying put
	row, col = forward(b, row, col)

	for chars.ClassOf(at(b, row, col)) == chars.Space {
		nr, nc := forward(b, row, col)
		if nr == row && nc == col {
			break
		}
		row, col = nr, nc
	}

	wordClass := chars.ClassOf(at(b, row, col))
	for {
		nr, nc := forward(b, row, col)
		if (nr == row && nc == col) || chars.ClassOf(at(b, nr, nc)) != wordClass {
			break
		}
		row, col = nr, nc
	}

	return row, col
}

func backward(b *buffer.Buffer, row, col int) (int, int) {
	if col > 0 {
		return row, col - 1
	}
	if row > 0 {
		return row - 1, b.RuneLen(row - 1)
	}
	return row, col
}

func PrevWordFrom(b *buffer.Buffer, row, col int) (int, int) {
	// step off the current word first, so pressing 'b' from a word-start
	// lands on the previous word instead of itself
	row, col = backward(b, row, col)

	for chars.ClassOf(at(b, row, col)) == chars.Space {
		nr, nc := backward(b, row, col)
		if nr == row && nc == col {
			break
		}
		row, col = nr, nc
	}

	wordClass := chars.ClassOf(at(b, row, col))
	for {
		pr, pc := backward(b, row, col)
		if (pr == row && pc == col) || chars.ClassOf(at(b, pr, pc)) != wordClass {
			break
		}
		row, col = pr, pc
	}

	return row, col
}
