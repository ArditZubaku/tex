// Package rename is '<leader>cr': the identifier under the cursor renamed
// wherever it is mentioned, read off the text the way 'gd' and 'gr' read it
// rather than from a language server. What it takes for a mention to be the
// same thing rather than the same spelling is in scope.go.
package rename

import (
	"bytes"
	"fmt"
	"strconv"

	"github.com/ArditZubaku/tex/internal/buffer"
	"github.com/ArditZubaku/tex/internal/chars"
	"github.com/ArditZubaku/tex/internal/decl"
	"github.com/ArditZubaku/tex/internal/editor/history"
	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/ArditZubaku/tex/internal/editor/view"
	"github.com/ArditZubaku/tex/internal/project"
	"github.com/ArditZubaku/tex/internal/syntax"
)

const noIdentifier = "E349: No identifier under the cursor"

// Start is '<leader>cr': the ':' line opened on ':rename <name>' with the name
// under the cursor already typed, which is where LazyVim's own rename starts
// from — a name is more often edited into the new one than retyped.
func Start(e *state.Editor) {
	from, ok := decl.Under(e.Buf.Line(e.Row), e.Col)
	if !ok {
		e.StatusMsg = noIdentifier
		return
	}

	e.StartPromptWith(':', "rename "+from)
}

// Run is ':rename bar', which renames the identifier under the cursor as far as
// it reaches and no further — a local through its own block, a package-level
// name through the project. Nothing is written: a file the rename reached joins
// the buffer list with its changes unsaved, so that it can be read through,
// undone with its own 'u' and written with ':w' or ':wa'.
func Run(e *state.Editor, to string) {
	line := e.Buf.Line(e.Row)
	from, ok := decl.Under(line, e.Col)
	switch {
	case !ok:
		e.StatusMsg = noIdentifier
		return
	case to == "":
		e.StatusMsg = "E471: Argument required"
		return
	case !isIdentifier(to):
		e.StatusMsg = "E474: Invalid argument: " + to
		return
	case to == from:
		e.StatusMsg = "already named " + from
		return
	}

	old, name := []rune(from), []rune(to)
	where := scopeOf(e, from, wordStart(line, e.Col))
	here, _ := where.matcherFor(where.own, old, name)
	at, before := here.mentionAt(line, e.Col)

	rows := [2]int{0, e.Buf.LineCount()}
	if where.local {
		rows = [2]int{where.from, where.to}
	}

	e.BeginChange()
	changes, lines := pass{
		b: e.Buf, lang: e.Lang, touch: e.TouchLine, m: here, rows: rows,
		at: [2]int{e.Row, wordStart(line, e.Col)},
	}.run()
	e.EndChange()

	e.Modified = true
	e.Col = at + before*(len(name)-len(old))
	e.ClampCol()

	elsewhere, files := 0, 1
	if !where.local {
		elsewhere, files = renameBeside(e, where, old, name)
		files++
	}
	e.StatusMsg = fmt.Sprintf("renamed %s to %s: %s", from, to,
		tally(where, changes+elsewhere, lines, files))
}

// renameBeside is the reach past the file being edited: the files of the same
// kind beside it, each renamed in as far as this name reaches into it. One
// already open is renamed in as it stands, unsaved changes and all, rather than
// as it sits on disk.
func renameBeside(e *state.Editor, where scope, from, to []rune) (changes, files int) {
	for path, content := range project.Siblings(e.SourceFile) {
		m, ok := where.matcherFor(homeOf(path, content), from, to)
		if !ok || !bytes.Contains(content, []byte(m.needle())) {
			continue
		}

		hits := renameFile(e, path, m)
		if hits == 0 {
			continue
		}
		changes, files = changes+hits, files+1
	}

	return changes, files
}

// renameFile takes a file the editor is not showing. It joins the buffer list
// only once the rename has actually changed it, so a file that spells the name
// without meaning it is left closed, and it brings the history of that one
// change with it: a file the rename reached is one 'u' from being put back.
func renameFile(e *state.Editor, path string, m matcher) int {
	if entry := view.Buffer(path); entry != nil {
		return renameEntry(entry, m)
	}

	b := buffer.Open(path)
	var hist history.History
	hist.Begin(0, 0)
	changes, _ := pass{
		b: b, lang: syntax.Detect(path), touch: func(row int) { hist.TouchLine(b, row) },
		m: m, rows: [2]int{0, b.LineCount()}, at: [2]int{-1, -1},
	}.run()
	hist.End(false)

	if changes == 0 {
		b.Close()
		return 0
	}
	view.Adopt(e, path, b, hist)

	return changes
}

func renameEntry(entry *view.Entry, m matcher) int {
	entry.Hist.Begin(entry.Row, entry.Col)
	changes, _ := pass{
		b: entry.Buf, lang: entry.Lang, touch: func(row int) { entry.Hist.TouchLine(entry.Buf, row) },
		m: m, rows: [2]int{0, entry.Buf.LineCount()}, at: [2]int{-1, -1},
	}.run()
	entry.Hist.End(false)

	if changes > 0 {
		entry.Modified = true
	}

	return changes
}

func isIdentifier(name string) bool {
	for i, ch := range name {
		if !chars.IsWord(ch) || (i == 0 && ch >= '0' && ch <= '9') {
			return false
		}
	}

	return name != ""
}

// tally is what the status line reports it did. How far the rename reached is
// the thing to know about: the block a local never left, or the files it opened
// and has not written.
func tally(where scope, changes, lines, files int) string {
	switch {
	case where.local:
		return strconv.Itoa(changes) + " changes, local to this block"
	case files > 1:
		return fmt.Sprintf("%d changes in %d files, none written (:wa)", changes, files)
	case changes == 1:
		return "1 change"
	case lines == 1:
		return strconv.Itoa(changes) + " changes on 1 line"
	default:
		return fmt.Sprintf("%d changes on %d lines", changes, lines)
	}
}
