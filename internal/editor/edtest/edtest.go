// Package edtest is the harness the editor's own tests share: a buffer written
// to a temporary file, an editor pointed at it, and the assertion that its
// lines read back as expected.
package edtest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ArditZubaku/tex/internal/buffer"
	"github.com/ArditZubaku/tex/internal/editor/state"
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
