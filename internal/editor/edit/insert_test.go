package edit_test

import (
	"strings"
	"testing"

	"github.com/nsf/termbox-go"

	"github.com/ArditZubaku/tex/internal/buffer"
	"github.com/ArditZubaku/tex/internal/editor/edit"
	"github.com/ArditZubaku/tex/internal/editor/edtest"
	"github.com/ArditZubaku/tex/internal/editor/state"
)

func TestInsertDoesNotCorruptNeighbours(t *testing.T) {
	e := state.New()

	e.Buf = buffer.Open(edtest.BigFile(t, 20000))
	defer e.Buf.Close()

	const row = 5001
	before := []string{string(e.Buf.Line(row - 1)), string(e.Buf.Line(row + 1))}

	e.Row, e.Col = row, 0
	for _, ch := range "abc" {
		edit.InsertRune(e, termbox.Event{Ch: ch})
	}

	if got := string(e.Buf.Line(row)); !strings.HasPrefix(got, "abc") {
		t.Fatalf("edited line = %q, want it to start with abc", got[:min(10, len(got))])
	}
	if got := string(e.Buf.Line(row - 1)); got != before[0] {
		t.Errorf("line above changed")
	}
	if got := string(e.Buf.Line(row + 1)); got != before[1] {
		t.Errorf("line below changed")
	}

	// The edit must survive a refill from elsewhere in the file.
	_ = e.Buf.Line(19000)
	if got := string(e.Buf.Line(row)); !strings.HasPrefix(got, "abc") {
		t.Error("edit lost after the window moved")
	}
}
