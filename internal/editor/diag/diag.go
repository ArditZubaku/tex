// Package diag is what a language server said about the files: which of them,
// where in each, and how bad. It is a store rather than a field on the buffer
// list because a server publishes for files no buffer holds — break one file
// and it reports the package around it too — and because closing a buffer
// destroys everything it held, while a server only publishes again on a change:
// a file closed and reopened would come back looking clean.
package diag

import (
	"cmp"
	"slices"

	"github.com/ArditZubaku/tex/internal/lsp"
)

// Severity is the protocol's own numbering, worst first, which is what lets the
// worse of two be the smaller of them.
type Severity int

const (
	Error Severity = iota + 1
	Warning
	Info
	Hint
)

// None is a row or a column with nothing said about it.
const None Severity = 0

// A Note is one thing a server said, in the editor's own coordinates: rows, and
// columns counted in runes. The end is exclusive, the way the protocol's own
// range is.
type Note struct {
	Severity       Severity
	Message        string
	Source         string
	Row, Col       int
	EndRow, EndCol int
}

// A File is everything said about one file, in the order it is read in.
type File []Note

// The store is keyed the way a server names a file rather than the way the
// editor does, so that a path typed relative, a path opened absolute and a URI
// the server sent all reach the same entry.
var files = map[string]File{}

// Set is what a server has just said about a file, replacing whatever it said
// before. An empty publish is honoured rather than ignored: it is the only way
// an underline ever comes off a line that has been fixed.
func Set(path string, notes File) {
	key := lsp.FileURI(path)
	if len(notes) == 0 {
		delete(files, key)

		return
	}

	inOrder(notes)
	files[key] = notes
}

// Of is what was said about a file, which is nothing at all for most of them.
func Of(path string) File { return files[lsp.FileURI(path)] }

// Reset drops everything, which is a server going away or a test starting.
func Reset() { clear(files) }

// Count is how many of each a file has, for the flag the status line carries.
// Anything milder than a warning is left out of it: a bar that says a file has
// eleven hints has spent the width that mattered.
func Count(path string) (errors, warnings int) {
	for _, note := range Of(path) {
		switch note.Severity {
		case Error:
			errors++
		case Warning:
			warnings++
		}
	}

	return errors, warnings
}

// At is the worst thing said about the row the cursor is on, for the status
// line to report. The row rather than the exact column: a message is worth
// reading while the cursor is anywhere on the line it belongs to, and hunting
// for the rune a range starts at is not something anybody does on purpose.
func At(path string, row int) (Note, bool) {
	best, found := Note{}, false
	for _, note := range Of(path) {
		if !note.covers(row) {
			continue
		}
		if !found || note.Severity < best.Severity {
			best, found = note, true
		}
	}

	return best, found
}

// Next is the first thing said about the file past where the cursor is, and
// Prev the last one before it, both wrapping at the end the way 'n' and 'N'
// wrap a search.
func Next(path string, row, col int) (Note, bool) {
	notes := Of(path)
	if len(notes) == 0 {
		return Note{}, false
	}

	for _, note := range notes {
		if note.Row > row || (note.Row == row && note.Col > col) {
			return note, true
		}
	}

	return notes[0], true
}

func Prev(path string, row, col int) (Note, bool) {
	notes := Of(path)
	if len(notes) == 0 {
		return Note{}, false
	}

	for i := len(notes) - 1; i >= 0; i-- {
		if note := notes[i]; note.Row < row || (note.Row == row && note.Col < col) {
			return note, true
		}
	}

	return notes[len(notes)-1], true
}

func worse(of, than Severity) Severity {
	if than == None || (of != None && of < than) {
		return of
	}

	return than
}

func inOrder(notes File) {
	slices.SortStableFunc(notes, func(a, b Note) int {
		if by := cmp.Compare(a.Row, b.Row); by != 0 {
			return by
		}

		return cmp.Compare(a.Col, b.Col)
	})
}
