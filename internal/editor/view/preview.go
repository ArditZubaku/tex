package view

import (
	"github.com/ArditZubaku/tex/internal/buffer"
	"github.com/ArditZubaku/tex/internal/editor/preview"
	"github.com/ArditZubaku/tex/internal/editor/state"
)

// There is at most one preview open at a time, moved to whichever markdown
// buffer is current rather than one kept per buffer: tabbing between several
// markdown files would otherwise fill the screen with their panes.
var (
	previewWin    *Window
	previewSource *Entry
	previewWidth  int
	dismissed     = map[*Entry]bool{}
)

// MaybeOpenPreview is every place a different buffer becomes the current one:
// glow renders it beside the edit the moment it is a markdown file, unless
// this file's preview was closed already, which is what dismissed remembers.
func MaybeOpenPreview(e *state.Editor) {
	entry := CurrentEntry(e)
	if dismissed[entry] || previewSource == entry {
		return
	}

	tryOpenPreview(e, entry)
}

// TogglePreview is '<leader>mp': open glow's rendering of the current buffer
// beside it, or close it if that is already showing. Closing it here is what
// sets dismissed, so an automatic switch back to this file does not reopen
// what was just asked to go away.
func TogglePreview(e *state.Editor) {
	entry := CurrentEntry(e)
	if previewWin != nil && previewSource == entry {
		closePreview(e)
		dismissed[entry] = true
		return
	}

	dismissed[entry] = false
	tryOpenPreview(e, entry)
}

func tryOpenPreview(e *state.Editor, entry *Entry) {
	if !preview.Available() || !preview.IsMarkdown(entry.Path) {
		return
	}

	openOrMovePreview(e, entry)
}

func openOrMovePreview(e *state.Editor, entry *Entry) {
	w := CurrentWindow(e)
	if previewWin == nil {
		if w.Rect.Cols <= 2*minWindowCols {
			return
		}

		fresh := &Window{Entry: &Entry{ReadOnly: true, Buf: buffer.NewEmpty()}}
		root = root.InsertBeside(w, true, fresh)
		Layout(e)
		previewWin = fresh
	}

	previewSource = entry
	renderPreviewInto(previewWin, entry)
}

// RefreshPreview is ':w' finding glow's rendering out of date: only the file
// it belongs to asks for it, so saving some other buffer does not re-run glow
// over the one a split away.
func RefreshPreview(e *state.Editor) {
	entry := CurrentEntry(e)
	if previewWin == nil || previewSource != entry {
		return
	}

	renderPreviewInto(previewWin, entry)
}

// renderPreviewInto leaves the pane showing whatever it last held rather than
// going blank when glow itself fails, a timeout or a file it refuses included.
func renderPreviewInto(w *Window, source *Entry) {
	rows, err := preview.Render(source.Path, max(w.Rect.Cols, 1))
	if err != nil {
		return
	}

	w.Entry.Path, w.Entry.PreviewCells = source.Path, rows
	w.CursorRow, w.CursorCol, w.OffsetRow, w.OffsetCol = 0, 0, 0, 0
	previewWidth = w.Rect.Cols
}

// SettlePreviewResize is a separator drag letting go: glow wrapped its
// rendering to the pane's width as of the last render, which dragging leaves
// wrong until this catches it up. It runs only once the drag is over rather
// than on every step of it, since re-running glow is not free enough to pay
// for on every column the pointer crosses.
func SettlePreviewResize(e *state.Editor) {
	if previewWin == nil || previewWin.Rect.Cols == previewWidth {
		return
	}

	renderPreviewInto(previewWin, previewSource)
}

func closePreview(e *state.Editor) {
	if previewWin == nil {
		return
	}

	root = root.Prune(previewWin)
	Layout(e)
	previewWin, previewSource = nil, nil
}

// PreviewScrollOffset is the row glow's rendering starts drawing from: no key
// ever reaches this pane to scroll it on its own, so it follows the fraction
// of the way down the source buffer the cursor already is.
func PreviewScrollOffset(w *Window) int {
	if previewSource == nil {
		return 0
	}

	total := previewSource.Buf.LineCount()
	room := len(w.Entry.PreviewCells) - w.Rect.Rows
	if total <= 1 || room <= 0 {
		return 0
	}

	frac := float64(previewSource.Row) / float64(total-1)

	return int(frac * float64(room))
}

// realCount is how many windows are the user's own splits rather than glow's
// pane, which is what ':q' and ':close' care about: the preview should never
// count as a second window worth refusing to close the last of.
func realCount(list []*Window) int {
	n := 0
	for _, w := range list {
		if !w.Entry.ReadOnly {
			n++
		}
	}

	return n
}

func RealWindowCount(e *state.Editor) int {
	return realCount(List(e))
}
