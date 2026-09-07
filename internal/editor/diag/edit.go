package diag

import (
	"slices"

	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/ArditZubaku/tex/internal/lsp"
)

// Edited moves what was said about a file along with the lines it was said
// about. A server republishes a fraction of a second after a change, and until
// it does the choice is between drawing underlines under the wrong text — the
// single thing that makes an integration look broken — and flushing the file on
// the first keystroke, which flickers the whole screen on every keypress.
// Moving them is neither: every underline but the one on the line being typed
// stays exactly where it belongs, including the ones about to be fixed next.
func Edited(path string, row int, what state.EditKind) {
	key := lsp.FileURI(path)
	notes, ok := files[key]
	if !ok {
		return
	}

	switch what {
	case state.FileRewritten:
		delete(files, key)

		return
	case state.LineInserted:
		notes = shifted(notes, row, 1)
	case state.LineDeleted:
		notes = shifted(dropped(notes, row), row+1, -1)
	case state.LineChanged:
		notes = dropped(notes, row)
	}

	if len(notes) == 0 {
		delete(files, key)

		return
	}
	files[key] = notes
}

// shifted moves every note from a row down, and stretches the ones that start
// above it and reach past it rather than moving those.
func shifted(notes File, from, by int) File {
	for at := range notes {
		note := &notes[at]
		switch {
		case note.Row >= from:
			note.Row, note.EndRow = note.Row+by, note.EndRow+by
		case note.EndRow >= from:
			note.EndRow += by
		}
	}

	return notes
}

// A line's runes may have moved anywhere along it, so everything said about it
// goes: an underline three runes out is worse than none, and this is the line
// being typed on, where it is already plain that something is unfinished.
func dropped(notes File, row int) File {
	return slices.DeleteFunc(notes, func(note Note) bool {
		return row >= note.Row && row <= note.EndRow
	})
}
