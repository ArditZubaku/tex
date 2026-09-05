package editor

import (
	"cmp"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/ArditZubaku/tex/internal/buffer"
	"github.com/ArditZubaku/tex/internal/editor/history"
	"github.com/ArditZubaku/tex/internal/syntax"
	"github.com/ArditZubaku/tex/internal/theme"
)

const noWriteSinceChange = "E37: No write since last change (add ! to override)"

// runExCommand runs one ':' command. Only the handful a VIM user reaches for
// reflexively is here; anything else is reported rather than guessed at.
func runExCommand(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}

	if row, ok := lineAddress(line); ok {
		currentRow, currentCol = row, 0
		clampCol()
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
		statusMsg = "E492: Not an editor command: " + line
		return
	}

	run(arg, force)
}

var exCommands = exCommandTable()

// exCommandTable spreads each command over the spellings VIM takes for it, so
// that ':bp' and ':bprevious' are one entry rather than one apiece. Every
// command is handed the argument and the '!' so that the table is one shape.
func exCommandTable() map[string]func(arg string, force bool) {
	commands := map[string]func(arg string, force bool){
		"w write": func(arg string, _ bool) { writeFile(arg) },
		"e edit":  func(arg string, force bool) { editFile(cmp.Or(arg, sourceFile), force) },
		"wq x xit": func(arg string, _ bool) {
			if writeFile(arg) {
				quitWindow(true)
			}
		},
		"q quit":                 func(_ string, force bool) { quitWindow(force) },
		"bn bnext":               func(string, bool) { nextBuffer() },
		"bp bprev bprevious bN":  func(string, bool) { prevBuffer() },
		"bd bdel bdelete":        func(_ string, force bool) { closeBuffer(force) },
		"ls buffers files":       func(string, bool) { listBuffers() },
		"sp split new":           func(arg string, force bool) { splitInto(arg, false, force) },
		"vs vsp vsplit vnew":     func(arg string, force bool) { splitInto(arg, true, force) },
		"clo close":              func(string, bool) { closeWindow() },
		"on only":                func(string, bool) { onlyWindow() },
		"noh nohl nohlsearch":    func(string, bool) { hlSearch = false },
		"theme colorscheme colo": func(arg string, _ bool) { setTheme(arg) },
	}

	table := make(map[string]func(arg string, force bool), len(commands))
	for names, run := range commands {
		for name := range strings.FieldsSeq(names) {
			table[name] = run
		}
	}

	return table
}

// lineAddress covers ':42' and ':$', the two addresses that are a jump on their
// own rather than a range in front of a command.
func lineAddress(line string) (int, bool) {
	if line == "$" {
		return buf.LineCount() - 1, true
	}

	row, err := strconv.Atoi(line)
	if err != nil {
		return 0, false
	}

	return min(max(row-1, 0), buf.LineCount()-1), true
}

// writeFile is ':w'. Given a name it writes there and carries on editing that
// file, the way VIM's ':saveas' does, because the buffer is reopened against
// whatever was written.
func writeFile(path string) bool {
	if path == "" {
		path = sourceFile
	}

	if err := buf.Save(path); err != nil {
		slog.Error("Failed to save file", "path", path, "error", err)
		statusMsg = "E212: Can't open file for writing: " + path
		return false
	}

	sourceFile, modified = path, false
	statusMsg = fmt.Sprintf("%q %dL written", path, buf.LineCount())

	return true
}

// editFile is ':e', and what the explorer does with a file it is given: the
// file joins the buffer list, which leaves the one being edited where it is.
// Rereading the file the cursor is already in is the one case that drops
// changes, so that is the one that refuses the way ':q' does.
func editFile(path string, force bool) bool {
	syncBuffer()
	rereading := bufferIndex(path) == currentBuffer

	if rereading && modified && !force {
		statusMsg = noWriteSinceChange
		return false
	}

	if rereading {
		buf.Close()
		buf, lang = buffer.Open(path), syntax.Detect(path)
		currentRow, currentCol, offsetRow, offsetCol = 0, 0, 0, 0
		modified = false
		hist = history.History{}
		syncBuffer()
	} else {
		openInBuffer(path)
	}
	statusMsg = fmt.Sprintf("%q %dL", path, buf.LineCount())

	return true
}

// setTheme is ':theme=2'. Without an argument it names the theme in use and
// the ones it could be swapped for.
func setTheme(arg string) {
	if arg == "" {
		statusMsg = "theme=" + strconv.Itoa(themeIndex()) + " (" + strings.Join(themeNames(), ", ") + ")"
		return
	}

	n, err := strconv.Atoi(arg)
	if err != nil || n < 1 || n > len(theme.Themes) {
		statusMsg = "E474: Invalid argument: theme=" + arg
		return
	}

	active = theme.Themes[n-1]
	statusMsg = "theme=" + arg + " (" + active.Name + ")"
}

func themeIndex() int {
	for i, t := range theme.Themes {
		if t.Name == active.Name {
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
func splitInto(path string, vertical, force bool) {
	if !splitWindow(vertical) {
		return
	}
	if path != "" {
		editFile(path, force)
	}
}

// quitWindow is ':q': it closes the window it was typed in, and quits the
// editor when that was the last one, the way VIM does.
func quitWindow(force bool) {
	if len(windowList()) > 1 {
		closeWindow()
		return
	}
	quit(force)
}

func quit(force bool) {
	switch {
	case force:
	case modified:
		statusMsg = noWriteSinceChange
		return
	default:
		if entry := modifiedBuffer(); entry != nil {
			statusMsg = unwritten(entry)
			return
		}
	}
	closeEditor()
}
