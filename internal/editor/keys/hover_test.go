package keys_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/nsf/termbox-go"

	"github.com/ArditZubaku/tex/internal/editor/edtest"
	"github.com/ArditZubaku/tex/internal/editor/keys"
	"github.com/ArditZubaku/tex/internal/editor/state"
)

// showHover puts more than one page of hover text up without going through the
// language server, numbered so a scroll is easy to tell from the box it left.
func showHover(t *testing.T, e *state.Editor) {
	t.Helper()

	var b strings.Builder
	b.WriteString("```\n")
	for i := range 40 {
		fmt.Fprintf(&b, "hover line %d\n", i)
	}
	b.WriteString("```\n")

	e.Hov.Show(b.String(), e.ScreenArea())
}

func mouse(e *state.Editor, key termbox.Key, row, col int) {
	keys.Handle(e, termbox.Event{Type: termbox.EventMouse, Key: key, MouseY: row, MouseX: col}, true)
}

func TestWheelOverTheHoverBoxScrollsItInsteadOfClosingIt(t *testing.T) {
	e := state.New()
	edtest.AtCursor(t, e, "package main\n", 0, 0)
	edtest.SingleWindow(e, 24, 80)
	showHover(t, e)
	before := e.Hov.Text()

	mouse(e, termbox.MouseWheelDown, e.CursorScreenRow()+1, e.CursorScreenCol())

	if !e.Hov.Showing() {
		t.Fatal("the wheel over the box closed it")
	}
	if after := e.Hov.Text(); after == before {
		t.Errorf("the box reads the same after the wheel: %q", after)
	}
}

func TestWheelAwayFromTheHoverBoxStillClosesIt(t *testing.T) {
	e := state.New()
	edtest.AtCursor(t, e, "package main\n", 0, 0)
	edtest.SingleWindow(e, 24, 80)
	showHover(t, e)

	mouse(e, termbox.MouseWheelDown, 0, 0)

	if e.Hov.Showing() {
		t.Error("the wheel away from the box left it open")
	}
}

func TestAClickOnTheHoverBoxStillClosesIt(t *testing.T) {
	e := state.New()
	edtest.AtCursor(t, e, "package main\n", 0, 0)
	edtest.SingleWindow(e, 24, 80)
	showHover(t, e)

	mouse(e, termbox.MouseLeft, e.CursorScreenRow()+1, e.CursorScreenCol())

	if e.Hov.Showing() {
		t.Error("a click on the box left it open")
	}
}
