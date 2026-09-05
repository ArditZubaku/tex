package editor

import (
	"testing"

	"github.com/ArditZubaku/tex/internal/buffer"
	"github.com/ArditZubaku/tex/internal/editor/filetree"
	"github.com/ArditZubaku/tex/internal/editor/history"
	"github.com/ArditZubaku/tex/internal/editor/prompt"
	"github.com/ArditZubaku/tex/internal/editor/register"
	"github.com/ArditZubaku/tex/internal/editor/render"
	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/ArditZubaku/tex/internal/editor/view"
	"github.com/ArditZubaku/tex/internal/search"
	"github.com/ArditZubaku/tex/internal/theme"
	"github.com/nsf/termbox-go"
)

func inReadMode(t *testing.T, content string, row, col int) *buffer.Buffer {
	t.Helper()

	b := atCursor(t, content, row, col)
	ed.Mode = state.ReadMode
	ed.PendingKeys, ed.PendingCount, ed.CmdCount = nil, 0, 1
	ed.Hist = history.History{}
	ed.Clip = register.Register{}
	ed.SearchPat, ed.SearchBack, ed.HlSearch = search.Pattern{}, false, false
	ed.Prompt, ed.StatusMsg = prompt.Line{}, ""
	ed.Quitting, ed.Palette = false, theme.Default()
	ed.ExplorerOpen, ed.Exp = false, filetree.Tree{}
	view.Reset()
	render.Reset()
	ed.WinRow, ed.WinCol = state.TabBarRows, 0

	return b
}

// press feeds keys the way the editor's own loop would, dispatching on the mode
// they arrive in as processKeyPress does.
func press(t *testing.T, keys string) {
	t.Helper()

	for _, ch := range keys {
		event := termbox.Event{Ch: ch}
		switch ch {
		case '\n':
			event = termbox.Event{Key: termbox.KeyEnter}
		case ' ':
			event = termbox.Event{Key: termbox.KeySpace}
		case '\b':
			event = termbox.Event{Key: termbox.KeyBackspace}
		case 27:
			event = termbox.Event{Key: termbox.KeyEsc}
		case 4:
			event = termbox.Event{Key: termbox.KeyCtrlD}
		case 21:
			event = termbox.Event{Key: termbox.KeyCtrlU}
		case '\t':
			event = termbox.Event{Key: termbox.KeyTab}
		}

		dispatchKey(event)
	}
}

func TestYankLineAndPaste(t *testing.T) {
	b := inReadMode(t, "foo\nbar\n", 0, 0)

	press(t, "yyp")

	wantLines(t, b, "foo", "foo", "bar")
	if ed.Row != 1 || ed.Col != 0 {
		t.Errorf("cursor at %d,%d, want 1,0", ed.Row, ed.Col)
	}
}

func TestPasteBeforePutsTheLineAbove(t *testing.T) {
	b := inReadMode(t, "foo\nbar\n", 1, 0)

	press(t, "yyP")

	wantLines(t, b, "foo", "bar", "bar")
	if ed.Row != 1 {
		t.Errorf("currentRow = %d, want 1", ed.Row)
	}
}

func TestCountedPaste(t *testing.T) {
	b := inReadMode(t, "foo\n", 0, 0)

	press(t, "yy3p")

	wantLines(t, b, "foo", "foo", "foo", "foo")
}

func TestCountedYankLine(t *testing.T) {
	b := inReadMode(t, "a\nb\nc\n", 0, 0)

	press(t, "2yyp")

	wantLines(t, b, "a", "a", "b", "b", "c")
}

func TestCharwiseYankAndPaste(t *testing.T) {
	b := inReadMode(t, "foo bar\n", 0, 0)

	press(t, "ywp")

	wantLines(t, b, "ffoo oo bar")
	if ed.Col != 4 {
		t.Errorf("currentCol = %d, want 4", ed.Col)
	}
}

func TestDeleteLineFillsTheRegister(t *testing.T) {
	b := inReadMode(t, "foo\nbar\n", 0, 0)

	press(t, "ddp")

	wantLines(t, b, "bar", "foo")
}

func TestDeleteRuneFillsTheRegister(t *testing.T) {
	b := inReadMode(t, "abcdef\n", 0, 0)

	press(t, "3xp")

	wantLines(t, b, "dabcef")
}

func TestPasteWithAnEmptyRegisterDoesNothing(t *testing.T) {
	b := inReadMode(t, "foo\n", 0, 0)

	press(t, "p")

	wantLines(t, b, "foo")
	if ed.Modified {
		t.Error("modified with nothing to paste")
	}
}
