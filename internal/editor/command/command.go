// Package command is the ':' line: the handful of VIM's ex commands the editor
// answers to, and the file writing and rereading they and Ctrl-S share.
package command

import (
	"cmp"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/ArditZubaku/tex/internal/buffer"
	"github.com/ArditZubaku/tex/internal/editor/find"
	"github.com/ArditZubaku/tex/internal/editor/history"
	"github.com/ArditZubaku/tex/internal/editor/notify"
	"github.com/ArditZubaku/tex/internal/editor/rename"
	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/ArditZubaku/tex/internal/editor/view"
	"github.com/ArditZubaku/tex/internal/format"
	"github.com/ArditZubaku/tex/internal/syntax"
	"github.com/ArditZubaku/tex/internal/theme"
)

// Run runs one ':' command. Only the handful a VIM user reaches for
// reflexively is here; anything else is reported rather than guessed at.
func Run(e *state.Editor, line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}

	// A line address is a jump like any other, so Ctrl-O comes back from it.
	if row, ok := lineAddress(e, line); ok {
		e.PushJump()
		e.Row, e.Col = row, 0
		e.ClampCol()
		return
	}

	// ':theme=2' and ':w file' are the same shape, an argument after the
	// command, so one split covers both separators
	name, arg := line, ""
	if i := strings.IndexAny(line, " ="); i >= 0 {
		name, arg = line[:i], strings.TrimSpace(line[i+1:])
	}

	force := strings.HasSuffix(name, "!")
	name = strings.TrimSuffix(name, "!")

	run, ok := exCommands[name]
	if !ok {
		e.StatusMsg = "E492: Not an editor command: " + line
		return
	}

	run(e, arg, force)
}

type exCommand func(e *state.Editor, arg string, force bool)

var exCommands = exCommandTable()

// exCommandTable spreads each command over the spellings VIM takes for it, so
// that ':bp' and ':bprevious' are one entry rather than one apiece. Every
// command is handed the argument and the '!' so that the table is one shape.
func exCommandTable() map[string]exCommand {
	commands := map[string]exCommand{
		"w write": func(e *state.Editor, arg string, _ bool) { Write(e, arg) },
		"wa wall": func(e *state.Editor, _ string, _ bool) { _ = WriteAll(e) },
		"e edit":  func(e *state.Editor, arg string, force bool) { Edit(e, cmp.Or(arg, e.SourceFile), force) },
		"wq x xit": func(e *state.Editor, arg string, _ bool) {
			if Write(e, arg) {
				quitWindow(e, false)
			}
		},
		"q quit":                 func(e *state.Editor, _ string, force bool) { quitWindow(e, force) },
		"qa qall quita quitall":  func(e *state.Editor, _ string, force bool) { quit(e, force) },
		"bn bnext":               func(e *state.Editor, _ string, _ bool) { view.NextBuffer(e) },
		"bp bprev bprevious bN":  func(e *state.Editor, _ string, _ bool) { view.PrevBuffer(e) },
		"bd bdel bdelete":        func(e *state.Editor, _ string, force bool) { view.CloseBuffer(e, force) },
		"ls buffers files":       func(e *state.Editor, _ string, _ bool) { view.ListBuffers(e) },
		"sp split new":           func(e *state.Editor, arg string, force bool) { splitInto(e, arg, false, force) },
		"vs vsp vsplit vnew":     func(e *state.Editor, arg string, force bool) { splitInto(e, arg, true, force) },
		"clo close":              func(e *state.Editor, _ string, _ bool) { view.CloseWindow(e) },
		"on only":                func(e *state.Editor, _ string, _ bool) { view.OnlyWindow(e) },
		"rename ren":             func(e *state.Editor, arg string, _ bool) { rename.Run(e, arg) },
		"noh nohl nohlsearch":    func(e *state.Editor, _ string, _ bool) { e.HlSearch = false },
		"diag diagnostics":       func(e *state.Editor, _ string, _ bool) { find.OpenDiagnostics(e) },
		"theme colorscheme colo": func(e *state.Editor, arg string, _ bool) { setTheme(e, arg) },
	}

	table := make(map[string]exCommand, len(commands))
	for names, run := range commands {
		for name := range strings.FieldsSeq(names) {
			table[name] = run
		}
	}

	return table
}

// lineAddress covers ':42' and ':$', the two addresses that are a jump on their
// own rather than a range in front of a command.
func lineAddress(e *state.Editor, line string) (int, bool) {
	if line == "$" {
		return e.Buf.LineCount() - 1, true
	}

	row, err := strconv.Atoi(line)
	if err != nil {
		return 0, false
	}

	return min(max(row-1, 0), e.Buf.LineCount()-1), true
}

// Save is Ctrl-S, in either mode: ':w' without the prompt.
func Save(e *state.Editor) {
	Write(e, "")
}

// Write is ':w'. Given a name it writes there and carries on editing that
// file, the way VIM's ':saveas' does, because the buffer is reopened against
// whatever was written.
func Write(e *state.Editor, path string) bool {
	if path == "" {
		path = e.SourceFile
	}

	if err := e.Buf.Save(path); err != nil {
		slog.Error("Failed to save file", "path", path, "error", err)
		e.StatusMsg = "E212: Can't open file for writing: " + path
		return false
	}

	e.SourceFile, e.Modified = path, false
	e.Note.Clear()

	note := reformat(e, path)
	e.StatusMsg = fmt.Sprintf("%q %dL written%s", path, e.Buf.LineCount(), note)

	return true
}

// reformat runs the file's formatter over what the write has just put on disk
// and reads the result back, which is what puts the formatting under the cursor
// rather than leaving it only in the file. The write itself has already landed,
// so a formatter that refuses the file — while it is being typed into, almost
// always a syntax error — is reported without taking the save down with it.
func reformat(e *state.Editor, path string) string {
	rows := e.Buf.LineCount()

	result, err := format.Run(path)
	if err != nil {
		e.Note.Show(result.Name, reasonOf(err))
		return ""
	}
	if !result.Changed {
		return ""
	}

	e.Buf.Reload(path)
	state.Edited(path, 0, state.FileRewritten)
	// Reindenting leaves every line where it was, so what undo remembers still
	// names the line it was recorded against. Adding or removing one moves every
	// row below it, and replaying that would put lines back in the wrong places.
	if e.Buf.LineCount() != rows {
		e.Hist = history.History{}
	}
	e.Row = min(e.Row, e.Buf.LineCount()-1)
	e.ClampCol()

	return ", " + result.Name
}

// reasonOf is the formatter's own complaint with the name taken off the front,
// since the box it goes in is titled with the name already.
func reasonOf(err error) string {
	var refused *format.Error
	if errors.As(err, &refused) {
		return refused.Reason
	}

	return err.Error()
}

// WriteAll is ':wa': every buffer holding unsaved changes written back where it
// came from, which is how a rename across files — or an edit made in several of
// them — is committed in one command rather than one buffer at a time. It
// reports whether every one of them landed, which is what a caller that means
// to act on the write — quitting once it is safe to — needs to know.
func WriteAll(e *state.Editor) bool {
	view.SyncBuffer(e)
	e.Note.Clear()

	written := 0
	for _, entry := range view.Buffers() {
		if !entry.Modified {
			continue
		}
		if err := entry.Buf.Save(entry.Path); err != nil {
			slog.Error("Failed to save file", "path", entry.Path, "error", err)
			e.StatusMsg = "E212: Can't open file for writing: " + entry.Path
			e.Modified = view.CurrentEntry(e).Modified
			return false
		}
		entry.Modified = false
		reformatEntry(entry, &e.Note)
		written++
	}

	e.Modified = false
	view.Restore(e)
	e.StatusMsg = fmt.Sprintf("%d files written", written)
	if written == 1 {
		e.StatusMsg = "1 file written"
	}

	return true
}

// reformatEntry is reformat for a file ':wa' wrote that is not the one under
// the cursor: the same read-back, against the cursor and the history the buffer
// list is holding on that file's behalf.
func reformatEntry(entry *view.Entry, note *notify.Note) {
	rows := entry.Buf.LineCount()

	result, err := format.Run(entry.Path)
	if err != nil {
		note.Show(result.Name, reasonOf(err))
		return
	}
	if !result.Changed {
		return
	}

	entry.Buf.Reload(entry.Path)
	state.Edited(entry.Path, 0, state.FileRewritten)
	if entry.Buf.LineCount() != rows {
		entry.Hist = history.History{}
	}
	entry.Row = min(entry.Row, entry.Buf.LineCount()-1)
	entry.Col = min(entry.Col, entry.Buf.RuneLen(entry.Row))
}

// Edit is ':e', and what the explorer does with a file it is given: the
// file joins the buffer list, which leaves the one being edited where it is.
// Rereading the file the cursor is already in is the one case that drops
// changes, so that is the one that refuses the way ':q' does.
func Edit(e *state.Editor, path string, force bool) bool {
	view.SyncBuffer(e)
	rereading := view.BufferIndex(path) == view.Index()

	if rereading && e.Modified && !force {
		e.StatusMsg = state.NoWriteSinceChange
		return false
	}

	if rereading {
		e.Buf.Close()
		e.Buf, e.Lang = buffer.Open(path), syntax.Detect(path)
		e.Row, e.Col, e.OffsetRow, e.OffsetCol = 0, 0, 0, 0
		e.Modified = false
		e.Hist = history.History{}
		view.SyncBuffer(e)
	} else {
		view.Open(e, path)
	}
	e.StatusMsg = fmt.Sprintf("%q %dL", path, e.Buf.LineCount())

	return true
}

// setTheme is ':theme=2'. Without an argument it names the theme in use and
// the ones it could be swapped for.
func setTheme(e *state.Editor, arg string) {
	if arg == "" {
		e.StatusMsg = "theme=" + strconv.Itoa(themeIndex(e)) + " (" + strings.Join(themeNames(), ", ") + ")"
		return
	}

	n, err := strconv.Atoi(arg)
	if err != nil || n < 1 || n > len(theme.Themes) {
		e.StatusMsg = "E474: Invalid argument: theme=" + arg
		return
	}

	e.Palette = theme.Themes[n-1]
	e.StatusMsg = "theme=" + arg + " (" + e.Palette.Name + ")"
}

func themeIndex(e *state.Editor) int {
	for i, t := range theme.Themes {
		if t.Name == e.Palette.Name {
			return i + 1
		}
	}

	return 0
}

func themeNames() []string {
	names := make([]string, 0, len(theme.Themes))
	for i, t := range theme.Themes {
		names = append(names, strconv.Itoa(i+1)+"="+t.Name)
	}

	return names
}

// splitInto is ':split' and ':vsplit', which take the name of a file to open in
// the window they make, as VIM's own do.
func splitInto(e *state.Editor, path string, vertical, force bool) {
	if !view.Split(e, vertical) {
		return
	}
	if path != "" {
		Edit(e, path, force)
	}
}

// quitWindow is ':q': it closes the window it was typed in, and quits the
// editor when that was the last one, the way VIM does.
func quitWindow(e *state.Editor, force bool) {
	if len(view.List(e)) > 1 {
		view.CloseWindow(e)
		return
	}
	quit(e, force)
}

// quit is the editor going, whatever asked: '!' takes the unsaved buffers down
// with it, and anything else asks about them first.
func quit(e *state.Editor, force bool) {
	if force {
		e.Close()
		return
	}
	Quit(e)
}
