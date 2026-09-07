package find

import (
	"fmt"
	"path/filepath"

	"github.com/ArditZubaku/tex/internal/editor/diag"
	"github.com/ArditZubaku/tex/internal/editor/picker"
	"github.com/ArditZubaku/tex/internal/editor/state"
)

// ']d' and '[d' walk what a server said about the file being edited, wrapping
// at either end the way 'n' and 'N' wrap a search — and, like them, neither
// remembering the jump nor centring the window: this is a walk through one
// file, not a jump to somewhere else in the project.

func NextDiagnostic(e *state.Editor) { goToDiagnostic(e, diag.Next) }
func PrevDiagnostic(e *state.Editor) { goToDiagnostic(e, diag.Prev) }

func goToDiagnostic(e *state.Editor, at func(path string, row, col int) (diag.Note, bool)) {
	note, ok := at(e.SourceFile, e.Row, e.Col)
	if !ok {
		e.StatusMsg = "No diagnostics in this file"

		return
	}

	e.Row, e.Col = min(note.Row, e.Buf.LineCount()-1), note.Col
	e.ClampCol()
}

// ':diag' is everything a server said about the project, worst first, in the
// same popup the references and the symbols use. It reaches past the buffer
// list on purpose: a server reports the package around a broken file, and those
// files are usually the ones worth opening.
func OpenDiagnostics(e *state.Editor) {
	found := diag.All()
	if len(found) == 0 {
		e.StatusMsg = "No diagnostics"

		return
	}

	entries := make([]picker.Entry, 0, len(found))
	for _, at := range found {
		entries = append(entries, picker.Entry{
			Label: fmt.Sprintf("%s %s:%d  %s",
				severityMark(at.Severity), filepath.Base(at.Path), at.Row+1, at.Message),
			Path: at.Path,
			Row:  at.Row,
			Col:  at.Col,
		})
	}

	showPicker(e, "Diagnostics", entries)
}

func severityMark(severity diag.Severity) string {
	switch severity {
	case diag.Error:
		return "E"
	case diag.Warning:
		return "W"
	case diag.Info:
		return "I"
	default:
		return "H"
	}
}
