package command

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/ArditZubaku/tex/internal/editor/view"
)

// QuitPrompt is the delimiter the quit confirmation is told apart by once Enter
// settles it, the way ':' tells an ex command from a search.
const QuitPrompt = 'Q'

// The status line is one row, so a list long enough to run off it is cut to a
// count, which says more than half a list does.
const namesShown = 3

// Quit is 'q' and ':q' on the last window. Every buffer holding changes that
// are not on disk is asked about, not the one on screen alone: a file left
// behind by Tab or by the picker holds its changes just the same, and quitting
// is the last chance to write them.
func Quit(e *state.Editor) {
	unsaved := view.UnsavedBuffers(e)
	if len(unsaved) == 0 {
		e.Close()
		return
	}

	e.StartLabelledPrompt(QuitPrompt, unsavedLabel(unsaved), "")
}

func unsavedLabel(unsaved []*view.Entry) string {
	names := make([]string, 0, namesShown)
	for _, entry := range unsaved[:min(len(unsaved), namesShown)] {
		names = append(names, filepath.Base(entry.Path))
	}

	list := strings.Join(names, ", ")
	if more := len(unsaved) - len(names); more > 0 {
		list += fmt.Sprintf(" and %d more", more)
	}

	return "save changes to " + list + "? [y]es/[n]o/[c]ancel "
}

// ConfirmQuit is what Enter does with the answer. A write that fails keeps the
// editor open: the changes it could not put on disk are still the only copy,
// and quitting on top of that error would be the one outcome nothing can undo.
func ConfirmQuit(e *state.Editor, input string) {
	switch strings.ToLower(strings.TrimSpace(input)) {
	case "y", "yes":
		if WriteAll(e) {
			e.Close()
		}
	case "n", "no":
		e.Close()
	default:
		e.StatusMsg = "quit cancelled"
	}
}
