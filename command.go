package main

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"
)

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
	if modified && !force {
		statusMsg = "E37: No write since last change (add ! to override)"
		return
	}
	closeEditor()
}
