package buffer

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeTemp(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "f.txt")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	return path
}

func TestBufferLineSplitting(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    []string
	}{
		{"trailing newline", "a\nbb\nccc\n", []string{"a", "bb", "ccc"}},
		{"no trailing newline", "a\nbb\nccc", []string{"a", "bb", "ccc"}},
		{"blank lines", "\n\na\n\n", []string{"", "", "a", ""}},
		{"crlf", "a\r\nbb\r\n", []string{"a", "bb"}},
		{"single line", "only", []string{"only"}},
		{"empty file", "", []string{""}},
		{"utf8", "héllo\nмир\n", []string{"héllo", "мир"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := Open(writeTemp(t, tc.content))
			defer b.Close()

			if got := b.LineCount(); got != len(tc.want) {
				t.Fatalf("LineCount = %d, want %d", got, len(tc.want))
			}

			for i, want := range tc.want {
				if got := string(b.Line(i)); got != want {
					t.Errorf("Line(%d) = %q, want %q", i, got, want)
				}
				if got := b.RuneLen(i); got != len([]rune(want)) {
					t.Errorf("RuneLen(%d) = %d, want %d", i, got, len([]rune(want)))
				}
			}
		})
	}
}

func TestBufferRandomAccessMatchesSequential(t *testing.T) {
	b := Open(writeTemp(t, "zero\none\ntwo\nthree\n"))
	defer b.Close()

	for _, i := range []int{3, 0, 2, 3, 1, 0} {
		want := []string{"zero", "one", "two", "three"}[i]
		if got := string(b.Line(i)); got != want {
			t.Errorf("Line(%d) = %q, want %q", i, got, want)
		}
	}
}

func TestBufferOverlayShadowsFile(t *testing.T) {
	b := Open(writeTemp(t, "a\nbb\n"))
	defer b.Close()

	edited := []rune("EDITED")
	b.SetLine(1, edited)

	if got := string(b.Line(1)); got != "EDITED" {
		t.Errorf("Line(1) = %q, want %q", got, "EDITED")
	}
	if got := b.RuneLen(1); got != 6 {
		t.Errorf("RuneLen(1) = %d, want 6", got)
	}
	if got := string(b.Line(0)); got != "a" {
		t.Errorf("Line(0) = %q, want %q", got, "a")
	}
	if !reflect.DeepEqual(b.Line(1), edited) {
		t.Error("overlay line was copied instead of shared")
	}
}

func TestBufferRuneBounds(t *testing.T) {
	b := Open(writeTemp(t, "ab\n"))
	defer b.Close()

	if ch, ok := b.Rune(0, 1); !ok || ch != 'b' {
		t.Errorf("Rune(0,1) = %q,%v", ch, ok)
	}
	for _, col := range []int{-1, 2, 99} {
		if _, ok := b.Rune(0, col); ok {
			t.Errorf("Rune(0,%d) unexpectedly in range", col)
		}
	}
	if _, ok := b.Rune(5, 0); ok {
		t.Error("Rune on out-of-range row unexpectedly in range")
	}
}

// The last line takes its extent from the file size rather than the next line's offset,
// so cover it both with and without a terminating newline.
func TestLongLastLine(t *testing.T) {
	long := strings.Repeat("z", WindowBytes+777)

	for _, tc := range []struct{ name, content string }{
		{"terminated", "a\n" + long + "\n"},
		{"unterminated", "a\n" + long},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := Open(writeTemp(t, tc.content))
			defer b.Close()

			if b.LineCount() != 2 {
				t.Fatalf("LineCount = %d, want 2", b.LineCount())
			}
			if got := string(b.Line(1)); got != long {
				t.Fatalf("last line len = %d, want %d", len(got), len(long))
			}
			if got := b.RuneLen(1); got != len(long) {
				t.Fatalf("RuneLen = %d, want %d", got, len(long))
			}
			// Re-read after the window moved off it and back.
			if got := string(b.Line(0)); got != "a" {
				t.Fatalf("Line(0) = %q", got)
			}
			if got := string(b.Line(1)); got != long {
				t.Fatalf("last line changed on re-read (len %d)", len(got))
			}
		})
	}
}

func TestEmptyBuffer(t *testing.T) {
	b := NewEmpty()

	if b.LineCount() != 1 || len(b.Line(0)) != 0 {
		t.Fatalf("empty buffer = %d lines, first %q", b.LineCount(), string(b.Line(0)))
	}

	b.SetLine(0, []rune("x"))
	if got := string(b.Line(0)); got != "x" {
		t.Errorf("Line(0) = %q, want %q", got, "x")
	}
}
