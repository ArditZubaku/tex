package view

import (
	"slices"

	"github.com/ArditZubaku/tex/internal/buffer"
	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/ArditZubaku/tex/internal/editor/terminal"
)

// There is at most one git-show pane open at a time, the way there is at most
// one terminal and one preview: asking about a different line replaces what
// is running rather than piling up another live process beside it.
//
// gitShowRoot/Hash/File is what it is currently showing, and gitShowCols/Rows
// is the size 'git show' — or rather its pager — actually read when it last
// ran: git and delta both measure the terminal once and format to that, the
// way glow does for the markdown preview, so neither the pty quietly being
// resized nor the pane simply being wider on screen reflows anything already
// written. Only running the command again does, which respawnGitShow is for.
var (
	gitShowWin                            *Window
	gitShowRoot, gitShowHash, gitShowFile string
	gitShowCols, gitShowRows              int
)

// OpenGitShow is ' gs': 'git show' for hash, scoped to file, run inside a
// real pty rather than captured as text — so git sees an actual terminal on
// its other end and pages and colours the diff through whatever it finds
// configured (delta, or 'less' with nothing set), exactly as it would typed
// by hand.
func OpenGitShow(e *state.Editor, repoRoot, hash, file string) {
	gitShowRoot, gitShowHash, gitShowFile = repoRoot, hash, file

	if gitShowWin != nil {
		respawnGitShow(e, gitShowWin.Rect.Cols, gitShowWin.Rect.Rows)
		focusWindow(e, gitShowWin)

		return
	}

	w := CurrentWindow(e)
	if w.Rect.Cols <= 2*minWindowCols {
		e.StatusMsg = "E36: Not enough room"

		return
	}

	// The pane isn't laid out yet, so there's no right size to start at — a
	// placeholder goes in the tree first, Layout says what it actually got,
	// and only then does 'git show' run, at that size from its very first
	// byte rather than guessed-then-corrected.
	SyncWindow(e)
	fresh := &Window{Entry: &Entry{Buf: buffer.NewEmpty()}}
	root = root.InsertBeside(w, true, fresh)
	Layout(e)

	gitShowWin = fresh
	if !respawnGitShow(e, fresh.Rect.Cols, fresh.Rect.Rows) {
		root = root.Prune(fresh)
		gitShowWin = nil
		Layout(e)

		return
	}
	applyWindow(e, fresh)
}

// respawnGitShow is 'git show' run again at cols by rows: opening the pane,
// asking about a different line and a drag settling at a new size are all
// the same problem underneath — whatever is running is behind the size the
// pane needs now — so all three go through here. The old process, if there
// is one, is only killed once the new one has actually started.
func respawnGitShow(e *state.Editor, cols, rows int) bool {
	sess, err := terminal.StartCommand("git", []string{"-C", gitShowRoot, "show", gitShowHash, "--", gitShowFile}, cols, rows)
	if err != nil {
		e.StatusMsg = err.Error()

		return false
	}

	if old := gitShowWin.Entry.Terminal; old != nil {
		old.Kill()
	}
	gitShowWin.Entry = &Entry{Terminal: sess, Buf: buffer.NewEmpty()}
	gitShowCols, gitShowRows = cols, rows
	if current == gitShowWin {
		ShowWindow(e, gitShowWin)
	}

	return true
}

// PollGitShow runs once a frame beside PollTerminal: quitting the pager ends
// 'git show' itself, which this is the only way to notice. It also keeps the
// pty's own size caught up to the pane's, the same as a plain shell's, so
// 'less' can still page what is already there while a drag is in progress —
// SettleGitShowResize is what fixes the formatting itself once it stops.
func PollGitShow(e *state.Editor) {
	if gitShowWin == nil {
		return
	}

	sess := gitShowWin.Entry.Terminal
	if sess.Exited() {
		closeGitShow(e)

		return
	}

	if cols, rows := sess.Size(); cols != gitShowWin.Rect.Cols || rows != gitShowWin.Rect.Rows {
		sess.Resize(gitShowWin.Rect.Cols, gitShowWin.Rect.Rows)
	}
}

// SettleGitShowResize is a separator drag letting go: the pane's own pty size
// tracks the drag as it happens (PollGitShow), but git and delta already
// wrote what they wrote at the size they started with, so only running 'git
// show' again picks up a new one — the gap SettlePreviewResize closes for
// glow. Like that one, it runs once the drag is over rather than on every
// column it crosses, since neither is cheap enough to pay for that often.
func SettleGitShowResize(e *state.Editor) {
	if gitShowWin == nil || (gitShowWin.Rect.Cols == gitShowCols && gitShowWin.Rect.Rows == gitShowRows) {
		return
	}

	respawnGitShow(e, gitShowWin.Rect.Cols, gitShowWin.Rect.Rows)
}

// closeGitShow is reached whether 'git show' exited on its own or the window
// it was in was closed out from under it, same as closeTerminal.
func closeGitShow(e *state.Editor) {
	if gitShowWin == nil || realCount(List(e)) < 2 {
		return
	}

	SyncWindow(e)
	wasCurrent := gitShowWin == current
	list := List(e)
	next := slices.Index(list, gitShowWin)

	gitShowWin.Entry.Terminal.Kill()
	root = root.Prune(gitShowWin)
	gitShowWin = nil
	Layout(e)

	if !wasCurrent {
		return
	}

	list = List(e)
	applyWindow(e, list[min(next, len(list)-1)])
}
