// Package command is the ':' line: the handful of VIM's ex commands the editor
// answers to, and the file writing and rereading they and Ctrl-S share.
package command

import (
	"cmp"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/ArditZubaku/tex/internal/buffer"
	"github.com/ArditZubaku/tex/internal/editor/history"
	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/ArditZubaku/tex/internal/editor/view"
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

	if row, ok := lineAddress(e, line); ok {
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
		"e edit":  func(e *state.Editor, arg string, force bool) { Edit(e, cmp.Or(arg, e.SourceFile), force) },
		"wq x xit": func(e *state.Editor, arg string, _ bool) {
			if Write(e, arg) {
				quitWindow(e, true)
			}
		},
		"q quit":                 func(e *state.Editor, _ string, force bool) { quitWindow(e, force) },
		"bn bnext":               func(e *state.Editor, _ string, _ bool) { view.NextBuffer(e) },
		"bp bprev bprevious bN":  func(e *state.Editor, _ string, _ bool) { view.PrevBuffer(e) },
		"bd bdel bdelete":        func(e *state.Editor, _ string, force bool) { view.CloseBuffer(e, force) },
		"ls buffers files":       func(e *state.Editor, _ string, _ bool) { view.ListBuffers(e) },
		"sp split new":           func(e *state.Editor, arg string, force bool) { splitInto(e, arg, false, force) },
		"vs vsp vsplit vnew":     func(e *state.Editor, arg string, force bool) { splitInto(e, arg, true, force) },
		"clo close":              func(e *state.Editor, _ string, _ bool) { view.CloseWindow(e) },
		"on only":                func(e *state.Editor, _ string, _ bool) { view.OnlyWindow(e) },
		"noh nohl nohlsearch":    func(e *state.Editor, _ string, _ bool) { e.HlSearch = false },
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
	e.StatusMsg = fmt.Sprintf("%q %dL written", path, e.Buf.LineCount())

	return true
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

func quit(e *state.Editor, force bool) {
	switch {
	case force:
	case e.Modified:
		e.StatusMsg = state.NoWriteSinceChange
		return
	default:
		if entry := view.ModifiedBuffer(e); entry != nil {
			e.StatusMsg = view.Unwritten(entry)
			return
		}
	}
	e.Close()
}
