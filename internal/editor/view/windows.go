package view

import (
	"slices"

	"github.com/nsf/termbox-go"

	"github.com/ArditZubaku/tex/internal/editor/edit"
	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/ArditZubaku/tex/internal/layout"
)

// A Window is one view of a buffer: which buffer it shows, where its cursor and
// its viewport are in it, and the rectangle of the screen it draws in. The
// editor's globals are the live state of the current window, so every command
// goes on working on globals and knows nothing about the split it runs in.
type Window struct {
	Entry                *Entry
	CursorRow, CursorCol int
	OffsetRow, OffsetCol int
	Rect                 layout.Rect
}

var (
	root    *layout.Tree[*Window]
	current *Window

	separators []layout.Separator
)

func Focused() *Window { return current }

// Reset drops the buffer list and the window tree, so that a test starts from
// the editor as it is before any file has been opened.
func Reset() {
	buffers, currentBuffer, altPath = nil, 0, ""
	root, current, separators = nil, nil, nil
	dragging.on = false
}

func Separators() []layout.Separator { return separators }

// A window narrower than its gutter and a few columns of text, or shorter than
// a line of text between two edges, is no use to anybody: VIM refuses the split
// rather than making one.
const (
	minWindowRows = 3
	minWindowCols = 20
)

// CurrentWindow adopts the whole area as one window when nothing has been split
// yet, so that the editor always has a window without main having to seed one.
func CurrentWindow(e *state.Editor) *Window {
	if current == nil {
		current = &Window{
			Entry:     CurrentEntry(e),
			CursorRow: e.Row, CursorCol: e.Col,
			OffsetRow: e.OffsetRow, OffsetCol: e.OffsetCol,
			Rect: layout.Rect{Row: state.TabBarRows, Rows: e.Rows, Cols: e.Cols},
		}
		root = layout.Leaf(current)
	}

	return current
}

// SyncWindow copies the editor's own state back onto the window it belongs to,
// so that leaving a window and coming back to it finds it as it was left.
func SyncWindow(e *state.Editor) {
	w := CurrentWindow(e)
	w.Entry = CurrentEntry(e)
	w.CursorRow, w.CursorCol = e.Row, e.Col
	w.OffsetRow, w.OffsetCol = e.OffsetRow, e.OffsetCol
	SyncBuffer(e)
}

// ShowWindow points the globals at a window without making it the current one,
// which is what lets one pass draw every window through the same renderer. It
// leaves the buffer's history alone: only the window being worked in owns that.
func ShowWindow(e *state.Editor, w *Window) {
	e.Buf, e.SourceFile, e.Lang = w.Entry.Buf, w.Entry.Path, w.Entry.Lang
	e.Row, e.Col = w.CursorRow, w.CursorCol
	e.OffsetRow, e.OffsetCol = w.OffsetRow, w.OffsetCol
	applyRect(e, w)
}

// applyRect is the part of a window the layout pass hands over on its own: the
// room it has to draw in, which a resize changes under a window that is
// otherwise untouched.
func applyRect(e *state.Editor, w *Window) {
	e.WinRow, e.WinCol, e.Rows, e.Cols = w.Rect.Row, w.Rect.Col, w.Rect.Rows, w.Rect.Cols
}

// applyWindow makes a window the one being worked in: it takes the buffer's
// history with it, and drops a selection, which belonged to the window left.
func applyWindow(e *state.Editor, w *Window) {
	if e.Mode == state.VisualMode {
		edit.ExitVisual(e)
	}
	current, currentBuffer = w, max(slices.Index(buffers, w.Entry), 0)
	applyEntry(e, w.Entry)
	ShowWindow(e, w)
	e.ClampCol()
}

func focusWindow(e *state.Editor, w *Window) {
	if w == nil || w == CurrentWindow(e) {
		return
	}

	SyncWindow(e)
	applyWindow(e, w)
}

// List walks the leaves in the order they lie on screen, adopting the
// area as one window when nothing has been laid out yet.
func List(e *state.Editor) []*Window {
	CurrentWindow(e)

	return root.Leaves(nil)
}

func SplitBelow(e *state.Editor) { Split(e, false) }
func SplitRight(e *state.Editor) { Split(e, true) }

// Split is ':split' and ':vsplit': the new window takes half the room of
// the one it splits and the cursor moves into it, showing the same buffer at
// the same place — below or to the right, as VIM does with 'splitbelow' and
// 'splitright' set, which is how LazyVim has them.
func Split(e *state.Editor, vertical bool) bool {
	w := CurrentWindow(e)
	if (vertical && w.Rect.Cols <= 2*minWindowCols) || (!vertical && w.Rect.Rows <= 2*minWindowRows) {
		e.StatusMsg = "E36: Not enough room"
		return false
	}

	SyncWindow(e)
	fresh := &Window{
		Entry:     w.Entry,
		CursorRow: w.CursorRow, CursorCol: w.CursorCol,
		OffsetRow: w.OffsetRow, OffsetCol: w.OffsetCol,
	}
	root = root.InsertBeside(w, vertical, fresh)
	Layout(e)
	applyWindow(e, fresh)

	return true
}

// CloseWindow is ':close' and 'Ctrl-W c'. The buffer it was showing stays in
// the buffer list, unsaved changes and all, so closing a window loses nothing.
func CloseWindow(e *state.Editor) {
	list := List(e)
	if len(list) < 2 {
		e.StatusMsg = "E444: Cannot close last window"
		return
	}

	SyncWindow(e)
	next := slices.Index(list, current)
	root = root.Prune(current)
	Layout(e)

	list = List(e)
	applyWindow(e, list[min(next, len(list)-1)])
}

// OnlyWindow is ':only' and 'Ctrl-W o': every other window is closed, leaving
// the one being worked in with the whole area to itself.
func OnlyWindow(e *state.Editor) {
	SyncWindow(e)
	root = layout.Leaf(current)
	Layout(e)
	applyWindow(e, current)
}

// A drag is the separator the pointer took hold of, and where that separator
// stands now. What is followed between one event and the next is the edge
// rather than the pointer: a pointer run past what the windows either side of
// it can give would otherwise come back with the edge owing it the difference.
var dragging struct {
	on       bool
	vertical bool
	row, col int
}

// Mouse is the pointer on the lines between the windows: pressing on one takes
// hold of it, moving with the button down drags it, and letting go lets it be.
// Anything else drops what was being dragged, so a drag cannot outlive the
// button that started it.
func Mouse(e *state.Editor, event termbox.Event) {
	switch {
	case event.Key != termbox.MouseLeft:
		dragging.on = false
	case event.Mod&termbox.ModMotion == 0:
		takeEdge(e, event.MouseY, event.MouseX)
	default:
		dragEdge(e, event.MouseY, event.MouseX)
	}
}

func takeEdge(e *state.Editor, row, col int) {
	CurrentWindow(e)
	vertical, ok := root.EdgeAt(e.ScreenArea(), row, col)
	dragging.on, dragging.vertical, dragging.row, dragging.col = ok, vertical, row, col
}

func dragEdge(e *state.Editor, row, col int) {
	if !dragging.on {
		return
	}

	delta, least := row-dragging.row, minWindowRows
	if dragging.vertical {
		delta, least = col-dragging.col, minWindowCols
	}
	if delta == 0 {
		return
	}

	moved := root.MoveEdge(e.ScreenArea(), dragging.row, dragging.col, delta, least)
	if moved == 0 {
		return
	}
	if dragging.vertical {
		dragging.col += moved
	} else {
		dragging.row += moved
	}
	Layout(e)
}

// MoveKeys are Ctrl-hjkl, which is all it takes to leave a window. Only outside
// Edit mode: Ctrl-H is also the Backspace that terminals sending 0x08 rather
// than 0x7F give, and typing has first call on it.
var MoveKeys = map[termbox.Key]func(*state.Editor){
	termbox.KeyCtrlH: FocusLeft,
	termbox.KeyCtrlJ: FocusDown,
	termbox.KeyCtrlK: FocusUp,
	termbox.KeyCtrlL: FocusRight,
}

func FocusLeft(e *state.Editor)  { focusDirection(e, 0, -1) }
func FocusRight(e *state.Editor) { focusDirection(e, 0, 1) }
func FocusUp(e *state.Editor)    { focusDirection(e, -1, 0) }
func FocusDown(e *state.Editor)  { focusDirection(e, 1, 0) }

// focusDirection is 'Ctrl-W h' and its three friends: the nearest window on
// that side of the current one, of those it shares any rows (or columns) with,
// so that a move up cannot land in a window off to one side.
func focusDirection(e *state.Editor, dRow, dCol int) {
	w := CurrentWindow(e)

	var best *Window
	bestGap := 0
	for _, other := range List(e) {
		if other == w {
			continue
		}

		gap, ok := w.Rect.Gap(other.Rect, dRow, dCol)
		if !ok {
			continue
		}
		if best == nil || gap < bestGap {
			best, bestGap = other, gap
		}
	}

	focusWindow(e, best)
}

// Layout hands every window its rectangle, which is what the renderer
// and the directional moves both read; it runs once a frame, since the terminal
// may have been resized since the last one.
func Layout(e *state.Editor) {
	CurrentWindow(e)
	separators = root.Place(
		layout.Rect{Row: state.TabBarRows, Rows: e.ScreenRows, Cols: e.ScreenCols},
		separators[:0],
		func(w *Window, rect layout.Rect) { w.Rect = rect },
	)
	applyRect(e, current)
}
