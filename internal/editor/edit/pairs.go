package edit

import "github.com/ArditZubaku/tex/internal/editor/state"

// closerFor is the bracket typing an opener puts after the cursor, and
// openerFor is that read the other way.
var closerFor = map[rune]rune{'(': ')', '[': ']', '{': '}'}

var openerFor = func() map[rune]rune {
	m := make(map[rune]rune, len(closerFor))
	for open, closed := range closerFor {
		m[closed] = open
	}
	return m
}()

// skipCloser is what stops typing the closing bracket of a pair the editor
// already wrote from leaving a second one behind: the cursor steps over it
// instead.
func skipCloser(e *state.Editor, ch rune) bool {
	if _, ok := openerFor[ch]; !ok {
		return false
	}

	at, ok := e.Buf.Rune(e.Row, e.Col)
	if !ok || at != ch {
		return false
	}

	e.Col++
	return true
}

// pairAt says whether the two runes from col are a pair the editor would have
// written itself, so that Backspace between them takes both.
func pairAt(e *state.Editor, col int) bool {
	open, ok := e.Buf.Rune(e.Row, col)
	if !ok {
		return false
	}

	closed, ok := e.Buf.Rune(e.Row, col+1)
	return ok && closerFor[open] == closed
}
