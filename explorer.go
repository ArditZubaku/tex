package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/nsf/termbox-go"
)

// The explorer is netrw as this editor needs it: one directory drawn over the
// whole window, with the buffer left where it is until a file is chosen.
type explorerEntry struct {
	name  string
	isDir bool
}

// explorerAll is everything the directory holds; explorerEntries is what the
// filter has left of it, which is what the selection indexes and the screen
// shows.
var (
	explorerOpen    bool
	explorerDir     string
	explorerAll     []explorerEntry
	explorerEntries []explorerEntry
	explorerFilter  string
	explorerSel     int
	explorerOffset  int
	explorerHidden  bool
)

const parentDir = ".."

// The header takes the first row, so that the directory being listed is on
// screen however far down the entries the selection has scrolled.
const explorerHeaderRows = 1

// openExplorer is '<leader>e', which toggles: the second press is what closes
// the explorer again.
func openExplorer() {
	if explorerOpen {
		closeExplorer()
		return
	}

	dir, err := filepath.Abs(filepath.Dir(sourceFile))
	if err != nil {
		dir = "."
	}

	if navigateTo(dir, filepath.Base(sourceFile)) {
		explorerOpen, mode = true, ExplorerMode
	}
}

func closeExplorer() {
	explorerOpen, mode = false, ReadMode
	clampCol()
}

// navigateTo is enterDir for the moves that leave a directory behind, which
// drop the filter with it; rereading the same directory keeps it.
func navigateTo(dir, on string) bool {
	explorerFilter = ""

	return enterDir(dir, on)
}

// enterDir lists dir and puts the selection on the entry named by on, so that
// stepping out of a directory leaves the cursor on the one just left.
func enterDir(dir, on string) bool {
	entries, err := readDir(dir)
	if err != nil {
		statusMsg = "E484: Can't open file " + dir
		return false
	}

	explorerDir, explorerAll = dir, entries
	selectEntry(on)

	return true
}

func leaveDir() {
	if parent := filepath.Dir(explorerDir); parent != explorerDir {
		navigateTo(parent, filepath.Base(explorerDir))
	}
}

// selectEntry re-runs the filter over the directory and puts the selection back
// on the entry named, or on the first one when it has been filtered away.
func selectEntry(name string) {
	explorerEntries = matching(explorerAll, explorerFilter)
	explorerSel, explorerOffset = 0, 0
	if i := slices.IndexFunc(explorerEntries, func(e explorerEntry) bool { return e.name == name }); i >= 0 {
		explorerSel = i
	}
}

func matching(entries []explorerEntry, filter string) []explorerEntry {
	if filter == "" {
		return entries
	}

	needle := strings.ToLower(filter)
	out := make([]explorerEntry, 0, len(entries))
	for _, e := range entries {
		if strings.Contains(strings.ToLower(e.name), needle) {
			out = append(out, e)
		}
	}

	return out
}

// startExplorerSearch is '/', which hands the status line to the same prompt
// the buffer's search uses; what is typed there narrows the listing as it goes.
func startExplorerSearch() { startPrompt('/') }

func filterExplorer(filter string) {
	on := selectedName()
	explorerFilter = filter
	selectEntry(on)
}

// clearFilter is Esc: it drops the filter it finds, and closes the explorer
// when there is none left to drop.
func clearFilter() {
	if explorerFilter == "" {
		closeExplorer()
		return
	}

	filterExplorer("")
}

func selectedName() string {
	if explorerSel < len(explorerEntries) {
		return explorerEntries[explorerSel].name
	}

	return ""
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
		navigateTo(path, "")
		return
	}

	if editFile(path, false) {
		closeExplorer()
	}
}

func toggleHidden() {
	explorerHidden = !explorerHidden
	enterDir(explorerDir, selectedName())
}

func explorerDown()     { explorerMove(1) }
func explorerUp()       { explorerMove(-1) }
func explorerGoTop()    { explorerSel = 0 }
func explorerGoBottom() { explorerSel = max(len(explorerEntries)-1, 0) }

// A half screen of the listing, which is what Ctrl-D and Ctrl-U move by, the
// same fraction of the window they move the buffer by.
func explorerPageDown() { explorerMove(explorerPage()) }
func explorerPageUp()   { explorerMove(-explorerPage()) }

func explorerPage() int {
	return max((ROWS-explorerHeaderRows)/2, 1)
}

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
	'/': startExplorerSearch,
	'q': closeExplorer,
}

var explorerSpecialActions = map[termbox.Key]func(){
	termbox.KeyEnter:      openSelected,
	termbox.KeyEsc:        clearFilter,
	termbox.KeyArrowDown:  explorerDown,
	termbox.KeyArrowUp:    explorerUp,
	termbox.KeyArrowRight: openSelected,
	termbox.KeyArrowLeft:  leaveDir,
	termbox.KeyCtrlD:      explorerPageDown,
	termbox.KeyCtrlU:      explorerPageUp,
	termbox.KeyPgdn:       explorerPageDown,
	termbox.KeyPgup:       explorerPageUp,
}

func handleExplorerKey(event termbox.Event) {
	if event.Key == termbox.KeySpace {
		pendingKeys, pendingTime = append(pendingKeys[:0], ' '), time.Now()
		return
	}

	if event.Ch != 0 {
		leader := len(pendingKeys) > 0 && time.Since(pendingTime) < chordTimeout
		pendingKeys = pendingKeys[:0]

		switch {
		case leader && event.Ch == 'e':
			openExplorer()
		case leader:
		default:
			if action, ok := explorerActions[event.Ch]; ok {
				action()
			}
		}
		return
	}

	pendingKeys = pendingKeys[:0]
	if action, ok := windowMoveKeys[event.Key]; ok {
		leaveExplorerFor(action)
		return
	}
	if action, ok := explorerSpecialActions[event.Key]; ok {
		action()
	}
}

// leaveExplorerFor is Ctrl-hjkl out of the tree: the window it lands in is
// showing a buffer, so the explorer is left behind the way opening a file from
// it leaves it. A move with no window that way changes nothing.
func leaveExplorerFor(move func()) {
	was := currentWindow()
	move()
	if current != was {
		closeExplorer()
	}
}

func displayExplorer() {
	scrollExplorer()
	printMessage(screenCol(0), screenRow(0), active.cursorLineNumber, active.background, padTo(explorerDir, COLS))

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

		printMessage(screenCol(0), screenRow(row), foreground, background, padTo(" "+entryLabel(entry), COLS))
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
	return screenRow(explorerSel - explorerOffset + explorerHeaderRows)
}

func explorerStatus() string {
	if explorerFilter != "" {
		return fmt.Sprintf(" EXPLORE: %s - %d/%d entries matching %q",
			filepath.Base(explorerDir), len(explorerEntries), len(explorerAll), explorerFilter)
	}

	return fmt.Sprintf(" EXPLORE: %s - %d entries", filepath.Base(explorerDir), len(explorerAll))
}
