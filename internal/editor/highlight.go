package editor

import (
	"github.com/nsf/termbox-go"
)

// lineColors is the buffer's own line asked of the language in use, in the
// palette in use.
func lineColors(line []rune, inBlock bool) ([]termbox.Attribute, bool) {
	return lang.LineColors(line, inBlock, &active)
}

// A block comment opened above the window still colours the top of it, so the
// state has to be lexed rather than assumed. Walking back to the start of the
// file would make a redraw cost the whole buffer, so the search is bounded:
// past blockLookback lines the window is taken to start outside a comment.
const blockLookback = 64

func blockStateBefore(row int) bool {
	if !lang.HasBlockComments() {
		return false
	}

	inBlock := false
	for i := max(row-blockLookback, 0); i < row; i++ {
		inBlock = lang.Highlight(buf.Line(i), inBlock, nil, &active)
	}

	return inBlock
}
