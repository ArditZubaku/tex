package editor

import (
	"path/filepath"
	"testing"

	"github.com/nsf/termbox-go"
)

func TestGrListsEveryMentionInTheFileAndTheOnesBesideIt(t *testing.T) {
	inDefinition(t, "func target() {}\n\ntarget()\n", 0, 5, map[string]string{
		"other.go":  "target()\n",
		"notes.txt": "target()\n",
	})

	press(t, "gr")

	if mode != PickerMode {
		t.Fatalf("mode = %v, want PickerMode", mode)
	}
	want := []string{"start.go:1: func target() {}", "start.go:3: target()", "other.go:1: target()"}
	got := matchedLabels()
	if len(got) != len(want) {
		t.Fatalf("references = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("references = %v, want %v", got, want)
		}
	}
}

func TestGrTakesTheWholeWordOnly(t *testing.T) {
	inDefinition(t, "target()\ntargeting()\nretarget()\n", 0, 0, nil)

	press(t, "gr")

	if got := len(pick.Matched()); got != 1 {
		t.Errorf("%d references, want 1: %v", got, matchedLabels())
	}
}

func TestEnterOnAReferenceGoesToIt(t *testing.T) {
	inDefinition(t, "func target() {}\n\nvar x = target\n", 0, 5, nil)

	press(t, "gr")
	pressKey(t, termbox.KeyCtrlN)
	pressKey(t, termbox.KeyEnter)

	wantAt(t, 2, 8)
	if pick.Open() {
		t.Error("the popup stayed open")
	}
}

func TestEnterOnAReferenceInAnotherFileOpensIt(t *testing.T) {
	inDefinition(t, "target()\n", 0, 0, map[string]string{"other.go": "package main\n\nvar target = 1\n"})

	press(t, "gr")
	pressKey(t, termbox.KeyCtrlN)
	pressKey(t, termbox.KeyEnter)

	if got := filepath.Base(sourceFile); got != "other.go" {
		t.Fatalf("editing %q, want other.go", got)
	}
	wantAt(t, 2, 4)
}

func TestCtrlOComesBackFromAReference(t *testing.T) {
	inDefinition(t, "func target() {}\n\nvar x = target\n", 0, 5, nil)

	press(t, "gr")
	pressKey(t, termbox.KeyCtrlN)
	pressKey(t, termbox.KeyEnter)
	pressKey(t, termbox.KeyCtrlO)

	wantAt(t, 0, 5)
}

func TestTypingNarrowsTheReferences(t *testing.T) {
	inDefinition(t, "func target() {}\n\nvar x = target\n", 0, 5, nil)

	press(t, "gr")
	press(t, "func")

	if got := matchedLabels(); len(got) != 1 || got[0] != "start.go:1: func target() {}" {
		t.Errorf("references = %v", got)
	}
}

func TestGrSaysWhenTheCursorIsOnNoIdentifier(t *testing.T) {
	inDefinition(t, "   \n", 0, 0, nil)

	press(t, "gr")

	if statusMsg != "E349: No identifier under the cursor" {
		t.Errorf("statusMsg = %q", statusMsg)
	}
}
