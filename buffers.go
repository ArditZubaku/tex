package main

import (
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/mattn/go-runewidth"
	"github.com/nsf/termbox-go"
)

// The editor keeps every file it has opened in a list, the way VIM's hidden
// buffers and LazyVim's buffer line do: opening another file puts the one being
// left beside it rather than over it, unsaved changes and all, and Tab walks
// the list. An entry holds what belongs to its own file; the editor's globals
// are the live state of whichever entry is current, so every command goes on
// working on globals and knows nothing about the list.
type bufferEntry struct {
	buf                  *Buffer
	path                 string
	syntax               *Syntax
	row, col             int
	offsetRow, offsetCol int
	modified             bool
	undoStack, redoStack []change
}

var (
	buffers       []*bufferEntry
	currentBuffer int
	altPath       string // the buffer '<leader>bb' goes back to
)

// The buffer line takes the first row of the window; everything the buffer and
// the explorer draw sits below it.
const tabBarRows = 1

const modifiedMark = '●'

func screenRow(row int) int { return row + tabBarRows }

func statusRow() int { return ROWS + tabBarRows }

// currentEntry adopts the buffer the editor started with when the list is still
// empty, so that the list always has the file being edited in it without main
// having to seed it.
func currentEntry() *bufferEntry {
	if len(buffers) == 0 {
		buffers, currentBuffer = []*bufferEntry{{buf: buf, path: sourceFile, syntax: syntax}}, 0
	}

	return buffers[currentBuffer]
}

// syncBuffer copies the editor's own state back onto the entry it belongs to,
// so that the list is what the buffer line and ':ls' read.
func syncBuffer() {
	entry := currentEntry()
	entry.buf, entry.path, entry.syntax = buf, sourceFile, syntax
	entry.row, entry.col = currentRow, currentCol
	entry.offsetRow, entry.offsetCol = offsetRow, offsetCol
	entry.modified = modified
	entry.undoStack, entry.redoStack = undoStack, redoStack
}

func loadBuffer(index int) {
	altPath, currentBuffer = sourceFile, index
	entry := buffers[index]

	buf, sourceFile, syntax = entry.buf, entry.path, entry.syntax
	currentRow, currentCol = entry.row, entry.col
	offsetRow, offsetCol = entry.offsetRow, entry.offsetCol
	modified = entry.modified
	undoStack, redoStack, pendingChange = entry.undoStack, entry.redoStack, nil
	clampCol()
}

// switchBuffer wraps at both ends, the way LazyVim's buffer keys do.
func switchBuffer(index int) {
	if len(buffers) < 2 {
		return
	}

	if mode == VisualMode {
		exitVisual()
	}
	syncBuffer()
	loadBuffer(((index % len(buffers)) + len(buffers)) % len(buffers))
}

func nextBuffer() { switchBuffer(currentBuffer + 1) }
func prevBuffer() { switchBuffer(currentBuffer - 1) }

// alternateBuffer is '<leader>bb': back to the buffer last left, which is what
// VIM's own ':b#' goes to. It is held as a path rather than a position, so
// closing a buffer cannot leave it pointing at the wrong one.
func alternateBuffer() {
	if i := bufferIndex(altPath); i >= 0 && i != currentBuffer {
		switchBuffer(i)
		return
	}
	statusMsg = "E23: No alternate file"
}

// openInBuffer is what ':e' and the explorer do with a file: one already in the
// list is switched to rather than opened twice, and any other joins the list
// beside the buffer being left, which keeps its cursor and its unsaved changes.
func openInBuffer(path string) {
	syncBuffer()

	if i := bufferIndex(path); i >= 0 {
		loadBuffer(i)
		return
	}

	if mode == VisualMode {
		exitVisual()
	}
	buffers = append(buffers, &bufferEntry{buf: openBuffer(path), path: path, syntax: detectSyntax(path)})
	loadBuffer(len(buffers) - 1)
}

// closeCurrentBuffer is '<leader>d', which is ':bd' without the colon: dropping
// unsaved changes needs the command, since a chord has no '!' to add.
func closeCurrentBuffer() { closeBuffer(false) }

// closeBuffer is ':bd': the buffer is dropped and the editor lands on the one
// after it, or on an empty buffer when it was the last one open.
func closeBuffer(force bool) {
	if modified && !force {
		statusMsg = noWriteSinceChange
		return
	}

	syncBuffer()
	buf.Close()
	buffers = slices.Delete(buffers, currentBuffer, currentBuffer+1)
	if len(buffers) == 0 {
		buffers = append(buffers, &bufferEntry{
			buf:    newEmptyBuffer(),
			path:   defaultFileName,
			syntax: detectSyntax(defaultFileName),
		})
	}

	if mode == VisualMode {
		exitVisual()
	}
	loadBuffer(min(currentBuffer, len(buffers)-1))
}

// closeOtherBuffers, closeBuffersLeft and closeBuffersRight are LazyVim's
// '<leader>bo', '<leader>bl' and '<leader>br'.
func closeOtherBuffers() { closeBuffersWhere("other buffers", func(int) bool { return true }) }
func closeBuffersLeft() {
	closeBuffersWhere("buffers to the left", func(i int) bool { return i < currentBuffer })
}
func closeBuffersRight() {
	closeBuffersWhere("buffers to the right", func(i int) bool { return i > currentBuffer })
}

// closeBuffersWhere drops every buffer the predicate names, never the one being
// edited. Unsaved changes refuse the whole move rather than half of it, so that
// a mistyped chord cannot cost some of them and report the rest.
func closeBuffersWhere(what string, drop func(int) bool) {
	syncBuffer()

	doomed := make([]*bufferEntry, 0, len(buffers))
	for i, entry := range buffers {
		if i == currentBuffer || !drop(i) {
			continue
		}
		if entry.modified {
			statusMsg = unwritten(entry)
			return
		}
		doomed = append(doomed, entry)
	}

	if len(doomed) == 0 {
		statusMsg = "no " + what + " to close"
		return
	}

	current := buffers[currentBuffer]
	kept := make([]*bufferEntry, 0, len(buffers)-len(doomed))
	for _, entry := range buffers {
		if slices.Contains(doomed, entry) {
			entry.buf.Close()
			continue
		}
		kept = append(kept, entry)
	}
	buffers, currentBuffer = kept, slices.Index(kept, current)
	statusMsg = fmt.Sprintf("%d buffers closed", len(doomed))
	if len(doomed) == 1 {
		statusMsg = "1 buffer closed"
	}
}

func closeBuffers() {
	syncBuffer()
	for _, entry := range buffers {
		entry.buf.Close()
	}
}

// modifiedBuffer names a buffer other than the current one with unsaved
// changes, which is what keeps ':q' from taking them down with it.
func modifiedBuffer() *bufferEntry {
	syncBuffer()
	for i, entry := range buffers {
		if i != currentBuffer && entry.modified {
			return entry
		}
	}

	return nil
}

func unwritten(entry *bufferEntry) string {
	return fmt.Sprintf("E162: No write since last change for buffer %q", entry.path)
}

func bufferIndex(path string) int {
	target := absPath(path)

	return slices.IndexFunc(buffers, func(e *bufferEntry) bool { return absPath(e.path) == target })
}

func absPath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}

	return abs
}

// listBuffers is ':ls' folded onto the one row the status line has: the current
// buffer is marked with '%' and an unsaved one with a dot, as VIM marks them.
func listBuffers() {
	syncBuffer()

	labels := make([]string, 0, len(buffers))
	for i, entry := range buffers {
		mark := " "
		if i == currentBuffer {
			mark = "%"
		}
		label := mark + strconv.Itoa(i+1) + " " + filepath.Base(entry.path)
		if entry.modified {
			label += " " + string(modifiedMark)
		}
		labels = append(labels, label)
	}

	statusMsg = strings.Join(labels, "  ")
}

// A cell of the buffer line, laid out before anything is drawn so that
// scrolling the row to keep the current buffer on screen is one window over it.
// A zero rune is the second half of a wide one, which termbox draws itself.
type tabCell struct {
	ch     rune
	fg, bg termbox.Attribute
}

var tabBarOffset int

func displayBufferLine() {
	cells, from, to := bufferLineCells()
	scrollBufferLine(len(cells), from, to)

	for col := range COLS {
		ch, fg, bg := ' ', active.tabFg, active.tabBarBg
		if i := col + tabBarOffset; i >= 0 && i < len(cells) {
			ch, fg, bg = cells[i].ch, cells[i].fg, cells[i].bg
		}
		if ch == 0 {
			continue
		}
		termbox.SetCell(col, 0, ch, fg, bg)
	}
}

func bufferLineCells() (cells []tabCell, activeFrom, activeTo int) {
	syncBuffer()

	for i, entry := range buffers {
		fg, bg := active.tabFg, active.tabBarBg
		if i == currentBuffer {
			fg, bg, activeFrom = active.tabActiveFg, active.tabActiveBg, len(cells)
		}

		cells = appendTab(cells, entry, fg, bg)
		if i == currentBuffer {
			activeTo = len(cells)
		}
	}

	return cells, activeFrom, activeTo
}

func appendTab(cells []tabCell, entry *bufferEntry, fg, bg termbox.Attribute) []tabCell {
	label := " " + filepath.Base(entry.path) + " "
	if entry.modified {
		label += string(modifiedMark) + " "
	}

	for _, ch := range label {
		markFg := fg
		if ch == modifiedMark {
			markFg = active.tabModified
		}
		cells = append(cells, tabCell{ch: ch, fg: markFg, bg: bg})
		for range runewidth.RuneWidth(ch) - 1 {
			cells = append(cells, tabCell{bg: bg})
		}
	}

	return cells
}

func scrollBufferLine(width, from, to int) {
	if to-from >= COLS {
		tabBarOffset = from
		return
	}

	if from < tabBarOffset {
		tabBarOffset = from
	}
	if to > tabBarOffset+COLS {
		tabBarOffset = to - COLS
	}
	tabBarOffset = max(min(tabBarOffset, width-COLS), 0)
}
