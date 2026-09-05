package editor

import (
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/ArditZubaku/tex/internal/editor/edit"
	"github.com/ArditZubaku/tex/internal/editor/screen"
	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/ArditZubaku/tex/internal/editor/tabbar"
	"github.com/ArditZubaku/tex/internal/editor/view"
	"github.com/ArditZubaku/tex/internal/gutter"
	"github.com/mattn/go-runewidth"
	"github.com/nsf/termbox-go"
)

func displayTextBuffer() {
	bufLen := ed.Buf.LineCount()
	gutterCols := gutter.Width(bufLen)
	textCols := ed.Cols - gutterCols
	inBlock := blockStateBefore(ed.OffsetRow)
	selected := edit.Selection(ed)

	for row := 0; row < ed.Rows; row++ {
		textBufRow := row + ed.OffsetRow

		// Past end of buffer: draw line indicator once per row
		if textBufRow >= bufLen {
			termbox.SetCell(ed.ScreenCol(0), ed.ScreenRow(row), '*', ed.Palette.EndOfBuffer, ed.Palette.Background)
			continue
		}
		if textBufRow < 0 {
			continue
		}

		numberColor, background := ed.Palette.LineNumber, ed.Palette.Background
		if textBufRow == ed.Row {
			numberColor, background = ed.Palette.CursorLineNumber, ed.Palette.CursorLineBg
			screen.Fill(ed.ScreenCol(0), ed.ScreenRow(row), ed.Cols, ed.Palette.Plain, ed.Palette.CursorLineBg)
		}
		screen.Print(ed.ScreenCol(0), ed.ScreenRow(row), numberColor, background, gutter.Label(textBufRow, ed.Row, gutterCols))

		line := ed.Buf.Line(textBufRow)
		lineLen := len(line)

		var colors []termbox.Attribute
		colors, inBlock = lineColors(line, inBlock)
		hits := lineHits(textBufRow)

		// Render visible characters in current row
		for col := range textCols {
			textBufCol := col + ed.OffsetCol
			if textBufCol < 0 {
				continue
			}
			inSelection := selected.Covers(textBufRow, textBufCol, lineLen)

			if textBufCol >= lineLen {
				if inSelection {
					termbox.SetCell(ed.ScreenCol(gutterCols+col), ed.ScreenRow(row), ' ', ed.Palette.Plain, ed.Palette.VisualBg)
				}
				continue
			}

			ch := line[textBufCol]
			if ch == '\t' {
				ch = ' '
			}

			foreground, cellBackground := ed.Palette.Plain, background
			if colors != nil {
				foreground = colors[textBufCol]
			}
			if hits.covers(textBufCol) {
				foreground, cellBackground = ed.Palette.MatchFg, ed.Palette.MatchBg
			}
			// the selection keeps the text's own colours and takes the
			// background, which is what makes it read as a band over them
			if inSelection {
				cellBackground = ed.Palette.VisualBg
			}
			termbox.SetCell(ed.ScreenCol(gutterCols+col), ed.ScreenRow(row), ch, foreground, cellBackground)
		}
	}
}

func displayStatusBar() {
	if txt, ok := promptStatus(); ok {
		screen.Print(0, ed.StatusRow(), ed.Palette.StatusFg, ed.Palette.StatusBg, screen.Pad(txt, ed.ScreenCols))
		return
	}

	if ed.ExplorerOpen {
		screen.Print(0, ed.StatusRow(), ed.Palette.StatusFg, ed.Palette.StatusBg, screen.Pad(explorerStatus(), ed.ScreenCols))
		return
	}

	var modeStatus, copyStatus, undoStatus, redoStatus, countStatus, fileStatus, cursorStatus string

	switch {
	case ed.Mode == state.EditMode:
		modeStatus = " EDIT: "
	case ed.Mode == state.VisualMode && ed.VisualLine:
		modeStatus = " V-LINE: "
	case ed.Mode == state.VisualMode:
		modeStatus = " VISUAL: "
	default:
		modeStatus = " VIEW: "
	}

	// the name alone: a file opened by the picker or by 'gd' carries the whole
	// path it was found at, which says nothing the buffer line does not
	name := filepath.Base(ed.SourceFile)
	fileNameLen := min(len(name), 16)

	status := "saved"
	if ed.Modified {
		status = "modified"
	}
	fileStatus = fmt.Sprintf("%s - %d lines %s", name[:fileNameLen], ed.Buf.LineCount(), status)

	cursorStatus = fmt.Sprintf("Row %s, Col %s ", strconv.Itoa(ed.Row+1), strconv.Itoa(ed.Col+1))

	if !ed.Clip.Empty() {
		copyStatus = " [Copy]"
	}

	if ed.Hist.CanUndo() {
		undoStatus = " [Undo]"
	}

	if ed.Hist.CanRedo() {
		redoStatus = " [Redo]"
	}

	if ed.PendingCount > 0 {
		countStatus = strconv.Itoa(ed.PendingCount) + " "
	}

	leftStatus := modeStatus + fileStatus + copyStatus + undoStatus + redoStatus
	rightStatus := countStatus + cursorStatus
	txt := screen.Pad(leftStatus, ed.ScreenCols-runewidth.StringWidth(rightStatus)) + rightStatus

	screen.Print(0, ed.StatusRow(), ed.Palette.StatusFg, ed.Palette.StatusBg, txt)
}

func scrollTextBuffer() {
	textCols := ed.Cols - gutter.Width(ed.Buf.LineCount())

	if ed.Row < ed.OffsetRow {
		ed.OffsetRow = ed.Row
	}

	if ed.Col < ed.OffsetCol {
		ed.OffsetCol = ed.Col
	}

	if ed.Row >= ed.OffsetRow+ed.Rows {
		ed.OffsetRow = ed.Row - ed.Rows + 1
	}

	if ed.Col >= ed.OffsetCol+textCols {
		ed.OffsetCol = ed.Col - textCols + 1
	}
}

var tabs tabbar.Bar

// displayWindows draws every window through the one renderer, pointing the
// globals at each in turn. Drawing changes nothing the editor is doing, so the
// current window's own state is put back at the end.
func displayWindows() {
	view.SyncWindow(ed)
	live := ed.Mode

	for _, w := range view.List(ed) {
		// only the window being worked in draws its mode: a selection belongs
		// to the window it was made in, not to every view of the buffer
		ed.Mode = live
		if w != view.Focused() {
			ed.Mode = state.ReadMode
		}

		view.ShowWindow(ed, w)
		if ed.ExplorerOpen && w == view.Focused() {
			displayExplorer()
			continue
		}
		scrollTextBuffer()
		displayTextBuffer()
		w.OffsetRow, w.OffsetCol = ed.OffsetRow, ed.OffsetCol
	}

	ed.Mode = live
	view.ShowWindow(ed, view.Focused())
	displaySeparators()
}

func displaySeparators() {
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
			termbox.SetCell(col, row, ch, ed.Palette.Separator, ed.Palette.Background)
		}
	}
}

func displayBufferLine() {
	tabs.Draw(0, ed.ScreenCols, openTabs(), view.Index(), &ed.Palette)
}

func openTabs() []tabbar.Tab {
	view.SyncBuffer(ed)

	open := make([]tabbar.Tab, 0, len(view.Buffers()))
	for _, entry := range view.Buffers() {
		open = append(open, tabbar.Tab{Name: filepath.Base(entry.Path), Modified: entry.Modified})
	}

	return open
}
