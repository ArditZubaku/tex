package find

import (
	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/ArditZubaku/tex/internal/editor/view"
	"github.com/ArditZubaku/tex/internal/lsp"
)

// What a language server answers arrives some frames after it was asked for,
// into an editor that may have moved on since. asked is the newest lookup: an
// answer to anything older belongs to a command already given up on, and
// acting on it would move the cursor — or throw a popup up — over whatever was
// started instead. Nothing is drawn in the meantime: a server answers a warm
// lookup in tens of milliseconds, and a message that appears for that long is
// a flicker rather than news.
var asked int

func ask(e *state.Editor) (int, state.Jump) {
	asked++

	return asked, state.Jump{Path: e.SourceFile, Row: e.Row, Col: e.Col}
}

func stale(e *state.Editor, token int, from state.Jump) bool {
	return token != asked ||
		e.SourceFile != from.Path || e.Row != from.Row || e.Col != from.Col
}

// Reset is one test not being answered out of another one's lookups.
func Reset() { asked = 0 }

// place is a single answer as somewhere to go, which is what 'gd' has. A file
// that is no longer there is no answer at all, and the caller falls back to the
// text the way it does when there is no server.
func place(found lsp.Location) (string, int, int, bool) {
	path := lsp.Path(found.URI)
	text, done := view.TextOf(path)
	if text == nil {
		return "", 0, 0, false
	}
	defer done()

	row, col := lsp.PositionEncoding().RowCol(text, found.Range.Start)

	return path, row, col, true
}
