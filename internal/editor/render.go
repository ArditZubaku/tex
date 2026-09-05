package editor

import (
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/ArditZubaku/tex/internal/editor/screen"
	"github.com/ArditZubaku/tex/internal/gutter"
	"github.com/mattn/go-runewidth"
	"github.com/nsf/termbox-go"
)

// Characters drawn over the band with SetChar keep the colours painted here.
func highlightRow(row int) {
	for col := range COLS {
		termbox.SetCell(screenCol(col), screenRow(row), ' ', active.Plain, active.CursorLineBg)
	}
}

func displayTextBuffer() {
	bufLen := buf.LineCount()
	gutterCols := gutter.Width(bufLen)
	textCols := COLS - gutterCols
	inBlock := blockStateBefore(offsetRow)
	selected := visualSelection()

	for row := 0; row < ROWS; row++ {
		textBufRow := row + offsetRow

		// Past end of buffer: draw line indicator once per row
		if textBufRow >= bufLen {
			termbox.SetCell(screenCol(0), screenRow(row), '*', active.EndOfBuffer, active.Background)
			continue
		}
		if textBufRow < 0 {
			continue
		}

		numberColor, background := active.LineNumber, active.Background
		if textBufRow == currentRow {
			numberColor, background = active.CursorLineNumber, active.CursorLineBg
			highlightRow(row)
		}
		screen.Print(screenCol(0), screenRow(row), numberColor, background, gutter.Label(textBufRow, currentRow, gutterCols))

		line := buf.Line(textBufRow)
		lineLen := len(line)

		var colors []termbox.Attribute
		colors, inBlock = lineColors(line, inBlock)
		hits := lineHits(textBufRow)

		// Render visible characters in current row
		for col := range textCols {
			textBufCol := col + offsetCol
			if textBufCol < 0 {
				continue
			}
			inSelection := selected.covers(textBufRow, textBufCol, lineLen)

			if textBufCol >= lineLen {
				if inSelection {
					termbox.SetCell(screenCol(gutterCols+col), screenRow(row), ' ', active.Plain, active.VisualBg)
				}
				continue
			}

			ch := line[textBufCol]
			if ch == '\t' {
				ch = ' '
			}

			foreground, cellBackground := active.Plain, background
			if colors != nil {
				foreground = colors[textBufCol]
			}
			if hits.covers(textBufCol) {
				foreground, cellBackground = active.MatchFg, active.MatchBg
			}
			// the selection keeps the text's own colours and takes the
			// background, which is what makes it read as a band over them
			if inSelection {
				cellBackground = active.VisualBg
			}
			termbox.SetCell(screenCol(gutterCols+col), screenRow(row), ch, foreground, cellBackground)
		}
	}
}

func displayStatusBar() {
	if txt, ok := promptStatus(); ok {
		screen.Print(0, statusRow(), active.StatusFg, active.StatusBg, screen.Pad(txt, screenCols))
		return
	}

	if explorerOpen {
		screen.Print(0, statusRow(), active.StatusFg, active.StatusBg, screen.Pad(explorerStatus(), screenCols))
		return
	}

	var modeStatus, copyStatus, undoStatus, redoStatus, countStatus, fileStatus, cursorStatus string

	switch {
	case mode == EditMode:
		modeStatus = " EDIT: "
	case mode == VisualMode && visualLine:
		modeStatus = " V-LINE: "
	case mode == VisualMode:
		modeStatus = " VISUAL: "
	default:
		modeStatus = " VIEW: "
	}

	// the name alone: a file opened by the picker or by 'gd' carries the whole
	// path it was found at, which says nothing the buffer line does not
	name := filepath.Base(sourceFile)
	fileNameLen := min(len(name), 16)

	status := "saved"
	if modified {
		status = "modified"
	}
	fileStatus = fmt.Sprintf("%s - %d lines %s", name[:fileNameLen], buf.LineCount(), status)

	cursorStatus = fmt.Sprintf("Row %s, Col %s ", strconv.Itoa(currentRow+1), strconv.Itoa(currentCol+1))

	if !clipboard.empty() {
		copyStatus = " [Copy]"
	}

	if len(undoStack) > 0 {
		undoStatus = " [Undo]"
	}

	if len(redoStack) > 0 {
		redoStatus = " [Redo]"
	}

	if pendingCount > 0 {
		countStatus = strconv.Itoa(pendingCount) + " "
	}

	leftStatus := modeStatus + fileStatus + copyStatus + undoStatus + redoStatus
	rightStatus := countStatus + cursorStatus
	txt := screen.Pad(leftStatus, screenCols-runewidth.StringWidth(rightStatus)) + rightStatus

	screen.Print(0, statusRow(), active.StatusFg, active.StatusBg, txt)
}

func scrollTextBuffer() {
	textCols := COLS - gutter.Width(buf.LineCount())

	if currentRow < offsetRow {
		offsetRow = currentRow
	}

	if currentCol < offsetCol {
		offsetCol = currentCol
	}

	if currentRow >= offsetRow+ROWS {
		offsetRow = currentRow - ROWS + 1
	}

	if currentCol >= offsetCol+textCols {
		offsetCol = currentCol - textCols + 1
	}
}
