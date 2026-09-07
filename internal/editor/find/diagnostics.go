package find

import (
	"github.com/ArditZubaku/tex/internal/editor/diag"
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
