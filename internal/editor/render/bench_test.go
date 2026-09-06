package render

import (
	"testing"

	"github.com/ArditZubaku/tex/internal/buffer"
	"github.com/ArditZubaku/tex/internal/editor/edit"
	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/ArditZubaku/tex/internal/editor/view"
	"github.com/ArditZubaku/tex/internal/fixture"
	"github.com/ArditZubaku/tex/internal/search"
	"github.com/ArditZubaku/tex/internal/syntax"
)

// termbox is never initialised here, so its back buffer has zero width and
// every SetCell falls out on its bounds check. What is measured is therefore
// the editor's own per-frame work — layout, buffer reads, highlighting, hit
// scanning — with the terminal write itself left out.
const (
	benchRows = 48
	benchCols = 160
	fileLines = 20_000
)

func benchEditor(b *testing.B, path string) *state.Editor {
	b.Helper()

	view.Reset()
	Reset()

	e := state.New()
	e.Buf = buffer.Open(path)
	b.Cleanup(func() {
		e.Buf.Close()
		view.Reset()
		Reset()
	})
	e.SourceFile = path
	e.Lang = syntax.Detect(path)
	e.ScreenRows, e.ScreenCols = benchRows, benchCols
	e.Rows, e.Cols = benchRows, benchCols
	e.WinRow, e.WinCol = state.TabBarRows, 0
	view.Layout(e)

	return e
}

// frame is what the editor's loop draws between two keystrokes.
func frame(e *state.Editor) {
	view.Layout(e)
	BufferLine(e)
	Windows(e)
	StatusBar(e)
}

func BenchmarkFrameSource(b *testing.B) {
	e := benchEditor(b, fixture.GoSource(b, fileLines))

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		frame(e)
		e.Row, e.OffsetRow = e.Row+1, e.OffsetRow+1
		if e.OffsetRow > fileLines-benchRows-2 {
			e.Row, e.OffsetRow = 0, 0
		}
	}
}

// The same frame with no language behind it, which is the highlighter's share
// of the cost on its own.
func BenchmarkFramePlain(b *testing.B) {
	e := benchEditor(b, fixture.PlainText(b, fileLines))

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		frame(e)
		e.Row, e.OffsetRow = e.Row+1, e.OffsetRow+1
		if e.OffsetRow > fileLines-benchRows-2 {
			e.Row, e.OffsetRow = 0, 0
		}
	}
}

// The frame a keystroke that does not scroll draws, which is most of them: the
// cursor moves inside the window and the viewport stays where it is.
func BenchmarkFrameCursorMove(b *testing.B) {
	e := benchEditor(b, fixture.GoSource(b, fileLines))
	e.OffsetRow = fileLines / 2

	b.ReportAllocs()
	b.ResetTimer()

	row := e.OffsetRow
	for b.Loop() {
		e.Row = row
		frame(e)
		row++
		if row >= e.OffsetRow+benchRows {
			row = e.OffsetRow
		}
	}
}

// A lit search adds a match scan to every visible line.
func BenchmarkFrameSearchLit(b *testing.B) {
	e := benchEditor(b, fixture.GoSource(b, fileLines))
	e.SearchPat, e.HlSearch = search.New([]rune("req")), true

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		frame(e)
		e.Row, e.OffsetRow = e.Row+1, e.OffsetRow+1
		if e.OffsetRow > fileLines-benchRows-2 {
			e.Row, e.OffsetRow = 0, 0
		}
	}
}

// A visual selection makes every cell of every row ask whether it is inside.
func BenchmarkFrameVisual(b *testing.B) {
	e := benchEditor(b, fixture.GoSource(b, fileLines))
	edit.StartVisualLine(e)
	e.AnchorRow = 0

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		frame(e)
		e.Row, e.OffsetRow = e.Row+1, e.OffsetRow+1
		if e.OffsetRow > fileLines-benchRows-2 {
			e.Row, e.OffsetRow, e.AnchorRow = 0, 0, 0
		}
	}
}

// A vertical split draws the file twice, which is what the layout pass and the
// per-window state swap cost on top of one window's worth of drawing.
func BenchmarkFrameSplit(b *testing.B) {
	e := benchEditor(b, fixture.GoSource(b, fileLines))
	if !view.Split(e, true) {
		b.Fatal("split refused")
	}

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		frame(e)
		e.Row, e.OffsetRow = e.Row+1, e.OffsetRow+1
		if e.OffsetRow > fileLines-benchRows-2 {
			e.Row, e.OffsetRow = 0, 0
		}
	}
}

// Text on its own, with the buffer line, the status bar and the layout pass
// left out, so that a change to the drawing loop is not hidden by them.
func BenchmarkText(b *testing.B) {
	e := benchEditor(b, fixture.GoSource(b, fileLines))

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		Text(e)
		e.Row, e.OffsetRow = e.Row+1, e.OffsetRow+1
		if e.OffsetRow > fileLines-benchRows-2 {
			e.Row, e.OffsetRow = 0, 0
		}
	}
}

func BenchmarkStatusBar(b *testing.B) {
	e := benchEditor(b, fixture.GoSource(b, fileLines))

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		StatusBar(e)
	}
}

// The look-back the top of a window pays when the language has block comments.
func BenchmarkBlockStateBefore(b *testing.B) {
	e := benchEditor(b, fixture.GoSource(b, fileLines))

	b.ReportAllocs()
	b.ResetTimer()

	row := blockLookback
	for b.Loop() {
		_ = blockStateBefore(e, row)
		row++
		if row > fileLines-2 {
			row = blockLookback
		}
	}
}
