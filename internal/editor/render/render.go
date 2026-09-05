// Package render is one frame: the windows and what is in them, the buffer
// line above them and the status line below.
package render

import (
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/mattn/go-runewidth"
	"github.com/nsf/termbox-go"

	"github.com/ArditZubaku/tex/internal/editor/edit"
	"github.com/ArditZubaku/tex/internal/editor/explorer"
	"github.com/ArditZubaku/tex/internal/editor/find"
	"github.com/ArditZubaku/tex/internal/editor/screen"
	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/ArditZubaku/tex/internal/editor/tabbar"
	"github.com/ArditZubaku/tex/internal/editor/view"
	"github.com/ArditZubaku/tex/internal/gutter"
)

func Text(e *state.Editor) {
	bufLen := e.Buf.LineCount()
	gutterCols := gutter.Width(bufLen)
	textCols := e.Cols - gutterCols
	inBlock := blockStateBefore(e, e.OffsetRow)
	selected := edit.Selection(e)

	for row := 0; row < e.Rows; row++ {
		textBufRow := row + e.OffsetRow

		// Past end of buffer: draw line indicator once per row
		if textBufRow >= bufLen {
			termbox.SetCell(e.ScreenCol(0), e.ScreenRow(row), '*', e.Palette.EndOfBuffer, e.Palette.Background)
			continue
		}
		if textBufRow < 0 {
			continue
		}

		numberColor, background := e.Palette.LineNumber, e.Palette.Background
		if textBufRow == e.Row {
			numberColor, background = e.Palette.CursorLineNumber, e.Palette.CursorLineBg
			screen.Fill(e.ScreenCol(0), e.ScreenRow(row), e.Cols, e.Palette.Plain, e.Palette.CursorLineBg)
		}
		screen.Print(e.ScreenCol(0), e.ScreenRow(row), numberColor, background, gutter.Label(textBufRow, e.Row, gutterCols))

		line := e.Buf.Line(textBufRow)
		lineLen := len(line)

		var colors []termbox.Attribute
		colors, inBlock = lineColors(e, line, inBlock)
		hits := find.LineHits(e, textBufRow)

		// Render visible characters in current row
		for col := range textCols {
			textBufCol := col + e.OffsetCol
			if textBufCol < 0 {
				continue
			}
			inSelection := selected.Covers(textBufRow, textBufCol, lineLen)

			if textBufCol >= lineLen {
				if inSelection {
					termbox.SetCell(e.ScreenCol(gutterCols+col), e.ScreenRow(row), ' ', e.Palette.Plain, e.Palette.VisualBg)
				}
				continue
			}

			ch := line[textBufCol]
			if ch == '\t' {
				ch = ' '
			}

			foreground, cellBackground := e.Palette.Plain, background
			if colors != nil {
				foreground = colors[textBufCol]
			}
			if hits.Covers(textBufCol) {
				foreground, cellBackground = e.Palette.MatchFg, e.Palette.MatchBg
			}
			// the selection keeps the text's own colours and takes the
			// background, which is what makes it read as a band over them
			if inSelection {
				cellBackground = e.Palette.VisualBg
			}
			termbox.SetCell(e.ScreenCol(gutterCols+col), e.ScreenRow(row), ch, foreground, cellBackground)
		}
	}
}

func StatusBar(e *state.Editor) {
	if txt, ok := e.PromptStatus(); ok {
		screen.Print(0, e.StatusRow(), e.Palette.StatusFg, e.Palette.StatusBg, screen.Pad(txt, e.ScreenCols))
		return
	}

	if e.ExplorerOpen {
		screen.Print(0, e.StatusRow(), e.Palette.StatusFg, e.Palette.StatusBg, screen.Pad(explorer.Status(e), e.ScreenCols))
		return
	}

	var modeStatus, copyStatus, undoStatus, redoStatus, countStatus, fileStatus, cursorStatus string

	switch {
	case e.Mode == state.EditMode:
		modeStatus = " EDIT: "
	case e.Mode == state.VisualMode && e.VisualLine:
		modeStatus = " V-LINE: "
	case e.Mode == state.VisualMode:
		modeStatus = " VISUAL: "
	default:
		modeStatus = " VIEW: "
	}

	// the name alone: a file opened by the picker or by 'gd' carries the whole
	// path it was found at, which says nothing the buffer line does not
	name := filepath.Base(e.SourceFile)
	fileNameLen := min(len(name), 16)

	status := "saved"
	if e.Modified {
		status = "modified"
	}
	fileStatus = fmt.Sprintf("%s - %d lines %s", name[:fileNameLen], e.Buf.LineCount(), status)

	cursorStatus = fmt.Sprintf("Row %s, Col %s ", strconv.Itoa(e.Row+1), strconv.Itoa(e.Col+1))

	if !e.Clip.Empty() {
		copyStatus = " [Copy]"
	}

	if e.Hist.CanUndo() {
		undoStatus = " [Undo]"
	}

	if e.Hist.CanRedo() {
		redoStatus = " [Redo]"
	}

	if e.PendingCount > 0 {
		countStatus = strconv.Itoa(e.PendingCount) + " "
	}

	leftStatus := modeStatus + fileStatus + copyStatus + undoStatus + redoStatus
	rightStatus := countStatus + cursorStatus
	txt := screen.Pad(leftStatus, e.ScreenCols-runewidth.StringWidth(rightStatus)) + rightStatus

	screen.Print(0, e.StatusRow(), e.Palette.StatusFg, e.Palette.StatusBg, txt)
}

func Scroll(e *state.Editor) {
	textCols := e.Cols - gutter.Width(e.Buf.LineCount())

	if e.Row < e.OffsetRow {
		e.OffsetRow = e.Row
	}

	if e.Col < e.OffsetCol {
		e.OffsetCol = e.Col
	}

	if e.Row >= e.OffsetRow+e.Rows {
		e.OffsetRow = e.Row - e.Rows + 1
	}

	if e.Col >= e.OffsetCol+textCols {
		e.OffsetCol = e.Col - textCols + 1
	}
}

var tabs tabbar.Bar

// Reset drops the buffer line's own scroll, so that a test starts from the
// editor as it is before any file has been opened.
func Reset() { tabs = tabbar.Bar{} }

// Windows draws every window through the one renderer, pointing the editor at
// each in turn. Drawing changes nothing the editor is doing, so the current
// window's own state is put back at the end.
func Windows(e *state.Editor) {
	view.SyncWindow(e)
	live := e.Mode

	for _, w := range view.List(e) {
		// only the window being worked in draws its mode: a selection belongs
		// to the window it was made in, not to every view of the buffer
		e.Mode = live
		if w != view.Focused() {
			e.Mode = state.ReadMode
		}

		view.ShowWindow(e, w)
		if e.ExplorerOpen && w == view.Focused() {
			explorer.Draw(e)
			continue
		}
		Scroll(e)
		Text(e)
		w.OffsetRow, w.OffsetCol = e.OffsetRow, e.OffsetCol
	}

	e.Mode = live
	view.ShowWindow(e, view.Focused())
	separators(e)
}

func separators(e *state.Editor) {
	for _, s := range view.Separators() {
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
			termbox.SetCell(col, row, ch, e.Palette.Separator, e.Palette.Background)
		}
	}
}

func BufferLine(e *state.Editor) {
	tabs.Draw(0, e.ScreenCols, Tabs(e), view.Index(), &e.Palette)
}

func Tabs(e *state.Editor) []tabbar.Tab {
	view.SyncBuffer(e)

	open := make([]tabbar.Tab, 0, len(view.Buffers()))
	for _, entry := range view.Buffers() {
		open = append(open, tabbar.Tab{Name: filepath.Base(entry.Path), Modified: entry.Modified})
	}

	return open
}
