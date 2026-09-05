package editor

import (
	"os"
	"path/filepath"
	"time"

	"github.com/ArditZubaku/tex/internal/editor/explorer"
	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/ArditZubaku/tex/internal/editor/view"
	"github.com/nsf/termbox-go"
)

// openExplorer is '<leader>e', which toggles: the second press is what closes
// the explorer again.
func openExplorer() {
	if ed.ExplorerOpen {
		closeExplorer()
		return
	}

	dir, err := filepath.Abs(filepath.Dir(ed.SourceFile))
	if err != nil {
		dir = "."
	}

	if explorerGo(dir, filepath.Base(ed.SourceFile)) {
		ed.ExplorerOpen, ed.Mode = true, state.ExplorerMode
	}
}

func closeExplorer() {
	ed.ExplorerOpen, ed.Mode = false, state.ReadMode
	ed.ClampCol()
}

func explorerGo(dir, on string) bool {
	if err := ed.Exp.Go(dir, on); err != nil {
		ed.StatusMsg = "E484: Can't open file " + dir
		return false
	}

	return true
}

func leaveDir() {
	if err := ed.Exp.Leave(); err != nil {
		ed.StatusMsg = "E484: Can't open file " + filepath.Dir(ed.Exp.Dir())
	}
}

// startExplorerSearch is '/', which hands the status line to the same prompt
// the buffer's search uses; what is typed there narrows the listing as it goes.
func startExplorerSearch() { startPrompt('/') }

func filterExplorer(filter string) { ed.Exp.Filter(filter) }

// clearFilter is Esc: it drops the filter it finds, and closes the explorer
// when there is none left to drop.
func clearFilter() {
	if ed.Exp.Filtered() == "" {
		closeExplorer()
		return
	}

	filterExplorer("")
}

func openSelected() {
	entry, ok := ed.Exp.Selected()
	if !ok {
		return
	}
	if entry.Name == explorer.ParentDir {
		leaveDir()
		return
	}

	// the entry's own flag is false for a symlink to a directory, so what to do
	// with the one being opened is worth the single stat
	path := ed.Exp.Path(entry.Name)
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		explorerGo(path, "")
		return
	}

	if editFile(path, false) {
		closeExplorer()
	}
}

func toggleHidden() {
	if err := ed.Exp.ToggleHidden(); err != nil {
		ed.StatusMsg = "E484: Can't open file " + ed.Exp.Dir()
	}
}

func explorerDown()     { ed.Exp.Move(1) }
func explorerUp()       { ed.Exp.Move(-1) }
func explorerGoTop()    { ed.Exp.Top() }
func explorerGoBottom() { ed.Exp.Bottom() }

// A half screen of the listing, which is what Ctrl-D and Ctrl-U move by, the
// same fraction of the window they move the buffer by.
func explorerPageDown() { ed.Exp.Move(explorerPage()) }
func explorerPageUp()   { ed.Exp.Move(-explorerPage()) }

func explorerPage() int {
	return max((ed.Rows-explorer.HeaderRows)/2, 1)
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
		ed.PendingKeys, ed.PendingTime = append(ed.PendingKeys[:0], ' '), time.Now()
		return
	}

	if event.Ch != 0 {
		leader := len(ed.PendingKeys) > 0 && time.Since(ed.PendingTime) < state.ChordTimeout
		ed.PendingKeys = ed.PendingKeys[:0]

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

	ed.PendingKeys = ed.PendingKeys[:0]
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
	was := view.CurrentWindow(ed)
	move()
	if view.Focused() != was {
		closeExplorer()
	}
}

func displayExplorer()       { ed.Exp.Draw(ed.WindowArea(), &ed.Palette) }
func explorerCursorRow() int { return ed.Exp.CursorRow(ed.WindowArea()) }
func explorerStatus() string { return ed.Exp.Status() }
