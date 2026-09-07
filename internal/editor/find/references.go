package find

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/ArditZubaku/tex/internal/decl"
	"github.com/ArditZubaku/tex/internal/editor/picker"
	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/ArditZubaku/tex/internal/lsp"
	"github.com/ArditZubaku/tex/internal/project"
)

// 'gr' lists everywhere the identifier under the cursor is mentioned, in the
// same popup the file picker uses, so that going to one of them is the same
// handful of keys. A language server is asked when there is one; failing that
// the text is read — the file being edited first, then the ones of the same
// kind beside it.
const maxReferences = 2000

func OpenReferences(e *state.Editor) {
	if lsp.Ready(e.SourceFile) {
		askReferences(e)

		return
	}

	referencesInText(e)
}

// The word is read for the title alone, so a cursor on nothing is a title
// without a name in it rather than a lookup refused: what the server was asked
// about is the position, not a word picked out of the line.
func askReferences(e *state.Editor) {
	token, from := ask(e)
	word, _ := wordUnderCursor(e)
	lsp.References(e.SourceFile, e.Buf, e.Row, e.Col, func(found []lsp.Location, err error) {
		if !stale(e, token, from) {
			listFound(e, word, found, err)
		}
	})
}

// A server that found nothing is not a server that could not answer: what it
// knows about a name nothing mentions is that nothing mentions it, and reading
// the text after that would only turn up the declaration again.
func listFound(e *state.Editor, word string, found []lsp.Location, err error) {
	if err != nil {
		referencesInText(e)

		return
	}

	entries := rows(referenced(found))
	if len(entries) == 0 {
		e.StatusMsg = "no references to " + word

		return
	}

	showPicker(e, fmt.Sprintf("%d references to %s", len(entries), word), entries)
}

func referenced(found []lsp.Location) []answer {
	out := make([]answer, 0, min(len(found), maxReferences))
	for _, one := range found[:min(len(found), maxReferences)] {
		out = append(out, answer{path: lsp.Path(one.URI), at: one.Range.Start})
	}

	return out
}

func referencesInText(e *state.Editor) {
	word, ok := wordUnderCursor(e)
	if !ok {
		e.StatusMsg = "E349: No identifier under the cursor"
		return
	}

	mentions, err := decl.Mentions(word)
	if err != nil {
		e.StatusMsg = "E486: Pattern not found: " + word
		return
	}

	entries := referencesInBuffer(e, mentions)
	entries = append(entries, referencesInFiles(e, mentions)...)
	if len(entries) == 0 {
		e.StatusMsg = "no references to " + word
		return
	}

	showPicker(e, fmt.Sprintf("%d references to %s", len(entries), word), entries)
}

func referencesInBuffer(e *state.Editor, mentions *regexp.Regexp) []picker.Entry {
	entries := make([]picker.Entry, 0, 16)
	for row := range e.Buf.LineCount() {
		line := e.LineBytes(row)
		for _, at := range mentions.FindAllIndex(line, -1) {
			entries = append(entries, reference(e.SourceFile, row, utf8.RuneCount(line[:at[0]]), string(line)))
			if len(entries) >= maxReferences {
				return entries
			}
		}
	}

	return entries
}

func referencesInFiles(e *state.Editor, mentions *regexp.Regexp) []picker.Entry {
	entries := make([]picker.Entry, 0, 16)
	for path, content := range project.Siblings(e.SourceFile) {
		for row, line := range strings.Split(string(content), "\n") {
			for _, at := range mentions.FindAllStringIndex(line, -1) {
				entries = append(entries, reference(path, row, utf8.RuneCountInString(line[:at[0]]), line))
				if len(entries) >= maxReferences {
					return entries
				}
			}
		}
	}

	return entries
}

func reference(path string, row, col int, line string) picker.Entry {
	return picker.Entry{
		Label: fmt.Sprintf("%s:%d: %s", filepath.Base(path), row+1, strings.TrimSpace(line)),
		Path:  path,
		Row:   row,
		Col:   col,
	}
}
