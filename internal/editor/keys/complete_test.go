package keys_test

import (
	"testing"

	"github.com/nsf/termbox-go"

	"github.com/ArditZubaku/tex/internal/editor/edtest"
	"github.com/ArditZubaku/tex/internal/editor/state"
)

func TestTabStillTypesASpaceWithNoCompletionMenuUp(t *testing.T) {
	e := state.New()

	b := edtest.InReadMode(t, e, "\n", 0, 0)
	edtest.Press(t, e, "i")
	edtest.PressKey(t, e, termbox.KeyTab)
	edtest.Press(t, e, "x")

	edtest.WantLines(t, b, " x")
}

func TestCtrlNWithNoLanguageServerSaysSoRatherThanNothing(t *testing.T) {
	e := state.New()

	edtest.InReadMode(t, e, "package main\n", 0, 0)
	edtest.Press(t, e, "i")
	edtest.PressKey(t, e, termbox.KeyCtrlN)

	if e.StatusMsg == "" {
		t.Error("Ctrl-N with nothing behind it said nothing at all")
	}
	if e.Mode != state.EditMode {
		t.Error("Ctrl-N left Edit mode")
	}
}

func TestTypingWithNoLanguageServerNeverOpensAMenu(t *testing.T) {
	e := state.New()

	b := edtest.InReadMode(t, e, "\n", 0, 0)
	edtest.Press(t, e, "i")
	edtest.TypeIn(t, e, "Println")

	if e.Comp.Open() {
		t.Error("a menu went up with no server to have offered anything")
	}
	edtest.WantLines(t, b, "Println")
}

// Entering Edit mode next to a name is not somebody asking what the name could
// be, so the key that entered it never opens a menu.
func TestEnteringEditModeBesideAWordOpensNoMenu(t *testing.T) {
	e := state.New()

	edtest.InReadMode(t, e, "Println\n", 0, 6)
	edtest.Press(t, e, "a")

	if e.Comp.Open() {
		t.Error("a menu went up on the key that entered Edit mode")
	}
}

func TestCtrlSpaceAsksForCompletionsRatherThanTypingAnything(t *testing.T) {
	e := state.New()

	b := edtest.InReadMode(t, e, "package main\n", 0, 0)
	edtest.Press(t, e, "i")
	edtest.PressKey(t, e, termbox.KeyCtrlSpace)

	if e.StatusMsg == "" {
		t.Error("Ctrl-Space with nothing behind it said nothing at all")
	}
	if e.Mode != state.EditMode {
		t.Error("Ctrl-Space left Edit mode")
	}
	edtest.WantLines(t, b, "package main")
}

func TestCtrlSpaceOutsideEditModeTypesNothing(t *testing.T) {
	e := state.New()

	b := edtest.InReadMode(t, e, "package main\n", 0, 0)
	edtest.PressKey(t, e, termbox.KeyCtrlSpace)

	if e.Mode != state.ReadMode {
		t.Error("Ctrl-Space left Read mode")
	}
	edtest.WantLines(t, b, "package main")
}
