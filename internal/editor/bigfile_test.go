package editor

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ArditZubaku/tex/internal/buffer"
	"github.com/ArditZubaku/tex/internal/editor/edit"
	"github.com/ArditZubaku/tex/internal/editor/edtest"
	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/nsf/termbox-go"
)

// bigFile needs many window refills, with line lengths varying either side
// of the window size so refill boundaries land in awkward places.
func bigFile(t *testing.T, lines int) string {
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

// fullDecode is the reference the windowed Buffer is checked against.
func fullDecode(t *testing.T, path string) [][]rune {
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

// An off-by-one in the window bounds surfaces here as a panic or stuck cursor.
func TestNavigationOverBigFile(t *testing.T) {
	e := state.New()

	e.Buf = buffer.Open(bigFile(t, 20000))
	defer e.Buf.Close()

	edtest.SingleWindow(e, 30, 80)
	e.Row, e.Col = 0, 0

	for range 100 {
		e.Down()
	}
	if e.Row != 100 {
		t.Fatalf("after 100 down, row = %d", e.Row)
	}

	e.GoToBottom()
	if e.Row != e.Buf.LineCount()-1 {
		t.Fatalf("G landed on row %d, want %d", e.Row, e.Buf.LineCount()-1)
	}
	e.Down()
	if e.Row != e.Buf.LineCount()-1 {
		t.Fatalf("down past the last line moved to %d", e.Row)
	}

	for range 20 {
		e.Up()
	}
	for range 200 {
		e.NextWord()
	}
	for range 200 {
		e.PrevWord()
	}
	for range 50 {
		e.EndOfWord()
	}

	e.GoToTop()
	if e.Row != 0 || e.Col != 0 {
		t.Fatalf("gg landed on %d,%d", e.Row, e.Col)
	}
	e.Up()
	e.Left()
	if e.Row != 0 || e.Col != 0 {
		t.Fatalf("moving off the top-left moved to %d,%d", e.Row, e.Col)
	}

	for range 40 {
		e.PageDown()
	}
	for range 100 {
		e.PageUp()
	}
	if e.Row != 0 {
		t.Fatalf("pageUp past the top landed on %d", e.Row)
	}
}

func TestInsertDoesNotCorruptNeighbours(t *testing.T) {
	e := state.New()

	e.Buf = buffer.Open(bigFile(t, 20000))
	defer e.Buf.Close()

	const row = 5001
	before := []string{string(e.Buf.Line(row - 1)), string(e.Buf.Line(row + 1))}

	e.Row, e.Col = row, 0
	for _, ch := range "abc" {
		edit.InsertRune(e, termbox.Event{Ch: ch})
	}

	if got := string(e.Buf.Line(row)); !strings.HasPrefix(got, "abc") {
		t.Fatalf("edited line = %q, want it to start with abc", got[:min(10, len(got))])
	}
	if got := string(e.Buf.Line(row - 1)); got != before[0] {
		t.Errorf("line above changed")
	}
	if got := string(e.Buf.Line(row + 1)); got != before[1] {
		t.Errorf("line below changed")
	}

	// The edit must survive a refill from elsewhere in the file.
	_ = e.Buf.Line(19000)
	if got := string(e.Buf.Line(row)); !strings.HasPrefix(got, "abc") {
		t.Error("edit lost after the window moved")
	}
}
