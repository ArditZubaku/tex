package editor

import (
	"os"
	"path/filepath"
	"time"

	"github.com/ArditZubaku/tex/internal/editor/explorer"
	"github.com/ArditZubaku/tex/internal/layout"
	"github.com/nsf/termbox-go"
)

var (
	explorerOpen bool
	exp          explorer.Explorer
)

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

	if explorerGo(dir, filepath.Base(sourceFile)) {
		explorerOpen, mode = true, ExplorerMode
	}
}

func closeExplorer() {
	explorerOpen, mode = false, ReadMode
	clampCol()
}

func explorerGo(dir, on string) bool {
	if err := exp.Go(dir, on); err != nil {
		statusMsg = "E484: Can't open file " + dir
		return false
	}

	return true
}

func leaveDir() {
	if err := exp.Leave(); err != nil {
		statusMsg = "E484: Can't open file " + filepath.Dir(exp.Dir())
	}
}

// startExplorerSearch is '/', which hands the status line to the same prompt
// the buffer's search uses; what is typed there narrows the listing as it goes.
func startExplorerSearch() { startPrompt('/') }

func filterExplorer(filter string) { exp.Filter(filter) }

// clearFilter is Esc: it drops the filter it finds, and closes the explorer
// when there is none left to drop.
func clearFilter() {
	if exp.Filtered() == "" {
		closeExplorer()
		return
	}

	filterExplorer("")
}

func openSelected() {
	entry, ok := exp.Selected()
	if !ok {
		return
	}
	if entry.Name == explorer.ParentDir {
		leaveDir()
		return
	}

	// the entry's own flag is false for a symlink to a directory, so what to do
	// with the one being opened is worth the single stat
	path := exp.Path(entry.Name)
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		explorerGo(path, "")
		return
	}

	if editFile(path, false) {
		closeExplorer()
	}
}

func toggleHidden() {
	if err := exp.ToggleHidden(); err != nil {
		statusMsg = "E484: Can't open file " + exp.Dir()
	}
}

func explorerDown()     { exp.Move(1) }
func explorerUp()       { exp.Move(-1) }
func explorerGoTop()    { exp.Top() }
func explorerGoBottom() { exp.Bottom() }

// A half screen of the listing, which is what Ctrl-D and Ctrl-U move by, the
// same fraction of the window they move the buffer by.
func explorerPageDown() { exp.Move(explorerPage()) }
func explorerPageUp()   { exp.Move(-explorerPage()) }

func explorerPage() int {
	return max((ROWS-explorer.HeaderRows)/2, 1)
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

func displayExplorer()       { exp.Draw(windowArea(), &active) }
func explorerCursorRow() int { return exp.CursorRow(windowArea()) }
func explorerStatus() string { return exp.Status() }

// windowArea is the room the window being drawn has to itself.
func windowArea() layout.Rect {
	return layout.Rect{Row: winRow, Col: winCol, Rows: ROWS, Cols: COLS}
}
