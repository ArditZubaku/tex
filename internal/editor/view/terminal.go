package view

import (
	"slices"

	"github.com/nsf/termbox-go"

	"github.com/ArditZubaku/tex/internal/buffer"
	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/ArditZubaku/tex/internal/editor/terminal"
)

// There is at most one terminal open at a time, the way there is at most one
// preview: '<leader>ft' finds it wherever it already is rather than opening a
// second shell beside a second buffer.
var terminalWin *Window

// OpenOrFocusTerminal is '<leader>ft': open-or-focus only, never a toggle, so
// the one key that reaches the shell can't also be the one that kills it.
func OpenOrFocusTerminal(e *state.Editor) {
	if terminalWin != nil {
		focusWindow(e, terminalWin)
		return
	}

	w := CurrentWindow(e)
	if w.Rect.Cols <= 2*minWindowCols {
		e.StatusMsg = "E36: Not enough room"
		return
	}

	sess, err := terminal.Start(w.Rect.Cols, w.Rect.Rows)
	if err != nil {
		e.StatusMsg = err.Error()
		return
	}

	SyncWindow(e)
	fresh := &Window{Entry: &Entry{Terminal: sess, Buf: buffer.NewEmpty()}}
	root = root.InsertBeside(w, true, fresh)
	Layout(e)
	sess.Resize(fresh.Rect.Cols, fresh.Rect.Rows)
	terminalWin = fresh
	applyWindow(e, fresh)
}

// PollTerminal runs once a frame: it's the only way a shell that exited on
// its own, rather than through its window closing, is ever noticed, and it's
// what keeps the pty caught up with a pane resized by a split or a drag —
// cheap enough, unlike glow's own re-render, to just check every frame.
func PollTerminal(e *state.Editor) {
	if terminalWin == nil {
		return
	}

	sess := terminalWin.Entry.Terminal
	if sess.Exited() {
		closeTerminal(e)
		return
	}

	if cols, rows := sess.Size(); cols != terminalWin.Rect.Cols || rows != terminalWin.Rect.Rows {
		sess.Resize(terminalWin.Rect.Cols, terminalWin.Rect.Rows)
	}
}

// closeTerminal is reached whether the shell exited on its own or the window
// it's in was closed out from under it. Like CloseWindow's own E444, it
// refuses to prune the last real window rather than leave the editor with
// none; unlike CloseWindow, the window closed here is not always the one
// focused, so focus only moves when it actually was.
func closeTerminal(e *state.Editor) {
	if terminalWin == nil || realCount(List(e)) < 2 {
		return
	}

	SyncWindow(e)
	wasCurrent := terminalWin == current
	list := List(e)
	next := slices.Index(list, terminalWin)

	terminalWin.Entry.Terminal.Kill()
	root = root.Prune(terminalWin)
	terminalWin = nil
	Layout(e)

	if !wasCurrent {
		return
	}

	list = List(e)
	applyWindow(e, list[min(next, len(list)-1)])
}

// detachPending marks that the last key was Ctrl-\, the first half of the
// detach chord: SSH and tmux both borrow a two-key escape out of a session
// that otherwise reads every key as its own, for the same reason.
var detachPending bool

// TerminalKey is the shell's own keymap: everything typed goes to the pty as
// if a real terminal had read it. Ctrl-\ Ctrl-N is the one sequence that
// doesn't — it leaves the shell running and moves focus off the window.
func TerminalKey(e *state.Editor, keyEvent termbox.Event) {
	sess := current.Entry.Terminal

	if detachPending {
		detachPending = false
		if keyEvent.Key == termbox.KeyCtrlN {
			detach(e)
			return
		}
		sess.Write([]byte{byte(termbox.KeyCtrlBackslash)})
	}

	if keyEvent.Key == termbox.KeyCtrlBackslash {
		detachPending = true
		return
	}

	sess.Write(terminal.Encode(keyEvent, sess.AppCursor()))
}

func detach(e *state.Editor) {
	for _, w := range List(e) {
		if w != current {
			focusWindow(e, w)
			return
		}
	}
}
