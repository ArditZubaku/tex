package render

import (
	"github.com/nsf/termbox-go"

	"github.com/ArditZubaku/tex/internal/editor/state"
)

// lineColors is the buffer's own line asked of the language in use, in the
// palette in use.
func lineColors(e *state.Editor, line []rune, inBlock bool) ([]termbox.Attribute, bool) {
	return e.Lang.LineColors(line, inBlock, &e.Palette)
}

// A block comment opened above the window still colours the top of it, so the
// state has to be lexed rather than assumed. Walking back to the start of the
// file would make a redraw cost the whole buffer, so the search is bounded:
// past blockLookback lines the window is taken to start outside a comment.
const blockLookback = 64

func blockStateBefore(e *state.Editor, row int) bool {
	if !e.Lang.HasBlockComments() {
		return false
	}

	inBlock := false
	for i := max(row-blockLookback, 0); i < row; i++ {
		inBlock = e.Lang.Highlight(e.Buf.Line(i), inBlock, nil, &e.Palette)
	}

	return inBlock
}
