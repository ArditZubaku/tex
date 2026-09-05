package editor

import (
	"testing"

	"github.com/ArditZubaku/txi/internal/buffer"
	"github.com/ArditZubaku/txi/internal/theme"
	"github.com/nsf/termbox-go"
)

func inReadMode(t *testing.T, content string, row, col int) *buffer.Buffer {
	t.Helper()

	b := atCursor(t, content, row, col)
	mode = ReadMode
	pendingKeys, pendingCount, cmdCount = nil, 0, 1
	undoStack, redoStack, pendingChange = nil, nil, nil
	clipboard = register{}
	searchPat, searchBack, hlSearch = pattern{}, false, false
	promptChar, promptInput, statusMsg = 0, nil, ""
	quitting, active = false, theme.Default()
	explorerOpen, explorerHidden, explorerFilter = false, false, ""
	buffers, currentBuffer, tabBarOffset = nil, 0, 0
	root, current, winRow, winCol = nil, nil, tabBarRows, 0

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
	if currentRow != 1 || currentCol != 0 {
		t.Errorf("cursor at %d,%d, want 1,0", currentRow, currentCol)
	}
}

func TestPasteBeforePutsTheLineAbove(t *testing.T) {
	b := inReadMode(t, "foo\nbar\n", 1, 0)

	press(t, "yyP")

	wantLines(t, b, "foo", "bar", "bar")
	if currentRow != 1 {
		t.Errorf("currentRow = %d, want 1", currentRow)
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
	if currentCol != 4 {
		t.Errorf("currentCol = %d, want 4", currentCol)
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
	if modified {
		t.Error("modified with nothing to paste")
	}
}
