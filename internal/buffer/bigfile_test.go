package buffer

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
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
			_, _ = fmt.Fprintf(w, "%d %s", i, strings.Repeat("x", WindowBytes+512))
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

func TestBufferMatchesFullDecode(t *testing.T) {
	path := bigFile(t, 20000)

	want := fullDecode(t, path)
	b := Open(path)
	defer b.Close()

	if b.LineCount() != len(want) {
		t.Fatalf("LineCount = %d, want %d", b.LineCount(), len(want))
	}

	check := func(i int) {
		t.Helper()
		if got := string(b.Line(i)); got != string(want[i]) {
			t.Fatalf("Line(%d) mismatch (len %d vs %d)", i, len(got), len(want[i]))
		}
		if got := b.RuneLen(i); got != len(want[i]) {
			t.Fatalf("RuneLen(%d) = %d, want %d", i, got, len(want[i]))
		}
	}

	// Crosses every refill boundary in the file.
	for i := range want {
		check(i)
	}

	// Backwards, then jumps far enough apart to miss the window every time.
	for i := range slices.Backward(want) {
		check(i)
	}
	for _, i := range []int{0, len(want) - 1, 1, len(want) / 2, 41, 13, len(want) - 2, 0} {
		check(i)
	}
}
