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

	name, arg := line, ""
	if i := strings.IndexByte(line, ' '); i >= 0 {
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

func quit(force bool) {
	if modified && !force {
		statusMsg = "E37: No write since last change (add ! to override)"
		return
	}
	closeEditor()
}
