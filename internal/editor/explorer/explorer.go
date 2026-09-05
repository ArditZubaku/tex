// Package explorer is the file tree as the editor drives it: what '<leader>e'
// opens, the keys it reads while it is up, and the file it hands to ':e'.
package explorer

import (
	"os"
	"path/filepath"
	"time"

	"github.com/ArditZubaku/tex/internal/editor/command"
	"github.com/ArditZubaku/tex/internal/editor/filetree"
	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/ArditZubaku/tex/internal/editor/view"
	"github.com/nsf/termbox-go"
)

// Open is '<leader>e', which toggles: the second press is what closes
// the explorer again.
func Open(e *state.Editor) {
	if e.ExplorerOpen {
		Close(e)
		return
	}

	dir, err := filepath.Abs(filepath.Dir(e.SourceFile))
	if err != nil {
		dir = "."
	}

	if goTo(e, dir, filepath.Base(e.SourceFile)) {
		e.ExplorerOpen, e.Mode = true, state.ExplorerMode
	}
}

func Close(e *state.Editor) {
	e.ExplorerOpen, e.Mode = false, state.ReadMode
	e.ClampCol()
}

func goTo(e *state.Editor, dir, on string) bool {
	if err := e.Exp.Go(dir, on); err != nil {
		e.StatusMsg = "E484: Can't open file " + dir
		return false
	}

	return true
}

func leaveDir(e *state.Editor) {
	if err := e.Exp.Leave(); err != nil {
		e.StatusMsg = "E484: Can't open file " + filepath.Dir(e.Exp.Dir())
	}
}

// startSearch is '/', which hands the status line to the same prompt
// the buffer's search uses; what is typed there narrows the listing as it goes.
func startSearch(e *state.Editor) { e.StartPrompt('/') }

func Filter(e *state.Editor, filter string) { e.Exp.Filter(filter) }

// clearFilter is Esc: it drops the filter it finds, and closes the explorer
// when there is none left to drop.
func clearFilter(e *state.Editor) {
	if e.Exp.Filtered() == "" {
		Close(e)
		return
	}

	Filter(e, "")
}

func openSelected(e *state.Editor) {
	entry, ok := e.Exp.Selected()
	if !ok {
		return
	}
	if entry.Name == filetree.ParentDir {
		leaveDir(e)
		return
	}

	// the entry's own flag is false for a symlink to a directory, so what to do
	// with the one being opened is worth the single stat
	path := e.Exp.Path(entry.Name)
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		goTo(e, path, "")
		return
	}

	if command.Edit(e, path, false) {
		Close(e)
	}
}

func toggleHidden(e *state.Editor) {
	if err := e.Exp.ToggleHidden(); err != nil {
		e.StatusMsg = "E484: Can't open file " + e.Exp.Dir()
	}
}

func down(e *state.Editor)   { e.Exp.Move(1) }
func up(e *state.Editor)     { e.Exp.Move(-1) }
func top(e *state.Editor)    { e.Exp.Top() }
func bottom(e *state.Editor) { e.Exp.Bottom() }

// A half screen of the listing, which is what Ctrl-D and Ctrl-U move by, the
// same fraction of the window they move the buffer by.
func pageDown(e *state.Editor) { e.Exp.Move(Page(e)) }
func pageUp(e *state.Editor)   { e.Exp.Move(-Page(e)) }

func Page(e *state.Editor) int {
	return max((e.Rows-filetree.HeaderRows)/2, 1)
}

var actions = map[rune]func(*state.Editor){
	'j': down,
	'k': up,
	'l': openSelected,
	'h': leaveDir,
	'-': leaveDir,
	'g': top,
	'G': bottom,
	'H': toggleHidden,
	'/': startSearch,
	'q': Close,
}

var specialActions = map[termbox.Key]func(*state.Editor){
	termbox.KeyEnter:      openSelected,
	termbox.KeyEsc:        clearFilter,
	termbox.KeyArrowDown:  down,
	termbox.KeyArrowUp:    up,
	termbox.KeyArrowRight: openSelected,
	termbox.KeyArrowLeft:  leaveDir,
	termbox.KeyCtrlD:      pageDown,
	termbox.KeyCtrlU:      pageUp,
	termbox.KeyPgdn:       pageDown,
	termbox.KeyPgup:       pageUp,
}

func Key(e *state.Editor, event termbox.Event) {
	if event.Key == termbox.KeySpace {
		e.PendingKeys, e.PendingTime = append(e.PendingKeys[:0], ' '), time.Now()
		return
	}

	if event.Ch != 0 {
		leader := len(e.PendingKeys) > 0 && time.Since(e.PendingTime) < state.ChordTimeout
		e.PendingKeys = e.PendingKeys[:0]

		switch {
		case leader && event.Ch == 'e':
			Open(e)
		case leader:
		default:
			if action, ok := actions[event.Ch]; ok {
				action(e)
			}
		}
		return
	}

	e.PendingKeys = e.PendingKeys[:0]
	if action, ok := view.MoveKeys[event.Key]; ok {
		leaveFor(e, func() { action(e) })
		return
	}
	if action, ok := specialActions[event.Key]; ok {
		action(e)
	}
}

// leaveFor is Ctrl-hjkl out of the tree: the window it lands in is
// showing a buffer, so the explorer is left behind the way opening a file from
// it leaves it. A move with no window that way changes nothing.
func leaveFor(e *state.Editor, move func()) {
	was := view.CurrentWindow(e)
	move()
	if view.Focused() != was {
		Close(e)
	}
}

func Draw(e *state.Editor)          { e.Exp.Draw(e.WindowArea(), &e.Palette) }
func CursorRow(e *state.Editor) int { return e.Exp.CursorRow(e.WindowArea()) }
func Status(e *state.Editor) string { return e.Exp.Status() }
