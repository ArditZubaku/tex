package view_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ArditZubaku/tex/internal/editor/edtest"
	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/ArditZubaku/tex/internal/editor/view"
)

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func initCommittedRepo(t *testing.T) (root, path, hash string) {
	t.Helper()

	root = t.TempDir()
	gitRun(t, root, "init", "-q")
	gitRun(t, root, "config", "user.email", "a@example.com")
	gitRun(t, root, "config", "user.name", "A Author")

	path = filepath.Join(root, "f.go")
	if err := os.WriteFile(path, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, root, "add", "f.go")
	gitRun(t, root, "commit", "-q", "-m", "initial scaffold")

	out, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}

	return root, path, strings.TrimSpace(string(out))
}

// openGitShow is ' gs' at the view layer, with the real 'git show' process it
// starts always killed at the end of the test that opened it.
func openGitShow(t *testing.T, e *state.Editor, root, hash, path string) *view.Window {
	t.Helper()

	view.OpenGitShow(e, root, hash, path)
	w := view.Focused()
	t.Cleanup(func() {
		if !w.Entry.Terminal.Exited() {
			w.Entry.Terminal.Kill()
		}
	})

	return w
}

func TestOpenGitShowOpensAPaneBesideTheWindowAndFocusesIt(t *testing.T) {
	e := state.New()
	inWindows(t, e)
	root, path, hash := initCommittedRepo(t)

	pane := openGitShow(t, e, root, hash, path)

	wantWindowCount(t, e, 2)
	if view.Focused() != pane {
		t.Error("focus did not move to the git-show pane")
	}
	if pane.Entry.Terminal == nil {
		t.Fatal("OpenGitShow() did not start a terminal-backed pane")
	}
	if e.Mode != state.TerminalMode {
		t.Errorf("mode = %v, want TerminalMode", e.Mode)
	}
}

func TestOpenGitShowRefusesWhenThereIsNoRoomForIt(t *testing.T) {
	e := state.New()
	inWindows(t, e)
	edtest.SingleWindow(e, 20, 30)
	root, path, hash := initCommittedRepo(t)

	view.OpenGitShow(e, root, hash, path)

	wantWindowCount(t, e, 1)
	if e.StatusMsg != "E36: Not enough room" {
		t.Errorf("statusMsg = %q", e.StatusMsg)
	}
}

func TestOpenGitShowOnADifferentCommitReplacesTheRunningOneRatherThanAddingASecondPane(t *testing.T) {
	e := state.New()
	inWindows(t, e)
	root, path, hash := initCommittedRepo(t)

	first := openGitShow(t, e, root, hash, path)

	gitRun(t, root, "commit", "--allow-empty", "-q", "-m", "second")
	out, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	second := strings.TrimSpace(string(out))

	oldSession := first.Entry.Terminal
	pane := openGitShow(t, e, root, second, path)

	wantWindowCount(t, e, 2)
	if pane != first {
		t.Error("asking about a different commit opened a second window instead of reusing the pane")
	}
	if pane.Entry.Terminal == oldSession {
		t.Error("the old 'git show' process was not replaced")
	}
	waitUntilExited(t, oldSession)
}

func TestPollGitShowClosesTheWindowWhenItExitsOnItsOwn(t *testing.T) {
	e := state.New()
	inWindows(t, e)
	root, path, hash := initCommittedRepo(t)

	pane := openGitShow(t, e, root, hash, path)
	other := view.List(e)[0]

	pane.Entry.Terminal.Kill()
	waitUntilExited(t, pane.Entry.Terminal)
	view.PollGitShow(e)

	wantWindowCount(t, e, 1)
	if view.Focused() != other {
		t.Error("focus did not land on the window left")
	}
	if e.Mode == state.TerminalMode {
		t.Error("mode stayed TerminalMode after the git-show pane closed")
	}
}

func TestPollGitShowResizesThePtyToMatchAResizedPane(t *testing.T) {
	e := state.New()
	inWindows(t, e)
	root, path, hash := initCommittedRepo(t)

	pane := openGitShow(t, e, root, hash, path)

	e.ScreenCols = 120
	view.Layout(e)
	view.PollGitShow(e)

	if cols, rows := pane.Entry.Terminal.Size(); cols != pane.Rect.Cols || rows != pane.Rect.Rows {
		t.Errorf("pty size = %dx%d, want %dx%d", cols, rows, pane.Rect.Cols, pane.Rect.Rows)
	}
}

// TestOpenGitShowStartsAtItsFinalSizeRatherThanTheFullWindow guards the bug
// behind SettleGitShowResize's own: 'git show' reads the pane's width once,
// so starting it at the pre-split width and resizing the pty straight after
// is too late — the fix is a placeholder window that Layout sizes first, with
// 'git show' only run once that is known, which this checks for without ever
// calling PollGitShow (whose own resize would otherwise paper over it).
func TestOpenGitShowStartsAtItsFinalSizeRatherThanTheFullWindow(t *testing.T) {
	e := state.New()
	inWindows(t, e)
	root, path, hash := initCommittedRepo(t)

	pane := openGitShow(t, e, root, hash, path)

	if cols, rows := pane.Entry.Terminal.Size(); cols != pane.Rect.Cols || rows != pane.Rect.Rows {
		t.Errorf("pty started at %dx%d, want its own pane's %dx%d", cols, rows, pane.Rect.Cols, pane.Rect.Rows)
	}
}

func TestSettleGitShowResizeRespawnsAfterADragChangesTheSize(t *testing.T) {
	e := state.New()
	inWindows(t, e)
	root, path, hash := initCommittedRepo(t)

	pane := openGitShow(t, e, root, hash, path)
	oldSession := pane.Entry.Terminal

	e.ScreenCols = 160
	view.Layout(e)
	// No PollGitShow here: the point is that the pty's own resize is not
	// what SettleGitShowResize depends on.
	view.SettleGitShowResize(e)
	t.Cleanup(func() {
		if !pane.Entry.Terminal.Exited() {
			pane.Entry.Terminal.Kill()
		}
	})

	if pane.Entry.Terminal == oldSession {
		t.Fatal("SettleGitShowResize() did not run 'git show' again")
	}
	waitUntilExited(t, oldSession)
	if cols, rows := pane.Entry.Terminal.Size(); cols != pane.Rect.Cols || rows != pane.Rect.Rows {
		t.Errorf("new pty size = %dx%d, want %dx%d", cols, rows, pane.Rect.Cols, pane.Rect.Rows)
	}
}

func TestDraggingTheSeparatorBesideAGitShowPaneRespawnsItAtTheNewWidth(t *testing.T) {
	e := state.New()
	inWindows(t, e)
	root, path, hash := initCommittedRepo(t)

	pane := openGitShow(t, e, root, hash, path)
	oldSession := pane.Entry.Terminal

	sep := view.Separators()
	if len(sep) != 1 {
		t.Fatalf("separators = %v, want exactly one", sep)
	}
	row, col := sep[0].Row, sep[0].Col

	edtest.PressMouse(t, e, row, col)
	edtest.DragMouse(t, e, row, col-10)
	edtest.ReleaseMouse(t, e, row, col-10)
	t.Cleanup(func() {
		if !pane.Entry.Terminal.Exited() {
			pane.Entry.Terminal.Kill()
		}
	})

	if pane.Entry.Terminal == oldSession {
		t.Fatal("dragging the separator did not respawn 'git show'")
	}
	waitUntilExited(t, oldSession)
	if cols, rows := pane.Entry.Terminal.Size(); cols != pane.Rect.Cols || rows != pane.Rect.Rows {
		t.Errorf("pty size after drag = %dx%d, want %dx%d", cols, rows, pane.Rect.Cols, pane.Rect.Rows)
	}
}

func TestSettleGitShowResizeDoesNothingWhenTheSizeDidNotChange(t *testing.T) {
	e := state.New()
	inWindows(t, e)
	root, path, hash := initCommittedRepo(t)

	pane := openGitShow(t, e, root, hash, path)
	before := pane.Entry.Terminal

	view.SettleGitShowResize(e)

	if pane.Entry.Terminal != before {
		t.Error("SettleGitShowResize() restarted 'git show' when nothing had changed")
	}
}
