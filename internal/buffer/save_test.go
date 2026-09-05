package buffer

import (
	"os"
	"path/filepath"
	"testing"
)

func readBack(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	return string(content)
}

func TestSaveWritesEditsAndDeletes(t *testing.T) {
	path := writeTemp(t, "a\nbb\nccc\ndddd\n")
	b := Open(path)
	defer b.Close()

	b.SetLine(1, []rune("EDITED"))
	b.InsertRune(3, 0, 'ü')
	b.DeleteLine(2)

	if err := b.Save(path); err != nil {
		t.Fatal(err)
	}
	if got := readBack(t, path); got != "a\nEDITED\nüdddd\n" {
		t.Fatalf("file = %q", got)
	}

	// the reopened buffer must read from the file it just wrote
	wantLines(t, b, "a", "EDITED", "üdddd")
}

func TestSaveTerminators(t *testing.T) {
	cases := []struct {
		name, content, want string
	}{
		{"trailing newline kept", "a\nbb\n", "a\nXbb\n"},
		{"absent trailing newline stays absent", "a\nbb", "a\nXbb"},
		{"crlf preserved on untouched lines", "a\r\nbb\r\n", "a\r\nXbb\n"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := writeTemp(t, tc.content)
			b := Open(path)
			defer b.Close()

			b.InsertRune(1, 0, 'X')
			if err := b.Save(path); err != nil {
				t.Fatal(err)
			}
			if got := readBack(t, path); got != tc.want {
				t.Fatalf("file = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSaveKeepsFileMode(t *testing.T) {
	path := writeTemp(t, "a\n")
	if err := os.Chmod(path, 0o640); err != nil {
		t.Fatal(err)
	}

	b := Open(path)
	defer b.Close()

	if err := b.Save(path); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o640 {
		t.Errorf("mode = %v, want 0640", got)
	}
}

func TestSaveCreatesANewFile(t *testing.T) {
	b := NewEmpty()
	path := filepath.Join(t.TempDir(), "new.txt")

	b.InsertRune(0, 0, 'h')
	b.InsertRune(0, 1, 'i')
	if err := b.Save(path); err != nil {
		t.Fatal(err)
	}
	defer b.Close()

	if got := readBack(t, path); got != "hi" {
		t.Fatalf("file = %q, want %q", got, "hi")
	}
	wantLines(t, b, "hi")
}

// Saving must not leave the directory littered if it fails, or on success.
func TestSaveLeavesNoTempFile(t *testing.T) {
	path := writeTemp(t, "a\n")
	b := Open(path)
	defer b.Close()

	if err := b.Save(path); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("directory holds %d files, want just the saved one", len(entries))
	}
}

func TestSaveBigFileMatchesFullDecode(t *testing.T) {
	path := bigFile(t, 5000)
	want := fullDecode(t, path)

	b := Open(path)
	defer b.Close()

	b.SetLine(41, []rune("was the oversized line"))
	b.DeleteLine(1000)
	want[41] = []rune("was the oversized line")
	want = append(want[:1000], want[1001:]...)

	if err := b.Save(path); err != nil {
		t.Fatal(err)
	}

	got := fullDecode(t, path)
	if len(got) != len(want) {
		t.Fatalf("saved %d lines, want %d", len(got), len(want))
	}
	for i := range want {
		if string(got[i]) != string(want[i]) {
			t.Fatalf("saved line %d = %.40q, want %.40q", i, string(got[i]), string(want[i]))
		}
	}
}
