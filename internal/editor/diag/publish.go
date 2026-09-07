package diag

import (
	"os"

	"github.com/ArditZubaku/tex/internal/buffer"
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

	text, done := textOf(path)
	if text == nil {
		Set(path, nil)

		return
	}
	defer done()

	encoding := lsp.PositionEncoding()
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

// The text to count columns in: what the editor holds if it holds the file, and
// otherwise the file itself. Most publishes are about files no buffer holds — a
// server reports the whole package around a broken one — and those are the
// files ':diag' is for.
//
// The stat is not redundant: opening a file that is not there logs, and the log
// goes to the terminal termbox owns.
func textOf(path string) (*buffer.Buffer, func()) {
	if entry := view.Buffer(path); entry != nil {
		return entry.Buf, func() {}
	}
	if _, err := os.Stat(path); err != nil {
		return nil, nil
	}

	text := buffer.Open(path)

	return text, text.Close
}

// A server that did not say how bad it is means it to be seen.
func severityOf(severity int) Severity {
	if severity < int(Error) || severity > int(Hint) {
		return Error
	}

	return Severity(severity)
}
