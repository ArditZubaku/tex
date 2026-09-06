package keys_test

import (
	"os"
	"testing"

	"github.com/nsf/termbox-go"

	"github.com/ArditZubaku/tex/internal/buffer"
	"github.com/ArditZubaku/tex/internal/editor/keys"
	"github.com/ArditZubaku/tex/internal/editor/render"
	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/ArditZubaku/tex/internal/editor/view"
	"github.com/ArditZubaku/tex/internal/fixture"
	"github.com/ArditZubaku/tex/internal/syntax"
)

const fileLines = 20_000

// The editor writes a DECSCUSR escape straight to stdout when the mode
// changes, which would land in the middle of the benchmark output.
func muteCursorShape(b *testing.B) {
	b.Helper()

	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		b.Fatal(err)
	}

	stdout := os.Stdout
	os.Stdout = devNull
	b.Cleanup(func() {
		os.Stdout = stdout
		_ = devNull.Close()
	})
}

func benchEditor(b *testing.B) *state.Editor {
	b.Helper()

	muteCursorShape(b)
	view.Reset()
	render.Reset()

	path := fixture.GoSource(b, fileLines)
	e := state.New()
	e.Buf = buffer.Open(path)
	b.Cleanup(func() {
		e.Buf.Close()
		view.Reset()
		render.Reset()
	})
	e.SourceFile, e.Lang = path, syntax.Detect(path)
	e.ScreenRows, e.ScreenCols = 48, 160
	e.Rows, e.Cols = 48, 160
	e.WinRow, e.WinCol = state.TabBarRows, 0
	e.CmdCount = 1
	view.Layout(e)

	return e
}

func press(e *state.Editor, keyEvents []termbox.Event) {
	for _, keyEvent := range keyEvents {
		keys.Dispatch(e, keyEvent)
	}
}

func runes(s string) []termbox.Event {
	out := make([]termbox.Event, 0, len(s))
	for _, ch := range s {
		out = append(out, termbox.Event{Ch: ch})
	}

	return out
}

// One 'j' is the cheapest key there is, and the one held down the longest.
func BenchmarkDown(b *testing.B) {
	e := benchEditor(b)
	keyEvent := termbox.Event{Ch: 'j'}

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		keys.Dispatch(e, keyEvent)
		if e.Row > fileLines-2 {
			e.Row = 0
		}
	}
}

// The word motions decode the line they walk, which is the cache's job.
func BenchmarkWordMotions(b *testing.B) {
	e := benchEditor(b)
	seq := runes("wwwbbbeee")

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		press(e, seq)
		if e.Row > fileLines-2 {
			e.Row, e.Col = 0, 0
		}
	}
}

// Typing: entering Edit mode, a run of characters into one line, and Esc.
func BenchmarkTypeRun(b *testing.B) {
	e := benchEditor(b)
	seq := append(runes("ifunc handler(ctx context.Context) error {"),
		termbox.Event{Key: termbox.KeyEsc})

	b.ReportAllocs()
	b.ResetTimer()

	row := 0
	for b.Loop() {
		e.Row, e.Col = row, 0
		press(e, seq)
		row = (row + 1) % (fileLines - 1)
	}
}

// A chord goes through the pending-key path, which every key of it pays for.
func BenchmarkChord(b *testing.B) {
	e := benchEditor(b)
	seq := runes("gg")

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		press(e, seq)
	}
}

// A count is read a digit at a time before the command it belongs to runs.
func BenchmarkCountedMotion(b *testing.B) {
	e := benchEditor(b)
	seq := runes("12j")

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		press(e, seq)
		if e.Row > fileLines-20 {
			e.Row = 0
		}
	}
}

// A line opened, typed into and undone is the whole change-recording path.
func BenchmarkOpenLineAndUndo(b *testing.B) {
	e := benchEditor(b)
	typed := append(runes("ototal += weights[i] * float64(i)"),
		termbox.Event{Key: termbox.KeyEsc})
	undo := runes("u")

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		press(e, typed)
		press(e, undo)
	}
}

// Enter splits a line and Backspace joins it back, which is the structural
// edit path: the index moves and the overlay is re-keyed both ways.
func BenchmarkSplitAndJoin(b *testing.B) {
	e := benchEditor(b)
	enter := []termbox.Event{{Ch: 'i'}, {Key: termbox.KeyEnter}, {Key: termbox.KeyEsc}}
	backspace := []termbox.Event{{Ch: 'i'}, {Key: termbox.KeyBackspace}, {Key: termbox.KeyEsc}}

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		e.Row, e.Col = fileLines/2, 4
		press(e, enter)
		e.Row, e.Col = fileLines/2+1, 0
		press(e, backspace)
	}
}

// Paging is a motion with a redraw's worth of scrolling behind it.
func BenchmarkPageDown(b *testing.B) {
	e := benchEditor(b)
	keyEvent := termbox.Event{Key: termbox.KeyCtrlD}

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		keys.Dispatch(e, keyEvent)
		if e.Row > fileLines-100 {
			e.Row = 0
		}
	}
}
