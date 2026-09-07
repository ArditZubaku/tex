package find

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ArditZubaku/tex/internal/editor/picker"
	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/ArditZubaku/tex/internal/project"
)

// '<leader>/' is LazyVim's project grep: ripgrep run from the project root
// over whatever is typed at the prompt, listed in the same popup 'gr' and the
// symbols use. There is no text-only fallback yet for a machine without
// ripgrep on its PATH — everything else in this package has one — so for now
// that is just reported rather than answered some slower way.

// GrepPrompt is the prompt's own tag for what Enter should do with the line,
// the way command.QuitPrompt tells a quit answer from an ex command; never
// drawn, since the label already says what is being asked for.
const GrepPrompt = 'G'

// OpenGrep asks for the pattern rather than reading one off the cursor, since
// a project search is a question typed out rather than a lookup on what is
// already there.
func OpenGrep(e *state.Editor) {
	e.StartLabelledPrompt(GrepPrompt, "Search> ", "")
}

// A real project turns up more matches than anybody reads; the popup's own
// filter is what finds the one that was meant.
const maxGrepResults = 2000

// The editor's loop blocks on the keyboard the way a save does, so ripgrep is
// given a bound for the same reason a formatter is.
const grepTimeout = 10 * time.Second

func RunGrep(e *state.Editor, pattern string) {
	if strings.TrimSpace(pattern) == "" {
		return
	}

	root := project.Root(e.SourceFile)
	entries, err := grep(root, pattern)
	if err != nil {
		e.StatusMsg = "ripgrep: " + err.Error()
		return
	}
	if len(entries) == 0 {
		e.StatusMsg = "no matches for " + pattern
		return
	}

	showPicker(e, fmt.Sprintf("%d matches for %s", len(entries), pattern), entries)
}

func grep(root, pattern string) ([]picker.Entry, error) {
	ctx, cancel := context.WithTimeout(context.Background(), grepTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "rg", "--vimgrep", "--smart-case", "--", pattern, ".")
	cmd.Dir = root

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	out, err := cmd.Output()

	var exitErr *exec.ExitError
	switch {
	case errors.Is(err, exec.ErrNotFound):
		return nil, errors.New("not found on PATH")
	case errors.As(err, &exitErr) && exitErr.ExitCode() == 1:
		return nil, nil // ripgrep's own way of saying nothing matched
	case err != nil:
		return nil, errors.New(firstLine(stderr.String(), err))
	}

	return parseVimgrep(root, out), nil
}

func parseVimgrep(root string, out []byte) []picker.Entry {
	trimmed := strings.TrimRight(string(out), "\n")
	if trimmed == "" {
		return nil
	}

	lines := strings.Split(trimmed, "\n")
	entries := make([]picker.Entry, 0, min(len(lines), maxGrepResults))
	for _, line := range lines {
		if entry, ok := vimgrepEntry(root, line); ok {
			entries = append(entries, entry)
			if len(entries) >= maxGrepResults {
				break
			}
		}
	}

	return entries
}

// vimgrepEntry reads one of ripgrep's own "path:line:col:text" rows back
// apart. The column it prints counts bytes, which only lines up with the
// cursor's own runes once what comes before it has been counted the same way.
func vimgrepEntry(root, line string) (picker.Entry, bool) {
	parts := strings.SplitN(line, ":", 4)
	if len(parts) != 4 {
		return picker.Entry{}, false
	}

	row, err := strconv.Atoi(parts[1])
	if err != nil {
		return picker.Entry{}, false
	}
	byteCol, err := strconv.Atoi(parts[2])
	if err != nil {
		return picker.Entry{}, false
	}

	idx := max(0, min(byteCol-1, len(parts[3])))
	col := utf8.RuneCountInString(parts[3][:idx])

	return reference(filepath.Join(root, parts[0]), row-1, col, parts[3]), true
}

// firstLine is the one line of ripgrep's own complaint worth putting on the
// status line, the way a formatter's is.
func firstLine(stderr string, err error) string {
	first, _, _ := strings.Cut(strings.TrimSpace(stderr), "\n")
	if first == "" {
		first = err.Error()
	}

	return first
}
