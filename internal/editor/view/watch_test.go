package view_test

import (
	"os"
	"testing"
	"time"

	"github.com/ArditZubaku/tex/internal/editor/edtest"
	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/ArditZubaku/tex/internal/editor/view"
)

// pollUntil polls the way the editor's own loop does, since a real fsnotify
// event arrives off its own goroutine rather than on the call that caused it.
func pollUntil(t *testing.T, e *state.Editor, cond func() bool) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		view.PollWatch(e)
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out waiting for the watch to pick up the change")
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPollWatchReloadsAFileWrittenFromOutsideTex(t *testing.T) {
	e := state.New()
	edtest.InReadMode(t, e, "one\ntwo\n", 0, 0)
	path := e.SourceFile
	view.CurrentEntry(e) // seeds the buffer list, which is what starts the watch

	writeFile(t, path, "three\nfour\nfive\n")

	pollUntil(t, e, func() bool { return e.Buf.LineCount() == 3 })
	edtest.WantLines(t, e.Buf, "three", "four", "five")
}

func TestPollWatchResyncsAnUnsavedFileWithoutLosingTheEdit(t *testing.T) {
	e := state.New()
	edtest.InReadMode(t, e, "one\ntwo\n", 0, 0)
	path := e.SourceFile
	view.CurrentEntry(e)

	e.Buf.SetLine(0, []rune("ONE EDITED"))
	e.Modified = true
	view.SyncBuffer(e)

	writeFile(t, path, "three\nfour\nfive\n")

	pollUntil(t, e, func() bool { return e.StatusMsg != "" })
	// Row 0 is the unsaved edit, kept exactly as the conflict left it; rows 1
	// and 2 were never edited, so they read through to what is on disk now.
	edtest.WantLines(t, e.Buf, "ONE EDITED", "four", "five")
}
