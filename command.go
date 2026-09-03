package main

import (
	"cmp"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
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

	switch name {
	case "w", "write":
		writeFile(arg)
	case "e", "edit":
		editFile(cmp.Or(arg, sourceFile), force)
	case "bn", "bnext":
		nextBuffer()
	case "bp", "bprev", "bprevious", "bN":
		prevBuffer()
	case "bd", "bdel", "bdelete":
		closeBuffer(force)
	case "ls", "buffers", "files":
		listBuffers()
	case "q", "quit":
		quit(force)
	case "wq", "x", "xit":
		if writeFile(arg) {
			quit(true)
		}
	case "noh", "nohl", "nohlsearch":
		hlSearch = false
	case "theme", "colorscheme", "colo":
		setTheme(arg)
	default:
		statusMsg = "E492: Not an editor command: " + line
	}
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
		buf, syntax = openBuffer(path), detectSyntax(path)
		currentRow, currentCol, offsetRow, offsetCol = 0, 0, 0, 0
		modified = false
		undoStack, redoStack, pendingChange = nil, nil, nil
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
	if err != nil || n < 1 || n > len(themes) {
		statusMsg = "E474: Invalid argument: theme=" + arg
		return
	}

	active = themes[n-1]
	statusMsg = "theme=" + arg + " (" + active.name + ")"
}

func themeIndex() int {
	for i, t := range themes {
		if t.name == active.name {
			return i + 1
		}
	}

	return 0
}

func themeNames() []string {
	names := make([]string, 0, len(themes))
	for i, t := range themes {
		names = append(names, strconv.Itoa(i+1)+"="+t.name)
	}

	return names
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
