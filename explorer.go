package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/nsf/termbox-go"
)

// The explorer is netrw as this editor needs it: one directory drawn over the
// whole window, with the buffer left where it is until a file is chosen.
type explorerEntry struct {
	name  string
	isDir bool
}

var (
	explorerDir     string
	explorerEntries []explorerEntry
	explorerSel     int
	explorerOffset  int
	explorerHidden  bool
)

const parentDir = ".."

// The header takes the first row, so that the directory being listed is on
// screen however far down the entries the selection has scrolled.
const explorerHeaderRows = 1

func openExplorer() {
	dir, err := filepath.Abs(filepath.Dir(sourceFile))
	if err != nil {
		dir = "."
	}

	if enterDir(dir, filepath.Base(sourceFile)) {
		mode = ExplorerMode
	}
}

func closeExplorer() {
	mode = ReadMode
	clampCol()
}

// enterDir lists dir and puts the selection on the entry named by on, so that
// stepping out of a directory leaves the cursor on the one just left.
func enterDir(dir, on string) bool {
	entries, err := readDir(dir)
	if err != nil {
		statusMsg = "E484: Can't open file " + dir
		return false
	}

	explorerDir, explorerEntries = dir, entries
	explorerSel, explorerOffset = 0, 0
	if i := slices.IndexFunc(entries, func(e explorerEntry) bool { return e.name == on }); i >= 0 {
		explorerSel = i
	}

	return true
}

func leaveDir() {
	if parent := filepath.Dir(explorerDir); parent != explorerDir {
		enterDir(parent, filepath.Base(explorerDir))
	}
}

func readDir(dir string) ([]explorerEntry, error) {
	listing, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	entries := make([]explorerEntry, 0, len(listing)+1)
	if parent := filepath.Dir(dir); parent != dir {
		entries = append(entries, explorerEntry{name: parentDir, isDir: true})
	}

	files := make([]explorerEntry, 0, len(listing))
	for _, e := range listing {
		if !explorerHidden && strings.HasPrefix(e.Name(), ".") {
			continue
		}
		files = append(files, explorerEntry{name: e.Name(), isDir: e.IsDir()})
	}

	// os.ReadDir sorts by name, so sorting directories to the front stably
	// keeps that order inside each of the two groups
	slices.SortStableFunc(files, func(a, b explorerEntry) int {
		switch {
		case a.isDir == b.isDir:
			return 0
		case a.isDir:
			return -1
		default:
			return 1
		}
	})

	return append(entries, files...), nil
}

func openSelected() {
	if len(explorerEntries) == 0 {
		return
	}

	entry := explorerEntries[explorerSel]
	if entry.name == parentDir {
		leaveDir()
		return
	}

	// the entry's own flag is false for a symlink to a directory, so what to do
	// with the one being opened is worth the single stat
	path := filepath.Join(explorerDir, entry.name)
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		enterDir(path, "")
		return
	}

	if editFile(path, false) {
		mode = ReadMode
	}
}

func toggleHidden() {
	explorerHidden = !explorerHidden

	on := ""
	if explorerSel < len(explorerEntries) {
		on = explorerEntries[explorerSel].name
	}
	enterDir(explorerDir, on)
}

func explorerDown()     { explorerMove(1) }
func explorerUp()       { explorerMove(-1) }
func explorerGoTop()    { explorerSel = 0 }
func explorerGoBottom() { explorerSel = max(len(explorerEntries)-1, 0) }

func explorerMove(delta int) {
	explorerSel = max(min(explorerSel+delta, len(explorerEntries)-1), 0)
}

var explorerActions = map[rune]func(){
	'j': explorerDown,
	'k': explorerUp,
	'l': openSelected,
	'h': leaveDir,
	'-': leaveDir,
	'g': explorerGoTop,
	'G': explorerGoBottom,
	'H': toggleHidden,
	'q': closeExplorer,
}

var explorerSpecialActions = map[termbox.Key]func(){
	termbox.KeyEnter:      openSelected,
	termbox.KeyEsc:        closeExplorer,
	termbox.KeyArrowDown:  explorerDown,
	termbox.KeyArrowUp:    explorerUp,
	termbox.KeyArrowRight: openSelected,
	termbox.KeyArrowLeft:  leaveDir,
}

func handleExplorerKey(event termbox.Event) {
	if event.Ch != 0 {
		if action, ok := explorerActions[event.Ch]; ok {
			action()
		}
		return
	}

	if action, ok := explorerSpecialActions[event.Key]; ok {
		action()
	}
}

func displayExplorer() {
	scrollExplorer()
	printMessage(0, 0, active.cursorLineNumber, active.background, padTo(explorerDir, COLS))

	for row := explorerHeaderRows; row < ROWS; row++ {
		i := row - explorerHeaderRows + explorerOffset
		if i >= len(explorerEntries) {
			break
		}

		entry := explorerEntries[i]
		foreground, background := active.plain, active.background
		if entry.isDir {
			foreground = active.function
		}
		if i == explorerSel {
			background = active.cursorLineBg
			highlightRow(row)
		}

		printMessage(0, row, foreground, background, padTo(" "+entryLabel(entry), COLS))
	}
}

func entryLabel(e explorerEntry) string {
	if e.isDir {
		return e.name + "/"
	}

	return e.name
}

func scrollExplorer() {
	rows := ROWS - explorerHeaderRows
	if rows < 1 {
		return
	}

	if explorerSel < explorerOffset {
		explorerOffset = explorerSel
	}
	if explorerSel >= explorerOffset+rows {
		explorerOffset = explorerSel - rows + 1
	}
}

func explorerCursorRow() int {
	return explorerSel - explorerOffset + explorerHeaderRows
}

func explorerStatus() string {
	return fmt.Sprintf(" EXPLORE: %s - %d entries", filepath.Base(explorerDir), len(explorerEntries))
}
