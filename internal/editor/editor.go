package editor

import (
	"log/slog"
	"os"

	"github.com/nsf/termbox-go"

	"github.com/ArditZubaku/tex/internal/buffer"
	"github.com/ArditZubaku/tex/internal/editor/diag"
	"github.com/ArditZubaku/tex/internal/editor/explorer"
	"github.com/ArditZubaku/tex/internal/editor/find"
	"github.com/ArditZubaku/tex/internal/editor/keys"
	"github.com/ArditZubaku/tex/internal/editor/render"
	"github.com/ArditZubaku/tex/internal/editor/screen"
	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/ArditZubaku/tex/internal/editor/view"
	"github.com/ArditZubaku/tex/internal/lsp"
	"github.com/ArditZubaku/tex/internal/syntax"
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

	ed := state.New()

	if len(args) > 0 {
		ed.SourceFile = args[0]
		ed.Buf = buffer.Open(ed.SourceFile)
	} else {
		ed.SourceFile = state.DefaultFileName
		ed.Buf = buffer.NewEmpty()
	}

	ed.Lang = syntax.Detect(ed.SourceFile)
	lsp.Wake(screen.StartWaker())
	// Nothing below state may reach back up to what holds the diagnostics, and
	// nothing in the language server may reach the editor at all, so the loop
	// is what joins them.
	state.OnEdit = diag.Edited
	lsp.Published = diag.Publish
	lsp.Gone = func(err error) {
		diag.Reset()
		ed.Note.Show("language server", err.Error())
	}

	for !ed.Quitting {
		// Fetch current screen dimensions
		ed.ScreenCols, ed.ScreenRows = termbox.Size()
		ed.ScreenRows -= 1 + state.TabBarRows

		if ed.ScreenCols < 80 {
			ed.ScreenCols = 80
		}
		view.Layout(ed)
		lsp.Poll()
		tellServer(ed)

		if err := termbox.Clear(ed.Palette.Plain, ed.Palette.Background); err != nil {
			slog.Error("Could not clear terminal", "error", err)
			os.Exit(1) // TODO: Will think of something better in such a case
		}

		render.BufferLine(ed)
		render.Windows(ed)
		find.DrawPicker(ed)
		ed.Note.Draw(ed.ScreenArea(), &ed.Palette)
		render.StatusBar(ed)

		switch ed.Mode {
		case state.PromptMode:
			termbox.SetCursor(ed.PromptCol(), ed.StatusRow())
		case state.PickerMode:
			termbox.SetCursor(ed.Pick.CursorCol(ed.ScreenArea()), ed.Pick.CursorRow(ed.ScreenArea()))
		case state.ExplorerMode:
			termbox.SetCursor(ed.ScreenCol(0), explorer.CursorRow(ed))
		default:
			termbox.SetCursor(ed.CursorScreenCol(), ed.CursorScreenRow())
		}

		if err := termbox.Flush(); err != nil {
			slog.Error("Could not show message", "error", err)
			os.Exit(1) // TODO: Will think of something better in such a case
		}

		keys.Read(ed)
	}

	lsp.Stop()
	view.CloseAll(ed)
	termbox.Close()
}

// open is the list handed to the reconciler, kept between frames so that saying
// what is open costs nothing per frame.
var open []lsp.File

// tellServer is where the editor's own idea of what is open meets the server's.
// It is the loop that does it rather than the edit commands because reading a
// buffer moves its window, so only this goroutine may, and because a frame is
// the one place every way text can have changed has already happened.
func tellServer(e *state.Editor) {
	view.SyncBuffer(e)

	open = open[:0]
	for _, entry := range view.Buffers() {
		open = append(open, lsp.File{Path: entry.Path, Buf: entry.Buf, Modified: entry.Modified})
	}
	lsp.Sync(open)
}
