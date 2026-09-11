package motion

import "github.com/ArditZubaku/tex/internal/buffer"

// The pairs '%' walks, which are VIM's own `matchpairs` and nothing more. A
// quote is not among them: both halves of one are the same rune, so there is no
// telling from the rune alone which half the cursor is on.
var closerFor = map[rune]rune{'(': ')', '[': ']', '{': '}'}

var openerFor = func() map[rune]rune {
	m := make(map[rune]rune, len(closerFor))
	for open, closed := range closerFor {
		m[closed] = open
	}

	return m
}()

// MatchFrom is where VIM's '%' lands: the match of the bracket under the
// cursor, or of the first one after it on the same line. It counts nesting and
// nothing else — a bracket inside a string or a comment is a bracket, which is
// what VIM's own '%' does before a plugin teaches it otherwise.
func MatchFrom(b *buffer.Buffer, row, col int) (int, int, bool) {
	at, ch, ok := bracketFrom(b, row, col)
	if !ok {
		return 0, 0, false
	}

	if closer, isOpen := closerFor[ch]; isOpen {
		return scan(b, row, at, ch, closer, 1)
	}

	return scan(b, row, at, ch, openerFor[ch], -1)
}

// The search for one to work from stops at the end of the line, which is what
// leaves '%' on a line with no bracket on it doing nothing at all.
func bracketFrom(b *buffer.Buffer, row, col int) (int, rune, bool) {
	line := b.Line(row)
	for at := max(col, 0); at < len(line); at++ {
		if isBracket(line[at]) {
			return at, line[at], true
		}
	}

	return 0, 0, false
}

func isBracket(ch rune) bool {
	_, opens := closerFor[ch]
	_, closes := openerFor[ch]

	return opens || closes
}

// scan walks a rune at a time from the bracket itself, so the depth it starts
// counting from is that bracket's own.
func scan(b *buffer.Buffer, row, col int, from, want rune, step int) (int, int, bool) {
	for depth := 0; ; {
		switch ch, _ := b.Rune(row, col); ch {
		case from:
			depth++
		case want:
			depth--
			if depth == 0 {
				return row, col, true
			}
		}

		next, at, ok := walk(b, row, col, step)
		if !ok {
			return 0, 0, false
		}
		row, col = next, at
	}
}

// walk is one rune along, over the line break at either end of a line. An empty
// line has no rune to stop on and is stepped straight through, since Rune says
// so and the next walk moves off it.
func walk(b *buffer.Buffer, row, col, step int) (int, int, bool) {
	if col += step; col >= 0 && col < b.RuneLen(row) {
		return row, col, true
	}

	if step > 0 {
		if row+1 >= b.LineCount() {
			return 0, 0, false
		}

		return row + 1, 0, true
	}
	if row == 0 {
		return 0, 0, false
	}

	return row - 1, max(b.RuneLen(row-1)-1, 0), true
}
