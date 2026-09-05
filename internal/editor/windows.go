package editor

import (
	"slices"

	"github.com/ArditZubaku/tex/internal/layout"
	"github.com/nsf/termbox-go"
)

// A window is one view of a buffer: which buffer it shows, where its cursor and
// its viewport are in it, and the rectangle of the screen it draws in. The
// editor's globals are the live state of the current window, so every command
// goes on working on globals and knows nothing about the split it runs in.
type window struct {
	entry                *bufferEntry
	cursorRow, cursorCol int
	offsetRow, offsetCol int
	rect                 layout.Rect
}

var (
	root    *layout.Tree[*window]
	current *window

	screenRows, screenCols int // the area the windows share
	winRow, winCol         int // where the window being drawn starts on screen

	separators []layout.Separator
)

// A window narrower than its gutter and a few columns of text, or shorter than
// a line of text between two edges, is no use to anybody: VIM refuses the split
// rather than making one.
const (
	minWindowRows = 3
	minWindowCols = 20
)

func screenRow(row int) int { return winRow + row }
func screenCol(col int) int { return winCol + col }

func statusRow() int { return screenRows + tabBarRows }

// currentWindow adopts the whole area as one window when nothing has been split
// yet, so that the editor always has a window without main having to seed one.
func currentWindow() *window {
	if current == nil {
		current = &window{
			entry:     currentEntry(),
			cursorRow: currentRow, cursorCol: currentCol,
			offsetRow: offsetRow, offsetCol: offsetCol,
			rect: layout.Rect{Row: tabBarRows, Rows: ROWS, Cols: COLS},
		}
		root = layout.Leaf(current)
	}

	return current
}

// syncWindow copies the editor's own state back onto the window it belongs to,
// so that leaving a window and coming back to it finds it as it was left.
func syncWindow() {
	w := currentWindow()
	w.entry = currentEntry()
	w.cursorRow, w.cursorCol = currentRow, currentCol
	w.offsetRow, w.offsetCol = offsetRow, offsetCol
	syncBuffer()
}

// showWindow points the globals at a window without making it the current one,
// which is what lets one pass draw every window through the same renderer. It
// leaves the buffer's history alone: only the window being worked in owns that.
func showWindow(w *window) {
	buf, sourceFile, lang = w.entry.buf, w.entry.path, w.entry.lang
	currentRow, currentCol = w.cursorRow, w.cursorCol
	offsetRow, offsetCol = w.offsetRow, w.offsetCol
	applyRect(w)
}

// applyRect is the part of a window the layout pass hands over on its own: the
// room it has to draw in, which a resize changes under a window that is
// otherwise untouched.
func applyRect(w *window) {
	winRow, winCol, ROWS, COLS = w.rect.Row, w.rect.Col, w.rect.Rows, w.rect.Cols
}

// applyWindow makes a window the one being worked in: it takes the buffer's
// history with it, and drops a selection, which belonged to the window left.
func applyWindow(w *window) {
	if mode == VisualMode {
		exitVisual()
	}
	current, currentBuffer = w, max(slices.Index(buffers, w.entry), 0)
	applyEntry(w.entry)
	showWindow(w)
	clampCol()
}

func focusWindow(w *window) {
	if w == nil || w == currentWindow() {
		return
	}

	syncWindow()
	applyWindow(w)
}

// windowList walks the leaves in the order they lie on screen, adopting the
// area as one window when nothing has been laid out yet.
func windowList() []*window {
	currentWindow()

	return root.Leaves(nil)
}

func splitBelow() { splitWindow(false) }
func splitRight() { splitWindow(true) }

// splitWindow is ':split' and ':vsplit': the new window takes half the room of
// the one it splits and the cursor moves into it, showing the same buffer at
// the same place — below or to the right, as VIM does with 'splitbelow' and
// 'splitright' set, which is how LazyVim has them.
func splitWindow(vertical bool) bool {
	w := currentWindow()
	if (vertical && w.rect.Cols <= 2*minWindowCols) || (!vertical && w.rect.Rows <= 2*minWindowRows) {
		statusMsg = "E36: Not enough room"
		return false
	}

	syncWindow()
	fresh := &window{
		entry:     w.entry,
		cursorRow: w.cursorRow, cursorCol: w.cursorCol,
		offsetRow: w.offsetRow, offsetCol: w.offsetCol,
	}
	root = root.InsertBeside(w, vertical, fresh)
	layoutWindows()
	applyWindow(fresh)

	return true
}

// closeWindow is ':close' and 'Ctrl-W c'. The buffer it was showing stays in
// the buffer list, unsaved changes and all, so closing a window loses nothing.
func closeWindow() {
	list := windowList()
	if len(list) < 2 {
		statusMsg = "E444: Cannot close last window"
		return
	}

	syncWindow()
	next := slices.Index(list, current)
	root = root.Prune(current)
	layoutWindows()

	list = windowList()
	applyWindow(list[min(next, len(list)-1)])
}

// onlyWindow is ':only' and 'Ctrl-W o': every other window is closed, leaving
// the one being worked in with the whole area to itself.
func onlyWindow() {
	syncWindow()
	root = layout.Leaf(current)
	layoutWindows()
	applyWindow(current)
}

func focusLeft()  { focusDirection(0, -1) }
func focusRight() { focusDirection(0, 1) }
func focusUp()    { focusDirection(-1, 0) }
func focusDown()  { focusDirection(1, 0) }

// focusDirection is 'Ctrl-W h' and its three friends: the nearest window on
// that side of the current one, of those it shares any rows (or columns) with,
// so that a move up cannot land in a window off to one side.
func focusDirection(dRow, dCol int) {
	w := currentWindow()

	var best *window
	bestGap := 0
	for _, other := range windowList() {
		if other == w {
			continue
		}

		gap, ok := w.rect.Gap(other.rect, dRow, dCol)
		if !ok {
			continue
		}
		if best == nil || gap < bestGap {
			best, bestGap = other, gap
		}
	}

	focusWindow(best)
}

// layoutWindows hands every window its rectangle, which is what the renderer
// and the directional moves both read; it runs once a frame, since the terminal
// may have been resized since the last one.
func layoutWindows() {
	currentWindow()
	separators = root.Place(
		layout.Rect{Row: tabBarRows, Rows: screenRows, Cols: screenCols},
		separators[:0],
		func(w *window, rect layout.Rect) { w.rect = rect },
	)
	applyRect(current)
}

// displayWindows draws every window through the one renderer, pointing the
// globals at each in turn. Drawing changes nothing the editor is doing, so the
// current window's own state is put back at the end.
func displayWindows() {
	syncWindow()
	live := mode

	for _, w := range windowList() {
		// only the window being worked in draws its mode: a selection belongs
		// to the window it was made in, not to every view of the buffer
		mode = live
		if w != current {
			mode = ReadMode
		}

		showWindow(w)
		if explorerOpen && w == current {
			displayExplorer()
			continue
		}
		scrollTextBuffer()
		displayTextBuffer()
		w.offsetRow, w.offsetCol = offsetRow, offsetCol
	}

	mode = live
	showWindow(current)
	displaySeparators()
}

func displaySeparators() {
	for _, s := range separators {
		ch := '─'
		if s.Vertical {
			ch = '│'
		}
		for i := range s.Length {
			row, col := s.Row, s.Col
			if s.Vertical {
				row += i
			} else {
				col += i
			}
			termbox.SetCell(col, row, ch, active.Separator, active.Background)
		}
	}
}
