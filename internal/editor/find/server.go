package find

import (
	"fmt"
	"strconv"

	"github.com/ArditZubaku/tex/internal/editor/picker"
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
func Reset() {
	asked = 0
	resetCompletion()
}

// An answer is one row a server sent, waiting on the text of its own file: the
// column it names is counted in the server's units, and which rune the cursor
// lands on is only knowable from the line itself. A row with no label of its
// own is labelled with that line, which is what 'gr' has always listed.
type answer struct {
	path  string
	at    lsp.Position
	label string
}

func rows(found []answer) []picker.Entry {
	entries := make([]picker.Entry, 0, len(found))
	for _, group := range byFile(found) {
		entries = append(entries, rowsIn(group)...)
	}

	return entries
}

// The order the server sent them in is kept, since it is already the order a
// list of them reads in. Grouping is only so that a file the editor does not
// hold is opened once rather than once per row.
func byFile(found []answer) [][]answer {
	groups := make([][]answer, 0, 4)
	where := make(map[string]int, 4)
	for _, one := range found {
		at, seen := where[one.path]
		if !seen {
			at = len(groups)
			where[one.path] = at
			groups = append(groups, nil)
		}
		groups[at] = append(groups[at], one)
	}

	return groups
}

func rowsIn(group []answer) []picker.Entry {
	text, done := view.TextOf(group[0].path)
	if text == nil {
		return nil
	}
	defer done()

	encoding := lsp.PositionEncoding()
	entries := make([]picker.Entry, 0, len(group))
	for _, one := range group {
		row, col := encoding.RowCol(text, one.at)
		if one.label == "" {
			entries = append(entries, reference(one.path, row, col, string(text.Line(row))))

			continue
		}
		entries = append(entries, picker.Entry{Label: one.label, Path: one.path, Row: row, Col: col})
	}

	return entries
}

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

// The protocol's own SymbolKind, named as the text-based listing names what it
// recognises, so that the two lists read the same whichever answered. The
// spellings are cut to the nine columns that listing gives the kind: Go has no
// constructors, so 'New' never stands for one here.
var kindNames = [...]string{
	1: "File", 2: "Module", 3: "Namespace", 4: "Package", 5: "Class",
	6: "Method", 7: "Property", 8: "Field", 9: "New", 10: "Enum",
	11: "Interface", 12: "Function", 13: "Variable", 14: "Constant",
	15: "String", 16: "Number", 17: "Boolean", 18: "Array", 19: "Object",
	20: "Key", 21: "Null", 22: "EnumValue", 23: "Struct", 24: "Event",
	25: "Operator", 26: "TypeParam",
}

// A kind the protocol grew after this list is drawn as the number the server
// sent: it still says that two rows are different kinds of thing.
func kindName(kind int) string {
	if kind < 1 || kind >= len(kindNames) {
		return strconv.Itoa(kind)
	}

	return kindNames[kind]
}

// Both listings label a row the same way, so that the popup reads the same
// whether a server or the text answered it.
func symbolLabel(kind, name string) string {
	return fmt.Sprintf("%-9s %s", kind, name)
}

// symbolRows is a server's declarations as the popup's own rows. The shape that
// names no file is a listing of one file, which is the one that was asked
// about.
func symbolRows(path string, found []lsp.Symbol) []picker.Entry {
	out := make([]answer, 0, len(found))
	for _, one := range found {
		in := one.In()
		if in == "" {
			in = path
		}
		out = append(out, answer{path: in, at: one.At(), label: symbolLabel(kindName(one.Kind), one.Name)})
	}

	return rows(out)
}
