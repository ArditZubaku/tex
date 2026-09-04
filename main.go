package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mattn/go-runewidth"
	"github.com/nsf/termbox-go"
)

func main() {
	runEditor()
}

func runEditor() {
	if err := termbox.Init(); err != nil {
		slog.Error("Could not init termbox", "error", err)
		os.Exit(1)
	}
	// The 8-colour palette has no shade dark enough for the cursor line; every
	// colour below is unchanged by this, since termbox numbers the first
	// sixteen of the 256 the same way it numbers the eight.
	termbox.SetOutputMode(termbox.Output256)

	if len(os.Args) > 1 {
		sourceFile = os.Args[1]
		buf = openBuffer(sourceFile)
	} else {
		sourceFile = defaultFileName
		buf = newEmptyBuffer()
	}

	syntax = detectSyntax(sourceFile)

	for !quitting {
		// Fetch current screen dimensions
		screenCols, screenRows = termbox.Size()
		screenRows -= 1 + tabBarRows

		if screenCols < 80 {
			screenCols = 80
		}
		layoutWindows()

		if err := termbox.Clear(active.plain, active.background); err != nil {
			slog.Error("Could not clear terminal", "error", err)
			os.Exit(1) // TODO: Will think of something better in such a case
		}

		displayBufferLine()
		displayWindows()
		displayPicker()
		displayStatusBar()

		switch mode {
		case PromptMode:
			termbox.SetCursor(promptCol(), statusRow())
		case PickerMode:
			termbox.SetCursor(pickerCursorCol(), pickerRow()+1)
		case ExplorerMode:
			termbox.SetCursor(screenCol(0), explorerCursorRow())
		default:
			termbox.SetCursor(screenCol(currentCol-offsetCol+gutterWidth(buf.LineCount())), screenRow(currentRow-offsetRow))
		}

		if err := termbox.Flush(); err != nil {
			slog.Error("Could not show message", "error", err)
			os.Exit(1) // TODO: Will think of something better in such a case
		}

		processKeyPress()
	}

	closeBuffers()
	termbox.Close()
}

// Characters drawn over the band with SetChar keep the colours painted here.
func highlightRow(row int) {
	for col := range COLS {
		termbox.SetCell(screenCol(col), screenRow(row), ' ', active.plain, active.cursorLineBg)
	}
}

func displayTextBuffer() {
	bufLen := buf.LineCount()
	gutter := gutterWidth(bufLen)
	textCols := COLS - gutter
	inBlock := blockStateBefore(offsetRow)
	selected := visualSelection()

	for row := 0; row < ROWS; row++ {
		textBufRow := row + offsetRow

		// Past end of buffer: draw line indicator once per row
		if textBufRow >= bufLen {
			termbox.SetCell(screenCol(0), screenRow(row), '*', active.endOfBuffer, active.background)
			continue
		}
		if textBufRow < 0 {
			continue
		}

		numberColor, background := active.lineNumber, active.background
		if textBufRow == currentRow {
			numberColor, background = active.cursorLineNumber, active.cursorLineBg
			highlightRow(row)
		}
		printMessage(screenCol(0), screenRow(row), numberColor, background, lineNumberLabel(textBufRow, currentRow, gutter))

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
					termbox.SetCell(screenCol(gutter+col), screenRow(row), ' ', active.plain, active.visualBg)
				}
				continue
			}

			ch := line[textBufCol]
			if ch == '\t' {
				ch = ' '
			}

			foreground, cellBackground := active.plain, background
			if colors != nil {
				foreground = colors[textBufCol]
			}
			if hits.covers(textBufCol) {
				foreground, cellBackground = active.matchFg, active.matchBg
			}
			// the selection keeps the text's own colours and takes the
			// background, which is what makes it read as a band over them
			if inSelection {
				cellBackground = active.visualBg
			}
			termbox.SetCell(screenCol(gutter+col), screenRow(row), ch, foreground, cellBackground)
		}
	}
}

func displayStatusBar() {
	if txt, ok := promptStatus(); ok {
		printMessage(0, statusRow(), active.statusFg, active.statusBg, padTo(txt, screenCols))
		return
	}

	if explorerOpen {
		printMessage(0, statusRow(), active.statusFg, active.statusBg, padTo(explorerStatus(), screenCols))
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
	txt := padTo(leftStatus, screenCols-runewidth.StringWidth(rightStatus)) + rightStatus

	printMessage(0, statusRow(), active.statusFg, active.statusBg, txt)
}

func padTo(txt string, width int) string {
	return txt + strings.Repeat(" ", max(width-runewidth.StringWidth(txt), 0))
}

func printMessage(col, row int, fg, bg termbox.Attribute, msg string) {
	for _, ch := range msg {
		termbox.SetCell(col, row, ch, fg, bg)
		col += runewidth.RuneWidth(ch)
	}
}

func scrollTextBuffer() {
	textCols := COLS - gutterWidth(buf.LineCount())

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

func getKey() termbox.Event {
	var keyEvent termbox.Event

	switch event := termbox.PollEvent(); event.Type {
	case termbox.EventKey:
		keyEvent = event
	case termbox.EventError:
		panic(event.Err) // TODO: Will think of something better in such a case
	}

	return keyEvent
}
