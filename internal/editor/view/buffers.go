// Package view is what the editor shows: the list of open files, and the tree
// of windows the screen is split into. The state package holds whichever of
// them is being worked in; this one holds the rest, and the moves between them.
package view

import (
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/ArditZubaku/tex/internal/buffer"
	"github.com/ArditZubaku/tex/internal/editor/edit"
	"github.com/ArditZubaku/tex/internal/editor/history"
	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/ArditZubaku/tex/internal/editor/tabbar"
	"github.com/ArditZubaku/tex/internal/syntax"
)

// The editor keeps every file it has opened in a list, the way VIM's hidden
// buffers and LazyVim's buffer line do: opening another file puts the one being
// left beside it rather than over it, unsaved changes and all, and Tab walks
// the list. An entry holds what belongs to its own file; the editor's globals
// are the live state of whichever entry is current, so every command goes on
// working on globals and knows nothing about the list.
type Entry struct {
	Buf                  *buffer.Buffer
	Path                 string
	Lang                 *syntax.Syntax
	Row, Col             int
	OffsetRow, OffsetCol int
	Modified             bool
	Hist                 history.History
}

var (
	buffers       []*Entry
	currentBuffer int
	altPath       string // the buffer '<leader>bb' goes back to
)

func Buffers() []*Entry { return buffers }

func Index() int { return currentBuffer }

// CurrentEntry adopts the buffer the editor started with when the list is still
// empty, so that the list always has the file being edited in it without main
// having to seed it.
func CurrentEntry(e *state.Editor) *Entry {
	if len(buffers) == 0 {
		buffers, currentBuffer = []*Entry{{Buf: e.Buf, Path: e.SourceFile, Lang: e.Lang}}, 0
	}

	return buffers[currentBuffer]
}

// SyncBuffer copies the editor's own state back onto the entry it belongs to,
// so that the list is what the buffer line and ':ls' read.
func SyncBuffer(e *state.Editor) {
	entry := CurrentEntry(e)
	entry.Buf, entry.Path, entry.Lang = e.Buf, e.SourceFile, e.Lang
	entry.Row, entry.Col = e.Row, e.Col
	entry.OffsetRow, entry.OffsetCol = e.OffsetRow, e.OffsetCol
	entry.Modified = e.Modified
	entry.Hist = e.Hist
}

// Restore is SyncBuffer the other way round: what the list holds for the
// current file copied back onto the editor, which is what a command that
// reached that file through the list rather than through the editor — ':wa'
// formatting every buffer it wrote — leaves the editor needing.
func Restore(e *state.Editor) {
	entry := CurrentEntry(e)
	applyEntry(e, entry)
	e.Row, e.Col = entry.Row, entry.Col
	e.ClampCol()
}

func loadBuffer(e *state.Editor, index int) {
	altPath, currentBuffer = e.SourceFile, index
	entry := buffers[index]

	applyEntry(e, entry)
	e.Row, e.Col = entry.Row, entry.Col
	e.OffsetRow, e.OffsetCol = entry.OffsetRow, entry.OffsetCol
	CurrentWindow(e).Entry = entry
	e.ClampCol()
}

// applyEntry makes a buffer the one being worked on, history and all; where the
// cursor and the viewport are in it belongs to the window showing it.
func applyEntry(e *state.Editor, entry *Entry) {
	e.Buf, e.SourceFile, e.Lang = entry.Buf, entry.Path, entry.Lang
	e.Modified = entry.Modified
	e.Hist = entry.Hist
	e.Hist.Abandon()
}

// switchBuffer wraps at both ends, the way LazyVim's buffer keys do.
func switchBuffer(e *state.Editor, index int) {
	if len(buffers) < 2 {
		return
	}

	if e.Mode == state.VisualMode {
		edit.ExitVisual(e)
	}
	SyncWindow(e)
	loadBuffer(e, ((index%len(buffers))+len(buffers))%len(buffers))
}

func NextBuffer(e *state.Editor) { switchBuffer(e, currentBuffer+1) }
func PrevBuffer(e *state.Editor) { switchBuffer(e, currentBuffer-1) }

// AlternateBuffer is '<leader>bb': back to the buffer last left, which is what
// VIM's own ':b#' goes to. It is held as a path rather than a position, so
// closing a buffer cannot leave it pointing at the wrong one.
func AlternateBuffer(e *state.Editor) {
	if i := BufferIndex(altPath); i >= 0 && i != currentBuffer {
		switchBuffer(e, i)
		return
	}
	e.StatusMsg = "E23: No alternate file"
}

// Open is what ':e' and the explorer do with a file: one already in the
// list is switched to rather than opened twice, and any other joins the list
// beside the buffer being left, which keeps its cursor and its unsaved changes.
func Open(e *state.Editor, path string) {
	SyncWindow(e)

	if i := BufferIndex(path); i >= 0 {
		loadBuffer(e, i)
		return
	}

	if e.Mode == state.VisualMode {
		edit.ExitVisual(e)
	}
	buffers = append(buffers, &Entry{Buf: buffer.Open(path), Path: path, Lang: syntax.Detect(path)})
	loadBuffer(e, len(buffers)-1)
}

// Buffer is the entry a path is open as, or nil when it is not open at all. A
// command that reaches into another file reads what the buffer holds through
// it, so unsaved changes are worked on rather than overwritten.
func Buffer(path string) *Entry {
	if i := BufferIndex(path); i >= 0 {
		return buffers[i]
	}

	return nil
}

// Adopt puts a buffer changed without ever being opened into the list, which is
// how '<leader>cr' hands over the files it renamed in: each of them is there to
// be looked at, undone and written, and the cursor never left the file the
// rename was asked for in.
func Adopt(e *state.Editor, path string, b *buffer.Buffer, hist history.History) *Entry {
	SyncBuffer(e)

	entry := &Entry{Buf: b, Path: path, Lang: syntax.Detect(path), Modified: true, Hist: hist}
	buffers = append(buffers, entry)

	return entry
}

// Drop is the buffer of a file that is no longer on disk, which is what the
// explorer's 'd' leaves behind. There is nowhere left to write unsaved changes
// back to, so the entry goes whether or not it holds any.
func Drop(e *state.Editor, path string) bool {
	at := BufferIndex(path)
	if at < 0 {
		return false
	}
	if at == currentBuffer {
		CloseBuffer(e, true)
		return true
	}

	SyncBuffer(e)
	gone, live := buffers[at], buffers[currentBuffer]
	gone.Buf.Close()
	buffers = slices.Delete(buffers, at, at+1)
	currentBuffer = slices.Index(buffers, live)
	showBufferInstead(e, gone, live)

	return true
}

// CloseCurrentBuffer is '<leader>d', which is ':bd' without the colon: dropping
// unsaved changes needs the command, since a chord has no '!' to add.
func CloseCurrentBuffer(e *state.Editor) { CloseBuffer(e, false) }

// CloseBuffer is ':bd': the buffer is dropped and the editor lands on the one
// after it, or on an empty buffer when it was the last one open.
func CloseBuffer(e *state.Editor, force bool) {
	if e.Modified && !force {
		e.StatusMsg = state.NoWriteSinceChange
		return
	}

	SyncWindow(e)
	gone := buffers[currentBuffer]
	e.Buf.Close()
	buffers = slices.Delete(buffers, currentBuffer, currentBuffer+1)
	if len(buffers) == 0 {
		buffers = append(buffers, &Entry{
			Buf:  buffer.NewEmpty(),
			Path: state.DefaultFileName,
			Lang: syntax.Detect(state.DefaultFileName),
		})
	}

	if e.Mode == state.VisualMode {
		edit.ExitVisual(e)
	}
	index := min(currentBuffer, len(buffers)-1)
	showBufferInstead(e, gone, buffers[index])
	loadBuffer(e, index)
}

// CloseOtherBuffers, CloseBuffersLeft and CloseBuffersRight are LazyVim's
// '<leader>bo', '<leader>bl' and '<leader>br'.
func CloseOtherBuffers(e *state.Editor) {
	closeBuffersWhere(e, "other buffers", func(int) bool { return true })
}

func CloseBuffersLeft(e *state.Editor) {
	closeBuffersWhere(e, "buffers to the left", func(i int) bool { return i < currentBuffer })
}

func CloseBuffersRight(e *state.Editor) {
	closeBuffersWhere(e, "buffers to the right", func(i int) bool { return i > currentBuffer })
}

// closeBuffersWhere drops every buffer the predicate names, never the one being
// edited. Unsaved changes refuse the whole move rather than half of it, so that
// a mistyped chord cannot cost some of them and report the rest.
func closeBuffersWhere(e *state.Editor, what string, drop func(int) bool) {
	SyncBuffer(e)

	doomed := make([]*Entry, 0, len(buffers))
	for i, entry := range buffers {
		if i == currentBuffer || !drop(i) {
			continue
		}
		if entry.Modified {
			e.StatusMsg = Unwritten(entry)
			return
		}
		doomed = append(doomed, entry)
	}

	if len(doomed) == 0 {
		e.StatusMsg = "no " + what + " to close"
		return
	}

	live := buffers[currentBuffer]
	kept := make([]*Entry, 0, len(buffers)-len(doomed))
	for _, entry := range buffers {
		if slices.Contains(doomed, entry) {
			entry.Buf.Close()
			continue
		}
		kept = append(kept, entry)
	}
	buffers, currentBuffer = kept, slices.Index(kept, live)
	for _, entry := range doomed {
		showBufferInstead(e, entry, live)
	}
	e.StatusMsg = fmt.Sprintf("%d buffers closed", len(doomed))
	if len(doomed) == 1 {
		e.StatusMsg = "1 buffer closed"
	}
}

// showBufferInstead is what keeps a window from being left showing a buffer
// that has just been closed.
func showBufferInstead(e *state.Editor, gone, replacement *Entry) {
	for _, w := range List(e) {
		if w.Entry == gone {
			w.Entry = replacement
		}
	}
}

func CloseAll(e *state.Editor) {
	SyncWindow(e)
	for _, entry := range buffers {
		entry.Buf.Close()
	}
}

// UnsavedBuffers is every buffer holding changes that are not on disk, the one
// being edited included, in the order the buffer line lists them. Quitting asks
// for this rather than for the current file alone: a buffer left behind by Tab
// or by the picker holds its changes just as the one on screen does.
func UnsavedBuffers(e *state.Editor) []*Entry {
	SyncBuffer(e)

	unsaved := make([]*Entry, 0, len(buffers))
	for _, entry := range buffers {
		if entry.Modified {
			unsaved = append(unsaved, entry)
		}
	}

	return unsaved
}

func Unwritten(entry *Entry) string {
	return fmt.Sprintf("E162: No write since last change for buffer %q", entry.Path)
}

func BufferIndex(path string) int {
	target := absPath(path)

	return slices.IndexFunc(buffers, func(entry *Entry) bool { return absPath(entry.Path) == target })
}

func absPath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}

	return abs
}

// ListBuffers is ':ls' folded onto the one row the status line has: the current
// buffer is marked with '%' and an unsaved one with a dot, as VIM marks them.
func ListBuffers(e *state.Editor) {
	SyncBuffer(e)

	labels := make([]string, 0, len(buffers))
	for i, entry := range buffers {
		mark := " "
		if i == currentBuffer {
			mark = "%"
		}
		label := mark + strconv.Itoa(i+1) + " " + filepath.Base(entry.Path)
		if entry.Modified {
			label += " " + string(tabbar.ModifiedMark)
		}
		labels = append(labels, label)
	}

	e.StatusMsg = strings.Join(labels, "  ")
}
