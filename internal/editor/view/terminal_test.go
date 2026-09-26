package view_test

import (
	"testing"
	"time"

	"github.com/nsf/termbox-go"

	"github.com/ArditZubaku/tex/internal/editor/edtest"
	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/ArditZubaku/tex/internal/editor/terminal"
	"github.com/ArditZubaku/tex/internal/editor/view"
)

func waitUntilExited(t *testing.T, sess *terminal.Session) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if sess.Exited() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out waiting for the shell to exit")
}

// openTerminal is '<leader>ft' from a fresh, unsplit window, with the real
// shell it starts always killed at the end of the test that opened it.
func openTerminal(t *testing.T, e *state.Editor) *view.Window {
	t.Helper()

	edtest.Press(t, e, " ft")
	w := view.Focused()
	t.Cleanup(w.Entry.Terminal.Kill)

	return w
}

// detachFromTerminal is Ctrl-\ Ctrl-N, run as a helper since most of what a
// terminal window refuses to do is only reachable once focus has left it.
func detachFromTerminal(t *testing.T, e *state.Editor) {
	t.Helper()

	edtest.PressKey(t, e, termbox.KeyCtrlBackslash)
	edtest.PressKey(t, e, termbox.KeyCtrlN)
}

func TestLeaderFtOpensATerminalBesideTheWindowAndFocusesIt(t *testing.T) {
	e := state.New()

	inWindows(t, e)

	term := openTerminal(t, e)

	wantWindowCount(t, e, 2)
	if view.Focused() != term {
		t.Error("focus did not move to the terminal")
	}
	if e.Mode != state.TerminalMode {
		t.Errorf("mode = %v, want TerminalMode", e.Mode)
	}
	if len(view.Separators()) != 1 || !view.Separators()[0].Vertical {
		t.Errorf("separators = %v, want one vertical", view.Separators())
	}
}

func TestLeaderFtRefusesWhenThereIsNoRoomForIt(t *testing.T) {
	e := state.New()

	inWindows(t, e)
	edtest.SingleWindow(e, 20, 30)

	edtest.Press(t, e, " ft")

	wantWindowCount(t, e, 1)
	if e.StatusMsg != "E36: Not enough room" {
		t.Errorf("statusMsg = %q", e.StatusMsg)
	}
}

func TestLeaderFtFocusesTheExistingTerminalRatherThanOpeningASecondOne(t *testing.T) {
	e := state.New()

	inWindows(t, e)

	term := openTerminal(t, e)
	detachFromTerminal(t, e)
	if view.Focused() == term {
		t.Fatal("detaching did not move focus off the terminal")
	}

	edtest.Press(t, e, " ft")

	wantWindowCount(t, e, 2)
	if view.Focused() != term {
		t.Error("<leader>ft opened a second terminal instead of refocusing the first")
	}
}

func TestCtrlHDoesNotLeaveTheTerminalWindowWhileFocused(t *testing.T) {
	e := state.New()

	inWindows(t, e)

	term := openTerminal(t, e)

	edtest.PressKey(t, e, termbox.KeyCtrlH)

	if view.Focused() != term {
		t.Error("Ctrl-H moved focus out of the terminal instead of forwarding it to the shell")
	}
}

func TestCtrlBackslashCtrlNDetachesFocusWithoutClosingTheShell(t *testing.T) {
	e := state.New()

	inWindows(t, e)

	term := openTerminal(t, e)

	detachFromTerminal(t, e)

	if view.Focused() == term {
		t.Fatal("focus stayed on the terminal")
	}
	if e.Mode == state.TerminalMode {
		t.Error("mode stayed TerminalMode after detaching")
	}
	wantWindowCount(t, e, 2)
	if term.Entry.Terminal.Exited() {
		t.Error("detaching killed the shell")
	}
}

func TestALoneCtrlBackslashDoesNotDetach(t *testing.T) {
	e := state.New()

	inWindows(t, e)

	term := openTerminal(t, e)

	edtest.PressKey(t, e, termbox.KeyCtrlBackslash)
	edtest.Press(t, e, "x") // anything but Ctrl-N cancels the pending detach

	if view.Focused() != term {
		t.Fatal("Ctrl-\\ followed by an ordinary key detached anyway")
	}

	edtest.PressKey(t, e, termbox.KeyCtrlN) // Ctrl-N alone, nothing pending before it

	if view.Focused() != term {
		t.Error("a lone Ctrl-N detached without a preceding Ctrl-\\")
	}
}

func TestOnlyClosesTheTerminalTooWhenRunFromElsewhere(t *testing.T) {
	e := state.New()

	inWindows(t, e)

	term := openTerminal(t, e)
	detachFromTerminal(t, e)

	edtest.Press(t, e, ":only\n")

	wantWindowCount(t, e, 1)
	waitUntilExited(t, term.Entry.Terminal)
}

func TestPollTerminalClosesTheWindowWhenTheShellExitsOnItsOwn(t *testing.T) {
	e := state.New()

	inWindows(t, e)

	term := openTerminal(t, e)
	other := view.List(e)[0]

	term.Entry.Terminal.Kill()
	waitUntilExited(t, term.Entry.Terminal)
	view.PollTerminal(e)

	wantWindowCount(t, e, 1)
	if view.Focused() != other {
		t.Error("focus did not land on the window left")
	}
	if e.Mode == state.TerminalMode {
		t.Error("mode stayed TerminalMode after the terminal window closed")
	}
}

func TestPollTerminalLeavesFocusAloneWhenTheShellExitsInTheBackground(t *testing.T) {
	e := state.New()

	inWindows(t, e)

	term := openTerminal(t, e)
	detachFromTerminal(t, e)
	elsewhere := view.Focused()

	term.Entry.Terminal.Kill()
	waitUntilExited(t, term.Entry.Terminal)
	view.PollTerminal(e)

	wantWindowCount(t, e, 1)
	if view.Focused() != elsewhere {
		t.Error("an unfocused terminal exiting moved focus")
	}
}

func TestPollTerminalRefusesToCloseTheLastWindow(t *testing.T) {
	e := state.New()

	inWindows(t, e)

	term := openTerminal(t, e)
	detachFromTerminal(t, e)
	edtest.Press(t, e, " wd") // closes the window detached to, leaving only the terminal

	wantWindowCount(t, e, 1)

	term.Entry.Terminal.Kill()
	waitUntilExited(t, term.Entry.Terminal)
	view.PollTerminal(e)

	wantWindowCount(t, e, 1)
}

func TestPollTerminalResizesThePtyToMatchAResizedPane(t *testing.T) {
	e := state.New()

	inWindows(t, e)

	term := openTerminal(t, e)

	e.ScreenCols = 120
	view.Layout(e)
	view.PollTerminal(e)

	if cols, rows := term.Entry.Terminal.Size(); cols != term.Rect.Cols || rows != term.Rect.Rows {
		t.Errorf("pty size = %dx%d, want %dx%d", cols, rows, term.Rect.Cols, term.Rect.Rows)
	}
}
