// Package edtest is the harness the editor's own tests share: a buffer written
// to a temporary file, an editor pointed at it, and the assertion that its
// lines read back as expected.
package edtest

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nsf/termbox-go"

	"github.com/ArditZubaku/tex/internal/buffer"
	"github.com/ArditZubaku/tex/internal/editor/filetree"
	"github.com/ArditZubaku/tex/internal/editor/history"
	"github.com/ArditZubaku/tex/internal/editor/keys"
	"github.com/ArditZubaku/tex/internal/editor/prompt"
	"github.com/ArditZubaku/tex/internal/editor/register"
	"github.com/ArditZubaku/tex/internal/editor/render"
	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/ArditZubaku/tex/internal/editor/view"
	"github.com/ArditZubaku/tex/internal/search"
	"github.com/ArditZubaku/tex/internal/theme"
)

func WriteTemp(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "f.txt")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	return path
}

// AtCursor opens content as the editor's buffer with the cursor put at row and
// col, which is where a command under test is run from.
func AtCursor(t *testing.T, e *state.Editor, content string, row, col int) *buffer.Buffer {
	t.Helper()

	path := WriteTemp(t, content)
	b := buffer.Open(path)
	t.Cleanup(b.Close)
	e.Buf, e.SourceFile, e.Row, e.Col, e.Modified = b, path, row, col, false

	return b
}

func Lines(t *testing.T, b *buffer.Buffer) []string {
	t.Helper()

	out := make([]string, b.LineCount())
	for i := range out {
		out[i] = string(b.Line(i))
	}

	return out
}

func WantLines(t *testing.T, b *buffer.Buffer, want ...string) {
	t.Helper()

	got := Lines(t, b)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("buffer = %q, want %q", got, want)
	}
}

// InReadMode is a fresh editor in Read mode over content, with the buffer
// list, the window tree and everything a previous test may have left set back
// to what the editor starts from.
func InReadMode(t *testing.T, e *state.Editor, content string, row, col int) *buffer.Buffer {
	t.Helper()

	b := AtCursor(t, e, content, row, col)
	e.Mode = state.ReadMode
	e.PendingKeys, e.PendingCount, e.CmdCount = nil, 0, 1
	e.Hist = history.History{}
	e.Clip = register.Register{}
	e.SearchPat, e.SearchBack, e.HlSearch = search.Pattern{}, false, false
	e.Prompt, e.StatusMsg = prompt.Line{}, ""
	e.Note.Clear()
	e.Quitting, e.Palette = false, theme.Default()
	e.ExplorerOpen, e.Exp = false, filetree.Tree{}
	view.Reset()
	render.Reset()
	e.WinRow, e.WinCol = state.TabBarRows, 0

	return b
}

// SingleWindow is the state the editor's own layout pass leaves behind for one
// unsplit window filling the screen, which is what every test that draws or
// scrolls assumes it starts from.
func SingleWindow(e *state.Editor, rows, cols int) {
	view.Reset()
	e.ScreenRows, e.ScreenCols = rows, cols
	e.Rows, e.Cols = rows, cols
	e.WinRow, e.WinCol = state.TabBarRows, 0
}

// Press feeds input the way the editor's own loop would, dispatching on the
// mode each key arrives in.
func Press(t *testing.T, e *state.Editor, input string) {
	t.Helper()

	for _, ch := range input {
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

		keys.Dispatch(e, event)
	}
}

// PressKey is the keys that arrive as keys rather than runes, and so take the
// path every special key does.
func PressKey(t *testing.T, e *state.Editor, key termbox.Key) {
	t.Helper()

	keys.Dispatch(e, termbox.Event{Key: key})
}

// Esc is what leaving a mode, a pending chord or a pending count goes through.
func Esc(t *testing.T, e *state.Editor) {
	t.Helper()

	PressKey(t, e, termbox.KeyEsc)
}

// BigFile needs many window refills, with line lengths varying either side of
// the window size so refill boundaries land in awkward places.
// BigFile needs many window refills, with line lengths varying either side of
// the window size so refill boundaries land in awkward places.
func BigFile(t *testing.T, lines int) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "big.txt")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	w := bufio.NewWriter(f)
	for i := range lines {
		switch i % 97 {
		case 0:
			// empty line
		case 13:
			_, _ = fmt.Fprintf(w, "%d %s", i, strings.Repeat("ünïcödé ", 9))
		case 41:
			// longer than the whole window, to force the oversize path
			_, _ = fmt.Fprintf(w, "%d %s", i, strings.Repeat("x", buffer.WindowBytes+512))
		default:
			_, _ = fmt.Fprintf(w, "%d %s", i, strings.Repeat("abcdefgh ", 1+i%14))
		}
		_ = w.WriteByte('\n')
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}

	return path
}

// FullDecode is the reference the windowed Buffer is checked against.

func FullDecode(t *testing.T, path string) [][]rune {
	t.Helper()

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	var lines [][]rune
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4<<20)
	for sc.Scan() {
		lines = append(lines, []rune(sc.Text()))
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}

	return lines
}

func bufferNames() []string {
	names := make([]string, 0, len(view.Buffers()))
	for _, entry := range view.Buffers() {
		names = append(names, filepath.Base(entry.Path))
	}

	return names
}

func WantBuffers(t *testing.T, want ...string) {
	t.Helper()

	got := bufferNames()
	if len(got) != len(want) {
		t.Fatalf("buffers = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("buffers = %v, want %v", got, want)
		}
	}
}

func WantCurrent(t *testing.T, e *state.Editor, want string) {
	t.Helper()

	if got := filepath.Base(e.SourceFile); got != want {
		t.Errorf("editing %q, want %q", got, want)
	}
}

func WantCursor(t *testing.T, e *state.Editor, row, col int) {
	t.Helper()

	if e.Row != row || e.Col != col {
		t.Errorf("cursor at %d,%d, want %d,%d", e.Row, e.Col, row, col)
	}
}

func TypeIn(t *testing.T, e *state.Editor, text string) {
	t.Helper()

	for _, ch := range text {
		keys.Dispatch(e, termbox.Event{Ch: ch})
	}
}
