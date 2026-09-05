package editor

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ArditZubaku/tex/internal/buffer"
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
	buf = buffer.Open(bigFile(t, 20000))
	defer buf.Close()

	singleWindow(30, 80)
	currentRow, currentCol = 0, 0

	for range 100 {
		down()
	}
	if currentRow != 100 {
		t.Fatalf("after 100 down, row = %d", currentRow)
	}

	goToBottom()
	if currentRow != buf.LineCount()-1 {
		t.Fatalf("G landed on row %d, want %d", currentRow, buf.LineCount()-1)
	}
	down()
	if currentRow != buf.LineCount()-1 {
		t.Fatalf("down past the last line moved to %d", currentRow)
	}

	for range 20 {
		up()
	}
	for range 200 {
		nextWord()
	}
	for range 200 {
		prevWord()
	}
	for range 50 {
		endOfWord()
	}

	goToTop()
	if currentRow != 0 || currentCol != 0 {
		t.Fatalf("gg landed on %d,%d", currentRow, currentCol)
	}
	up()
	left()
	if currentRow != 0 || currentCol != 0 {
		t.Fatalf("moving off the top-left moved to %d,%d", currentRow, currentCol)
	}

	for range 40 {
		pageDown()
	}
	for range 100 {
		pageUp()
	}
	if currentRow != 0 {
		t.Fatalf("pageUp past the top landed on %d", currentRow)
	}
}

func TestInsertDoesNotCorruptNeighbours(t *testing.T) {
	buf = buffer.Open(bigFile(t, 20000))
	defer buf.Close()

	const row = 5001
	before := []string{string(buf.Line(row - 1)), string(buf.Line(row + 1))}

	currentRow, currentCol = row, 0
	for _, ch := range "abc" {
		insertRune(termbox.Event{Ch: ch})
	}

	if got := string(buf.Line(row)); !strings.HasPrefix(got, "abc") {
		t.Fatalf("edited line = %q, want it to start with abc", got[:min(10, len(got))])
	}
	if got := string(buf.Line(row - 1)); got != before[0] {
		t.Errorf("line above changed")
	}
	if got := string(buf.Line(row + 1)); got != before[1] {
		t.Errorf("line below changed")
	}

	// The edit must survive a refill from elsewhere in the file.
	_ = buf.Line(19000)
	if got := string(buf.Line(row)); !strings.HasPrefix(got, "abc") {
		t.Error("edit lost after the window moved")
	}
}
