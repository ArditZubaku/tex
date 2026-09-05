package editor

import (
	"path/filepath"
	"testing"

	"github.com/ArditZubaku/tex/internal/editor/edtest"
	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/nsf/termbox-go"
)

func TestGrListsEveryMentionInTheFileAndTheOnesBesideIt(t *testing.T) {
	e := state.New()

	inDefinition(t, e, "func target() {}\n\ntarget()\n", 0, 5, map[string]string{
		"other.go":  "target()\n",
		"notes.txt": "target()\n",
	})

	edtest.Press(t, e, "gr")

	if e.Mode != state.PickerMode {
		t.Fatalf("mode = %v, want PickerMode", e.Mode)
	}
	want := []string{"start.go:1: func target() {}", "start.go:3: target()", "other.go:1: target()"}
	got := matchedLabels(e)
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
	e := state.New()

	inDefinition(t, e, "target()\ntargeting()\nretarget()\n", 0, 0, nil)

	edtest.Press(t, e, "gr")

	if got := len(e.Pick.Matched()); got != 1 {
		t.Errorf("%d references, want 1: %v", got, matchedLabels(e))
	}
}

func TestEnterOnAReferenceGoesToIt(t *testing.T) {
	e := state.New()

	inDefinition(t, e, "func target() {}\n\nvar x = target\n", 0, 5, nil)

	edtest.Press(t, e, "gr")
	edtest.PressKey(t, e, termbox.KeyCtrlN)
	edtest.PressKey(t, e, termbox.KeyEnter)

	wantAt(t, e, 2, 8)
	if e.Pick.Open() {
		t.Error("the popup stayed open")
	}
}

func TestEnterOnAReferenceInAnotherFileOpensIt(t *testing.T) {
	e := state.New()

	inDefinition(t, e, "target()\n", 0, 0, map[string]string{"other.go": "package main\n\nvar target = 1\n"})

	edtest.Press(t, e, "gr")
	edtest.PressKey(t, e, termbox.KeyCtrlN)
	edtest.PressKey(t, e, termbox.KeyEnter)

	if got := filepath.Base(e.SourceFile); got != "other.go" {
		t.Fatalf("editing %q, want other.go", got)
	}
	wantAt(t, e, 2, 4)
}

func TestCtrlOComesBackFromAReference(t *testing.T) {
	e := state.New()

	inDefinition(t, e, "func target() {}\n\nvar x = target\n", 0, 5, nil)

	edtest.Press(t, e, "gr")
	edtest.PressKey(t, e, termbox.KeyCtrlN)
	edtest.PressKey(t, e, termbox.KeyEnter)
	edtest.PressKey(t, e, termbox.KeyCtrlO)

	wantAt(t, e, 0, 5)
}

func TestTypingNarrowsTheReferences(t *testing.T) {
	e := state.New()

	inDefinition(t, e, "func target() {}\n\nvar x = target\n", 0, 5, nil)

	edtest.Press(t, e, "gr")
	edtest.Press(t, e, "func")

	if got := matchedLabels(e); len(got) != 1 || got[0] != "start.go:1: func target() {}" {
		t.Errorf("references = %v", got)
	}
}

func TestGrSaysWhenTheCursorIsOnNoIdentifier(t *testing.T) {
	e := state.New()

	inDefinition(t, e, "   \n", 0, 0, nil)

	edtest.Press(t, e, "gr")

	if e.StatusMsg != "E349: No identifier under the cursor" {
		t.Errorf("statusMsg = %q", e.StatusMsg)
	}
}
