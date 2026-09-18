package edit

import (
	"slices"

	"github.com/ArditZubaku/tex/internal/editor/complete"
	"github.com/ArditZubaku/tex/internal/editor/state"
)

// Accept is a candidate settled on: what has been typed of the word is replaced
// by what the server offered, and the edits it named elsewhere in the file go
// in with it. Those are what an import is — a server offers a name out of a
// package the file does not import yet, and without the line it asks for
// alongside, the name it just wrote is a compile error.
//
// A candidate that is called carries its own parentheses in, with the cursor
// left between them where the arguments go.
func Accept(e *state.Editor, item complete.Item) {
	row := e.Row
	from := min(max(item.From, 0), e.Buf.RuneLen(row))
	to := min(max(e.Col, from), e.Buf.RuneLen(row))

	line := e.Buf.Line(row)
	text := []rune(item.Text)
	call := item.Call && !alreadyCalled(line, to, text)
	if call {
		text = append(text, '(', ')')
	}

	e.TouchLine(row)
	e.Buf.SetLine(row, slices.Replace(slices.Clone(line), from, to, text...))
	e.Row, e.Col = row, from+len(text)
	if call {
		e.Col--
	}
	e.Modified = true

	Elsewhere(e, item.Extra)
}

// alreadyCalled is the parentheses being there to write into already: the name
// in front of a call being changed, or a server that sent them itself.
func alreadyCalled(line []rune, at int, text []rune) bool {
	if n := len(text); n > 0 && (text[n-1] == '(' || text[n-1] == ')') {
		return true
	}

	return at < len(line) && line[at] == '('
}

// Elsewhere is the edits a candidate needs away from the word itself. It is its
// own entry because a server may hold them back until the candidate is settled
// on and answer with them a frame or two after the name has already gone in.
func Elsewhere(e *state.Editor, edits []complete.Edit) { splice(e, edits, true) }
