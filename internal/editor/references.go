package editor

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/ArditZubaku/tex/internal/decl"
	"github.com/ArditZubaku/tex/internal/editor/picker"
	"github.com/ArditZubaku/tex/internal/project"
)

// 'gr' lists everywhere the identifier under the cursor is mentioned — the file
// being edited first, then the ones of the same kind beside it — in the same
// popup the file picker uses, so that going to one of them is the same handful
// of keys.
const maxReferences = 2000

func openReferences() {
	word, ok := wordUnderCursor()
	if !ok {
		statusMsg = "E349: No identifier under the cursor"
		return
	}

	mentions, err := decl.Mentions(word)
	if err != nil {
		statusMsg = "E486: Pattern not found: " + word
		return
	}

	entries := referencesInBuffer(mentions)
	entries = append(entries, referencesInFiles(mentions)...)
	if len(entries) == 0 {
		statusMsg = "no references to " + word
		return
	}

	showPicker(fmt.Sprintf("%d references to %s", len(entries), word), entries)
}

func referencesInBuffer(mentions *regexp.Regexp) []picker.Entry {
	entries := make([]picker.Entry, 0, 16)
	for row := range buf.LineCount() {
		line := lineBytes(row)
		for _, at := range mentions.FindAllIndex(line, -1) {
			entries = append(entries, reference(sourceFile, row, utf8.RuneCount(line[:at[0]]), string(line)))
			if len(entries) >= maxReferences {
				return entries
			}
		}
	}

	return entries
}

func referencesInFiles(mentions *regexp.Regexp) []picker.Entry {
	entries := make([]picker.Entry, 0, 16)
	for path, content := range project.Siblings(sourceFile) {
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
