package notify

import (
	"strings"
	"testing"
	"time"

	"github.com/ArditZubaku/tex/internal/layout"
)

func at(t *testing.T, when time.Time) {
	t.Helper()

	now = func() time.Time { return when }
	t.Cleanup(func() { now = time.Now })
}

func TestWrapBreaksOnSpaces(t *testing.T) {
	got := wrap("a.go:2:6: expected declaration", 12)

	want := []string{"a.go:2:6:", "expected", "declaration"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("wrap = %q, want %q", got, want)
	}
}

func TestWrapBreaksAWordTooLongForTheBox(t *testing.T) {
	got := wrap("aa bbbbbbbb", 4)

	want := []string{"aa", "bbbb", "bbbb"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("wrap = %q, want %q", got, want)
	}
}

func TestWrapKeepsWhatFitsOnOneLine(t *testing.T) {
	if got := wrap("expected declaration", 40); len(got) != 1 || got[0] != "expected declaration" {
		t.Errorf("wrap = %q, want it left whole", got)
	}
}

func TestTheBoxSitsInTheTopRightCorner(t *testing.T) {
	var n Note
	n.Show("gofmt", "expected declaration")

	within := layout.Rect{Row: 1, Col: 0, Rows: 24, Cols: 80}
	_, frame := n.frame(within)

	if frame.Row != within.Row {
		t.Errorf("row = %d, want the top row of the area", frame.Row)
	}
	if right := frame.Col + frame.Cols; right != within.Cols {
		t.Errorf("right edge at %d, want the area's own %d", right, within.Cols)
	}
}

func TestTheBoxIsSizedToTheMessage(t *testing.T) {
	var n Note
	n.Show("gofmt", "short")

	_, frame := n.frame(layout.Rect{Row: 1, Rows: 24, Cols: 200})

	if frame.Cols > maxCols || frame.Rows != 3 {
		t.Errorf("frame = %dx%d, want one line inside a frame no wider than %d", frame.Cols, frame.Rows, maxCols)
	}
}

func TestALongMessageIsCutToTheRowsTheBoxHas(t *testing.T) {
	var n Note
	n.Show("gofmt", strings.Repeat("word ", 200))

	lines, frame := n.frame(layout.Rect{Row: 1, Rows: 24, Cols: 80})

	if len(lines) != maxRows-2 || frame.Rows != maxRows {
		t.Errorf("frame = %d rows over %d lines, want %d", frame.Rows, len(lines), maxRows)
	}
}

func TestANoteStopsShowingOnceItsTimeHasRunOut(t *testing.T) {
	raised := time.Now()
	at(t, raised)

	var n Note
	n.Show("gofmt", "expected declaration")

	at(t, raised.Add(ttl-time.Second))
	if !n.Showing() {
		t.Error("gone before its time was up")
	}

	at(t, raised.Add(ttl))
	if n.Showing() {
		t.Error("still showing past its time")
	}
}

func TestClearTakesTheNoteDown(t *testing.T) {
	var n Note
	n.Show("gofmt", "expected declaration")
	n.Clear()

	if n.Showing() || n.Text() != "" {
		t.Errorf("note = %q, want nothing left", n.Text())
	}
}
