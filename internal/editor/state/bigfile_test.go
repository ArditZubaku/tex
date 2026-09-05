package state_test

import (
	"testing"

	"github.com/ArditZubaku/tex/internal/buffer"
	"github.com/ArditZubaku/tex/internal/editor/edtest"
	"github.com/ArditZubaku/tex/internal/editor/state"
)

// An off-by-one in the window bounds surfaces here as a panic or stuck cursor.
func TestNavigationOverBigFile(t *testing.T) {
	e := state.New()

	e.Buf = buffer.Open(edtest.BigFile(t, 20000))
	defer e.Buf.Close()

	edtest.SingleWindow(e, 30, 80)
	e.Row, e.Col = 0, 0

	for range 100 {
		e.Down()
	}
	if e.Row != 100 {
		t.Fatalf("after 100 down, row = %d", e.Row)
	}

	e.GoToBottom()
	if e.Row != e.Buf.LineCount()-1 {
		t.Fatalf("G landed on row %d, want %d", e.Row, e.Buf.LineCount()-1)
	}
	e.Down()
	if e.Row != e.Buf.LineCount()-1 {
		t.Fatalf("down past the last line moved to %d", e.Row)
	}

	for range 20 {
		e.Up()
	}
	for range 200 {
		e.NextWord()
	}
	for range 200 {
		e.PrevWord()
	}
	for range 50 {
		e.EndOfWord()
	}

	e.GoToTop()
	if e.Row != 0 || e.Col != 0 {
		t.Fatalf("gg landed on %d,%d", e.Row, e.Col)
	}
	e.Up()
	e.Left()
	if e.Row != 0 || e.Col != 0 {
		t.Fatalf("moving off the top-left moved to %d,%d", e.Row, e.Col)
	}

	for range 40 {
		e.PageDown()
	}
	for range 100 {
		e.PageUp()
	}
	if e.Row != 0 {
		t.Fatalf("pageUp past the top landed on %d", e.Row)
	}
}
