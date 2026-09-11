package prompt

import (
	"testing"

	"github.com/nsf/termbox-go"
)

func key(ch rune) termbox.Event { return termbox.Event{Ch: ch} }

func keyCode(k termbox.Key) termbox.Event { return termbox.Event{Key: k} }

func TestInsertAtCursor(t *testing.T) {
	var l Line
	l.Start(':')
	l.Key(key('a'))
	l.Key(key('c'))
	l.Key(keyCode(termbox.KeyArrowLeft))
	l.Key(key('b'))

	if got, want := string(l.Input()), "abc"; got != want {
		t.Errorf("Input() = %q, want %q", got, want)
	}
}

func TestBackspaceAtCursor(t *testing.T) {
	var l Line
	l.StartWith(':', "abc")
	l.Key(keyCode(termbox.KeyArrowLeft))
	l.Key(keyCode(termbox.KeyBackspace))

	if got, want := string(l.Input()), "ac"; got != want {
		t.Errorf("Input() = %q, want %q", got, want)
	}
}

func TestBackspaceAtStartIsNoOp(t *testing.T) {
	var l Line
	l.StartWith(':', "abc")
	l.Key(keyCode(termbox.KeyHome))

	if action := l.Key(keyCode(termbox.KeyBackspace)); action != Handled {
		t.Errorf("action = %v, want Handled", action)
	}
	if got, want := string(l.Input()), "abc"; got != want {
		t.Errorf("Input() = %q, want %q", got, want)
	}
}

func TestBackspaceOnEmptyCloses(t *testing.T) {
	var l Line
	l.Start(':')

	if action := l.Key(keyCode(termbox.KeyBackspace)); action != Closed {
		t.Errorf("action = %v, want Closed", action)
	}
}

func TestArrowsClamp(t *testing.T) {
	var l Line
	l.StartWith(':', "ab")
	for range 5 {
		l.Key(keyCode(termbox.KeyArrowRight))
	}
	l.Key(key('c'))
	if got, want := string(l.Input()), "abc"; got != want {
		t.Errorf("Input() = %q, want %q", got, want)
	}

	for range 5 {
		l.Key(keyCode(termbox.KeyArrowLeft))
	}
	l.Key(key('z'))
	if got, want := string(l.Input()), "zabc"; got != want {
		t.Errorf("Input() = %q, want %q", got, want)
	}
}

func TestHomeEnd(t *testing.T) {
	var l Line
	l.StartWith(':', "abc")
	l.Key(keyCode(termbox.KeyHome))
	l.Key(key('z'))
	l.Key(keyCode(termbox.KeyEnd))
	l.Key(key('!'))

	if got, want := string(l.Input()), "zabc!"; got != want {
		t.Errorf("Input() = %q, want %q", got, want)
	}
}

func TestCtrlUClearsCursorToStart(t *testing.T) {
	var l Line
	l.StartWith(':', "abcdef")
	l.Key(keyCode(termbox.KeyArrowLeft))
	l.Key(keyCode(termbox.KeyArrowLeft))
	l.Key(keyCode(termbox.KeyCtrlU))

	if got, want := string(l.Input()), "ef"; got != want {
		t.Errorf("Input() = %q, want %q", got, want)
	}
}

func TestCursorWidthTracksPosition(t *testing.T) {
	var l Line
	l.StartWith(':', "abc")
	if got, want := l.CursorWidth(), len(":abc"); got != want {
		t.Errorf("CursorWidth() = %d, want %d", got, want)
	}

	l.Key(keyCode(termbox.KeyHome))
	if got, want := l.CursorWidth(), len(":"); got != want {
		t.Errorf("CursorWidth() = %d, want %d", got, want)
	}
}
