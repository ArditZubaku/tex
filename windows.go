package main

import (
	"slices"

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
	row, col, rows, cols int
}

// The layout is a tree of rows and columns of windows: a leaf holds one window,
// and a node divides its rectangle equally between its children — side by side
// when it is vertical, stacked when it is not. Splitting in the direction a
// node already runs adds a child to it rather than nesting under it, which is
// what makes a third split a third of the room rather than a quarter.
type layout struct {
	win      *window
	vertical bool
	children []*layout
}

// A separator is the line drawn between two windows, kept from the layout pass
// so that drawing them is one walk over the gaps rather than a second one over
// the tree.
type separator struct {
	row, col, length int
	vertical         bool
}

var (
	root    *layout
	current *window

	screenRows, screenCols int // the area the windows share
	winRow, winCol         int // where the window being drawn starts on screen

	separators []separator
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
			row: tabBarRows, rows: ROWS, cols: COLS,
		}
		root = &layout{win: current}
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
	buf, sourceFile, syntax = w.entry.buf, w.entry.path, w.entry.syntax
	currentRow, currentCol = w.cursorRow, w.cursorCol
	offsetRow, offsetCol = w.offsetRow, w.offsetCol
	applyRect(w)
}

// applyRect is the part of a window the layout pass hands over on its own: the
// room it has to draw in, which a resize changes under a window that is
// otherwise untouched.
func applyRect(w *window) {
	winRow, winCol, ROWS, COLS = w.row, w.col, w.rows, w.cols
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

	return appendWindows(nil, root)
}

func appendWindows(out []*window, node *layout) []*window {
	if node == nil {
		return out
	}
	if node.win != nil {
		return append(out, node.win)
	}

	for _, child := range node.children {
		out = appendWindows(out, child)
	}

	return out
}

func splitBelow() { splitWindow(false) }
func splitRight() { splitWindow(true) }

// splitWindow is ':split' and ':vsplit': the new window takes half the room of
// the one it splits and the cursor moves into it, showing the same buffer at
// the same place — below or to the right, as VIM does with 'splitbelow' and
// 'splitright' set, which is how LazyVim has them.
func splitWindow(vertical bool) bool {
	w := currentWindow()
	if (vertical && w.cols <= 2*minWindowCols) || (!vertical && w.rows <= 2*minWindowRows) {
		statusMsg = "E36: Not enough room"
		return false
	}

	syncWindow()
	fresh := &window{
		entry:     w.entry,
		cursorRow: w.cursorRow, cursorCol: w.cursorCol,
		offsetRow: w.offsetRow, offsetCol: w.offsetCol,
	}
	root = insertBeside(root, w, vertical, &layout{win: fresh})
	layoutWindows()
	applyWindow(fresh)

	return true
}

func insertBeside(node *layout, target *window, vertical bool, fresh *layout) *layout {
	if node.win != nil {
		if node.win != target {
			return node
		}

		return &layout{vertical: vertical, children: []*layout{node, fresh}}
	}

	for i, child := range node.children {
		if child.win != target {
			node.children[i] = insertBeside(child, target, vertical, fresh)
			continue
		}
		if node.vertical == vertical {
			node.children = slices.Insert(node.children, i+1, fresh)
			return node
		}
		node.children[i] = &layout{vertical: vertical, children: []*layout{child, fresh}}
		return node
	}

	return node
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
	root = prune(root, current)
	layoutWindows()

	list = windowList()
	applyWindow(list[min(next, len(list)-1)])
}

func prune(node *layout, target *window) *layout {
	if node.win != nil {
		if node.win == target {
			return nil
		}

		return node
	}

	kept := node.children[:0]
	for _, child := range node.children {
		if pruned := prune(child, target); pruned != nil {
			kept = append(kept, pruned)
		}
	}
	node.children = kept

	switch len(node.children) {
	case 0:
		return nil
	case 1:
		return node.children[0]
	}

	return node
}

// onlyWindow is ':only' and 'Ctrl-W o': every other window is closed, leaving
// the one being worked in with the whole area to itself.
func onlyWindow() {
	syncWindow()
	root = &layout{win: current}
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

		var gap int
		switch {
		case dCol < 0:
			gap = w.col - (other.col + other.cols)
		case dCol > 0:
			gap = other.col - (w.col + w.cols)
		case dRow < 0:
			gap = w.row - (other.row + other.rows)
		default:
			gap = other.row - (w.row + w.rows)
		}

		if gap < 0 || !overlaps(w, other, dCol != 0) {
			continue
		}
		if best == nil || gap < bestGap {
			best, bestGap = other, gap
		}
	}

	focusWindow(best)
}

func overlaps(a, b *window, rows bool) bool {
	if rows {
		return b.row < a.row+a.rows && a.row < b.row+b.rows
	}

	return b.col < a.col+a.cols && a.col < b.col+b.cols
}

// layoutWindows hands every window its rectangle, which is what the renderer
// and the directional moves both read; it runs once a frame, since the terminal
// may have been resized since the last one.
func layoutWindows() {
	currentWindow()
	separators = separators[:0]
	place(root, tabBarRows, 0, screenRows, screenCols)
	applyRect(current)
}

func place(node *layout, row, col, rows, cols int) {
	if node.win != nil {
		node.win.row, node.win.col, node.win.rows, node.win.cols = row, col, rows, cols
		return
	}

	count := len(node.children)
	if node.vertical {
		room := cols - (count - 1) // a column between each pair carries the separator
		for i, child := range node.children {
			width := share(room, count, i)
			place(child, row, col, rows, width)
			col += width
			if i < count-1 {
				separators = append(separators, separator{row: row, col: col, length: rows, vertical: true})
				col++
			}
		}
		return
	}

	room := rows - (count - 1)
	for i, child := range node.children {
		height := share(room, count, i)
		place(child, row, col, height, cols)
		row += height
		if i < count-1 {
			separators = append(separators, separator{row: row, col: col, length: cols})
			row++
		}
	}
}

// share hands the remainder to the first windows, so that the room divides
// whole however many are sharing it.
func share(room, count, i int) int {
	size := room / count
	if i < room%count {
		size++
	}

	return size
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
		if s.vertical {
			ch = '│'
		}
		for i := range s.length {
			row, col := s.row, s.col
			if s.vertical {
				row += i
			} else {
				col += i
			}
			termbox.SetCell(col, row, ch, active.separator, active.background)
		}
	}
}
