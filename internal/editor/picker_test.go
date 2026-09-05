package editor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ArditZubaku/tex/internal/editor/edtest"
	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/nsf/termbox-go"
)

func inPicker(t *testing.T, e *state.Editor, names ...string) {
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

	edtest.InReadMode(t, e, "first\n", 0, 0)
	e.SourceFile = filepath.Join(dir, "start.txt")
	edtest.SingleWindow(e, 20, 80)
}

func matchedLabels(e *state.Editor) []string {
	matched := e.Pick.Matched()
	paths := make([]string, 0, len(matched))
	for _, entry := range matched {
		paths = append(paths, entry.Label)
	}

	return paths
}

func wantMatches(t *testing.T, e *state.Editor, want ...string) {
	t.Helper()

	got := matchedLabels(e)
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
	e := state.New()

	inPicker(t, e, "one.go", "two.go", filepath.Join("sub", "three.go"))

	edtest.Press(t, e, "  ")

	if e.Mode != state.PickerMode {
		t.Fatalf("mode = %v, want PickerMode", e.Mode)
	}
	wantMatches(t, e, "one.go", filepath.Join("sub", "three.go"), "two.go")
}

func TestThePickerLeavesHiddenDirectoriesOut(t *testing.T) {
	e := state.New()

	inPicker(t, e, "one.go", filepath.Join(".git", "config"), ".env")

	edtest.Press(t, e, "  ")

	wantMatches(t, e, "one.go")
}

func TestTypingNarrowsThePickerToWhatMatches(t *testing.T) {
	e := state.New()

	inPicker(t, e, "buffers.go", "windows.go", "README.md")

	edtest.Press(t, e, "  ")
	edtest.Press(t, e, "win")

	wantMatches(t, e, "windows.go")
}

func TestThePickerMatchesLettersItDoesNotHaveTogether(t *testing.T) {
	e := state.New()

	inPicker(t, e, "buffers.go", "windows.go")

	edtest.Press(t, e, "  ")
	edtest.Press(t, e, "bfg")

	wantMatches(t, e, "buffers.go")
}

func TestThePickerPutsTheNameBeforeTheDirectoryItIsIn(t *testing.T) {
	e := state.New()

	inPicker(t, e, filepath.Join("theme", "one.go"), "theme.go")

	edtest.Press(t, e, "  ")
	edtest.Press(t, e, "theme")

	wantMatches(t, e, "theme.go", filepath.Join("theme", "one.go"))
}

func TestEnterOpensThePickedFileAsABuffer(t *testing.T) {
	e := state.New()

	inPicker(t, e, "one.go", "two.go")

	edtest.Press(t, e, "  ")
	edtest.Press(t, e, "two")
	edtest.PressKey(t, e, termbox.KeyEnter)

	if e.Pick.Open() {
		t.Error("the picker stayed open")
	}
	edtest.WantCurrent(t, e, "two.go")
	edtest.WantBuffers(t, "start.txt", "two.go")
}

func TestCtrlNAndCtrlPWalkTheListing(t *testing.T) {
	e := state.New()

	inPicker(t, e, "one.go", "two.go")

	edtest.Press(t, e, "  ")
	edtest.PressKey(t, e, termbox.KeyCtrlN)
	edtest.PressKey(t, e, termbox.KeyCtrlN) // which is as far as two files go
	edtest.PressKey(t, e, termbox.KeyEnter)

	edtest.WantCurrent(t, e, "two.go")
}

func TestEscClosesThePickerAndLeavesTheBufferAlone(t *testing.T) {
	e := state.New()

	inPicker(t, e, "one.go")

	edtest.Press(t, e, "  ")
	edtest.PressKey(t, e, termbox.KeyEsc)

	if e.Pick.Open() || e.Mode != state.ReadMode {
		t.Errorf("picker open = %v, mode = %v", e.Pick.Open(), e.Mode)
	}
	edtest.WantCurrent(t, e, "start.txt")
}

func TestBackspacingOffAnEmptyQueryClosesThePicker(t *testing.T) {
	e := state.New()

	inPicker(t, e, "one.go")

	edtest.Press(t, e, "  ")
	edtest.Press(t, e, "o")
	edtest.PressKey(t, e, termbox.KeyBackspace2)
	edtest.PressKey(t, e, termbox.KeyBackspace2)

	if e.Pick.Open() {
		t.Error("the picker stayed open")
	}
}
