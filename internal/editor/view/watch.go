package view

import (
	"os"

	"github.com/ArditZubaku/tex/internal/editor/history"
	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/ArditZubaku/tex/internal/editor/watch"
)

// PollWatch runs once a frame: it's what notices a file changed from outside
// tex, the way PollTerminal notices a shell exited on its own. A stat compares
// what changed against what the buffer last read or wrote itself, since the
// watch a save's own rename fires is indistinguishable from another
// program's write at the fsnotify event alone.
func PollWatch(e *state.Editor) {
	for _, path := range watch.Changed() {
		entry := Buffer(path)
		if entry == nil {
			continue
		}

		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		size, mtime := entry.Buf.DiskStat()
		if info.Size() == size && info.ModTime().Equal(mtime) {
			continue
		}

		if entry.Modified {
			resyncEntry(e, entry)
			continue
		}

		reloadEntry(e, entry)
	}
}

// resyncEntry is reloadEntry's gentler cousin, for an entry a conflict left
// alone rather than reloaded: the overlay and the history are the unsaved work
// the conflict chose to keep, not reformat's or Reload's to discard. Only the
// file behind the unedited lines needs to catch up, which is Resync rather
// than Reload.
func resyncEntry(e *state.Editor, entry *Entry) {
	if err := entry.Buf.Resync(entry.Path); err != nil {
		return // the next change event, if one comes, gets another try
	}
	state.Edited(entry.Path, 0, state.FileRewritten)

	if entry == CurrentEntry(e) {
		e.StatusMsg = ChangedOnDisk(entry)
		e.Row = min(e.Row, e.Buf.LineCount()-1)
		e.ClampCol()
		return
	}

	entry.Row = min(entry.Row, entry.Buf.LineCount()-1)
	entry.Col = min(entry.Col, entry.Buf.RuneLen(entry.Row))
}

// reloadEntry is reformat's own read-back, for a change that landed from
// outside tex rather than from its own formatter. Unlike reformat's, the
// history is always dropped rather than only when the line count moved: a
// formatter stays close to what was just typed, but whatever wrote this could
// have rewritten every line, and undo replaying rows against content it never
// produced would be worse than undo having nothing to say at all.
func reloadEntry(e *state.Editor, entry *Entry) {
	entry.Buf.Reload(entry.Path)
	state.Edited(entry.Path, 0, state.FileRewritten)

	if entry == CurrentEntry(e) {
		e.Hist = history.History{}
		e.Row = min(e.Row, e.Buf.LineCount()-1)
		e.ClampCol()
		return
	}

	entry.Hist = history.History{}
	entry.Row = min(entry.Row, entry.Buf.LineCount()-1)
	entry.Col = min(entry.Col, entry.Buf.RuneLen(entry.Row))
}
