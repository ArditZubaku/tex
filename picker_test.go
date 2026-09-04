package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nsf/termbox-go"
)

func inPicker(t *testing.T, names ...string) {
	t.Helper()

	dir := t.TempDir()
	for _, name := range names {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("in "+name+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	inReadMode(t, "first\n", 0, 0)
	sourceFile = filepath.Join(dir, "start.txt")
	singleWindow(20, 80)
}

func matchedLabels() []string {
	paths := make([]string, 0, len(pickerMatches))
	for _, match := range pickerMatches {
		paths = append(paths, pickerEntries[match.at].label)
	}

	return paths
}

func wantMatches(t *testing.T, want ...string) {
	t.Helper()

	got := matchedLabels()
	if len(got) != len(want) {
		t.Fatalf("matches = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("matches = %v, want %v", got, want)
		}
	}
}

func TestLeaderLeaderOpensThePickerOnEveryFileUnderTheRoot(t *testing.T) {
	inPicker(t, "one.go", "two.go", filepath.Join("sub", "three.go"))

	press(t, "  ")

	if mode != PickerMode {
		t.Fatalf("mode = %v, want PickerMode", mode)
	}
	wantMatches(t, "one.go", filepath.Join("sub", "three.go"), "two.go")
}

func TestThePickerLeavesHiddenDirectoriesOut(t *testing.T) {
	inPicker(t, "one.go", filepath.Join(".git", "config"), ".env")

	press(t, "  ")

	wantMatches(t, "one.go")
}

func TestTypingNarrowsThePickerToWhatMatches(t *testing.T) {
	inPicker(t, "buffers.go", "windows.go", "README.md")

	press(t, "  ")
	press(t, "win")

	wantMatches(t, "windows.go")
}

func TestThePickerMatchesLettersItDoesNotHaveTogether(t *testing.T) {
	inPicker(t, "buffers.go", "windows.go")

	press(t, "  ")
	press(t, "bfg")

	wantMatches(t, "buffers.go")
}

func TestThePickerPutsTheNameBeforeTheDirectoryItIsIn(t *testing.T) {
	inPicker(t, filepath.Join("theme", "one.go"), "theme.go")

	press(t, "  ")
	press(t, "theme")

	wantMatches(t, "theme.go", filepath.Join("theme", "one.go"))
}

func TestEnterOpensThePickedFileAsABuffer(t *testing.T) {
	inPicker(t, "one.go", "two.go")

	press(t, "  ")
	press(t, "two")
	pressKey(t, termbox.KeyEnter)

	if pickerOpen {
		t.Error("the picker stayed open")
	}
	wantCurrent(t, "two.go")
	wantBuffers(t, "start.txt", "two.go")
}

func TestCtrlNAndCtrlPWalkTheListing(t *testing.T) {
	inPicker(t, "one.go", "two.go")

	press(t, "  ")
	pressKey(t, termbox.KeyCtrlN)
	pressKey(t, termbox.KeyCtrlN) // which is as far as two files go
	pressKey(t, termbox.KeyEnter)

	wantCurrent(t, "two.go")
}

func TestEscClosesThePickerAndLeavesTheBufferAlone(t *testing.T) {
	inPicker(t, "one.go")

	press(t, "  ")
	pressKey(t, termbox.KeyEsc)

	if pickerOpen || mode != ReadMode {
		t.Errorf("picker open = %v, mode = %v", pickerOpen, mode)
	}
	wantCurrent(t, "start.txt")
}

func TestBackspacingOffAnEmptyQueryClosesThePicker(t *testing.T) {
	inPicker(t, "one.go")

	press(t, "  ")
	press(t, "o")
	pressKey(t, termbox.KeyBackspace2)
	pressKey(t, termbox.KeyBackspace2)

	if pickerOpen {
		t.Error("the picker stayed open")
	}
}
