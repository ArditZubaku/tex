package editor

import (
	"log/slog"
	"os"

	"github.com/ArditZubaku/tex/internal/buffer"
	"github.com/ArditZubaku/tex/internal/editor/state"
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
		ed.SourceFile = args[0]
		ed.Buf = buffer.Open(ed.SourceFile)
	} else {
		ed.SourceFile = state.DefaultFileName
		ed.Buf = buffer.NewEmpty()
	}

	ed.Lang = syntax.Detect(ed.SourceFile)

	for !ed.Quitting {
		// Fetch current screen dimensions
		ed.ScreenCols, ed.ScreenRows = termbox.Size()
		ed.ScreenRows -= 1 + state.TabBarRows

		if ed.ScreenCols < 80 {
			ed.ScreenCols = 80
		}
		layoutWindows()

		if err := termbox.Clear(ed.Palette.Plain, ed.Palette.Background); err != nil {
			slog.Error("Could not clear terminal", "error", err)
			os.Exit(1) // TODO: Will think of something better in such a case
		}

		displayBufferLine()
		displayWindows()
		displayPicker()
		displayStatusBar()

		switch ed.Mode {
		case state.PromptMode:
			termbox.SetCursor(promptCol(), ed.StatusRow())
		case state.PickerMode:
			termbox.SetCursor(ed.Pick.CursorCol(ed.ScreenArea()), ed.Pick.CursorRow(ed.ScreenArea()))
		case state.ExplorerMode:
			termbox.SetCursor(ed.ScreenCol(0), explorerCursorRow())
		default:
			termbox.SetCursor(ed.ScreenCol(ed.Col-ed.OffsetCol+gutter.Width(ed.Buf.LineCount())), ed.ScreenRow(ed.Row-ed.OffsetRow))
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
