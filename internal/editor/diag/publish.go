package diag

import (
	"github.com/ArditZubaku/tex/internal/editor/view"
	"github.com/ArditZubaku/tex/internal/lsp"
)

// Publish is what a server said about one file, turned into the editor's own
// coordinates. The turning happens here rather than in the client because
// counting columns needs the text they were counted in, and reading a line
// moves a buffer's window: only the loop's goroutine may, and this is it.
func Publish(path string, notes []lsp.Diagnostic) {
	if len(notes) == 0 {
		Set(path, nil)

		return
	}

	text, done := view.TextOf(path)
	if text == nil {
		Set(path, nil)

		return
	}
	defer done()

	encoding := lsp.PositionEncoding(path)
	file := make(File, 0, len(notes))
	for _, note := range notes {
		row, col := encoding.RowCol(text, note.Range.Start)
		endRow, endCol := encoding.RowCol(text, note.Range.End)
		file = append(file, Note{
			Severity: severityOf(note.Severity),
			Message:  note.Message,
			Source:   note.Source,
			Row:      row,
			Col:      col,
			EndRow:   endRow,
			EndCol:   endCol,
		})
	}

	Set(path, file)
}

// A server that did not say how bad it is means it to be seen.
func severityOf(severity int) Severity {
	if severity < int(Error) || severity > int(Hint) {
		return Error
	}

	return Severity(severity)
}
