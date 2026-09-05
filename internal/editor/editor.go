package editor

import (
	"log/slog"
	"os"

	"github.com/ArditZubaku/tex/internal/buffer"
	"github.com/ArditZubaku/tex/internal/gutter"
	"github.com/ArditZubaku/tex/internal/syntax"
	"github.com/nsf/termbox-go"
)

// Run is the editor: it takes over the terminal, draws and dispatches keys
// until something quits, and hands the terminal back.
func Run(args []string) {
	if err := termbox.Init(); err != nil {
		slog.Error("Could not init termbox", "error", err)
		os.Exit(1)
	}
	// The 8-colour palette has no shade dark enough for the cursor line; every
	// colour below is unchanged by this, since termbox numbers the first
	// sixteen of the 256 the same way it numbers the eight.
	termbox.SetOutputMode(termbox.Output256)

	if len(args) > 0 {
		sourceFile = args[0]
		buf = buffer.Open(sourceFile)
	} else {
		sourceFile = defaultFileName
		buf = buffer.NewEmpty()
	}

	lang = syntax.Detect(sourceFile)

	for !quitting {
		// Fetch current screen dimensions
		screenCols, screenRows = termbox.Size()
		screenRows -= 1 + tabBarRows

		if screenCols < 80 {
			screenCols = 80
		}
		layoutWindows()

		if err := termbox.Clear(active.Plain, active.Background); err != nil {
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
			termbox.SetCursor(pick.CursorCol(screenArea()), pick.CursorRow(screenArea()))
		case ExplorerMode:
			termbox.SetCursor(screenCol(0), explorerCursorRow())
		default:
			termbox.SetCursor(screenCol(currentCol-offsetCol+gutter.Width(buf.LineCount())), screenRow(currentRow-offsetRow))
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
