// Package render is one frame: the windows and what is in them, the buffer
// line above them and the status line below.
package render

import (
	"path/filepath"
	"strconv"

	"github.com/nsf/termbox-go"

	"github.com/ArditZubaku/tex/internal/editor/diag"
	"github.com/ArditZubaku/tex/internal/editor/edit"
	"github.com/ArditZubaku/tex/internal/editor/explorer"
	"github.com/ArditZubaku/tex/internal/editor/find"
	"github.com/ArditZubaku/tex/internal/editor/screen"
	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/ArditZubaku/tex/internal/editor/tabbar"
	"github.com/ArditZubaku/tex/internal/editor/view"
	"github.com/ArditZubaku/tex/internal/gutter"
	"github.com/ArditZubaku/tex/internal/theme"
)

// textScratch is the row being drawn, decoded into an array that outlives the
// frame: a redraw reads every visible line and keeps none of them.
var textScratch []rune

func Text(e *state.Editor) {
	bufLen := e.Buf.LineCount()
	gutterCols := gutter.Width(bufLen)
	textCols := e.Cols - gutterCols
	inBlock := blockStateBefore(e, e.OffsetRow)
	selected := edit.Selection(e)
	notes := diag.Of(e.SourceFile)

	for row := range e.Rows {
		textBufRow := row + e.OffsetRow

		// Past end of buffer: draw line indicator once per row
		if textBufRow >= bufLen {
			termbox.SetCell(e.ScreenCol(0), e.ScreenRow(row), '*', e.Palette.EndOfBuffer, e.Palette.Background)
			continue
		}
		if textBufRow < 0 {
			continue
		}

		// Most files have nothing said about them, and the zero value already
		// answers "nothing on this row" for every column of it.
		var marks diag.Rows
		if len(notes) > 0 {
			marks = notes.Row(textBufRow)
		}

		numberColor, background := e.Palette.LineNumber, e.Palette.Background
		if textBufRow == e.Row {
			numberColor, background = e.Palette.CursorLineNumber, e.Palette.CursorLineBg
			screen.Fill(e.ScreenCol(0), e.ScreenRow(row), e.Cols, e.Palette.Plain, e.Palette.CursorLineBg)
		}
		// The number rather than a column of its own: a marker beside the text
		// would move every line across the moment a server came up, and the row
		// is worth marking even when what was said about it is scrolled out of
		// sight sideways.
		if worst := marks.Worst(); worst != diag.None {
			numberColor = severityColor(&e.Palette, worst)
		}
		screen.Print(e.ScreenCol(0), e.ScreenRow(row), numberColor, background, gutter.Label(textBufRow, e.Row, gutterCols))

		textScratch = e.Buf.LineInto(textBufRow, textScratch)
		paint := rowPaint{
			line:       textScratch,
			hits:       find.LineHits(e, textBufRow),
			marks:      marks,
			selected:   selected,
			row:        textBufRow,
			screenRow:  e.ScreenRow(row),
			startCol:   e.ScreenCol(gutterCols),
			background: background,
		}
		paint.colors, inBlock = lineColors(e, paint.line, inBlock)

		// Past the end of the line there is nothing to draw unless a selection
		// bands over it, and the frame was cleared before any of this ran.
		paint.cols = textCols
		if !selected.Active {
			paint.cols = max(min(textCols, len(paint.line)-e.OffsetCol), 0)
		}

		drawRunes(e, paint)
	}
}

// A rowPaint is one row of text with everything it is drawn against already
// resolved: what the row holds, where on screen it starts and how far it
// reaches, and the colours that belong to the whole of it rather than to a cell.
type rowPaint struct {
	line       []rune
	colors     []termbox.Attribute
	hits       find.HitScan
	marks      diag.Rows
	selected   edit.Span
	row        int
	screenRow  int
	startCol   int
	cols       int
	background termbox.Attribute
}

// drawRunes is the cells of one row. Which colour a cell ends up in is settled
// here and in this order, each beating the one before it: the plain text, what
// the syntax says, what a server said about it, what a search matched, and what
// a selection covers. The last two are deliberate, transient acts, which is why
// they win over a diagnostic that will still be there afterwards.
func drawRunes(e *state.Editor, paint rowPaint) {
	lineLen := len(paint.line)

	for col := range paint.cols {
		textBufCol := col + e.OffsetCol
		if textBufCol < 0 {
			continue
		}
		inSelection := paint.selected.Covers(paint.row, textBufCol, lineLen)

		if textBufCol >= lineLen {
			if inSelection {
				termbox.SetCell(paint.startCol+col, paint.screenRow, ' ', e.Palette.Plain, e.Palette.VisualBg)
			}
			continue
		}

		ch := paint.line[textBufCol]
		if ch == '\t' {
			ch = ' '
		}

		foreground, cellBackground := e.Palette.Plain, paint.background
		if paint.colors != nil {
			foreground = paint.colors[textBufCol]
		}
		if severity := paint.marks.Under(textBufCol); severity != diag.None {
			foreground = severityColor(&e.Palette, severity) | termbox.AttrUnderline
		}
		if paint.hits.Covers(textBufCol) {
			foreground, cellBackground = e.Palette.MatchFg, e.Palette.MatchBg
		}
		// the selection keeps the text's own colours and takes the background,
		// which is what makes it read as a band over them
		if inSelection {
			cellBackground = e.Palette.VisualBg
		}
		termbox.SetCell(paint.startCol+col, paint.screenRow, ch, foreground, cellBackground)
	}
}

// Info and a hint share a colour: the difference between them is not worth a
// third shade the eye has to learn.
func severityColor(palette *theme.Palette, severity diag.Severity) termbox.Attribute {
	switch severity {
	case diag.Error:
		return palette.DiagError
	case diag.Warning:
		return palette.DiagWarn
	default:
		return palette.DiagHint
	}
}

func StatusBar(e *state.Editor) {
	screen.Print(0, e.StatusRow(), e.Palette.StatusFg, e.Palette.StatusBg, Status(e))
}

// Status is the bottom line as text, padded out to the width so that the bar
// takes the whole of it in the status colours. It is built as a string rather
// than drawn straight onto the screen so that what it says can be read back:
// termbox is never initialised under test, and a cell nothing was drawn into
// cannot be told from one drawn off screen.
func Status(e *state.Editor) string {
	if txt, ok := e.PromptStatus(); ok {
		return screen.Pad(txt, e.ScreenCols)
	}

	if e.ExplorerOpen {
		return screen.Pad(explorer.Status(e), e.ScreenCols)
	}

	var modeStatus string

	switch {
	case e.Mode == state.EditMode:
		modeStatus = " EDIT: "
	case e.Mode == state.VisualMode && e.VisualLine:
		modeStatus = " V-LINE: "
	case e.Mode == state.VisualMode:
		modeStatus = " VISUAL: "
	default:
		modeStatus = " VIEW: "
	}

	// the name alone: a file opened by the picker or by 'gd' carries the whole
	// path it was found at, which says nothing the buffer line does not
	name := filepath.Base(e.SourceFile)
	fileNameLen := min(len(name), 16)

	status := "saved"
	if e.Modified {
		status = "modified"
	}
	right := rightScratch[:0]
	if e.PendingCount > 0 {
		right = strconv.AppendInt(right, int64(e.PendingCount), 10)
		right = append(right, ' ')
	}
	right = append(right, "Row "...)
	right = strconv.AppendInt(right, int64(e.Row+1), 10)
	right = append(right, ", Col "...)
	right = strconv.AppendInt(right, int64(e.Col+1), 10)
	right = append(right, ' ')
	rightScratch = right

	shown := name[:fileNameLen]
	line := lineScratch[:0]
	line = append(line, modeStatus...)
	line = append(line, shown...)
	line = append(line, " - "...)
	line = strconv.AppendInt(line, int64(e.Buf.LineCount()), 10)
	line = append(line, " lines "...)
	line = append(line, status...)
	if !e.Clip.Empty() {
		line = append(line, " [Copy]"...)
	}
	if e.Hist.CanUndo() {
		line = append(line, " [Undo]"...)
	}
	if e.Hist.CanRedo() {
		line = append(line, " [Redo]"...)
	}
	line = diagFlag(e.SourceFile, line)

	// Everything in the left half but the file name is ASCII, so only its own
	// width has to be measured.
	width := len(line) - len(shown) + screen.Width(shown)

	for ; width < e.ScreenCols-len(right); width++ {
		line = append(line, ' ')
	}
	line = append(line, right...)
	lineScratch = line

	return string(line)
}

// The count is a flag beside [Copy] and [Undo] rather than a line of its own,
// and stops at warnings: a bar reporting eleven hints has spent the width that
// mattered.
func diagFlag(path string, into []byte) []byte {
	errors, warnings := diag.Count(path)
	if errors+warnings == 0 {
		return into
	}

	into = append(into, " ["...)
	if errors > 0 {
		into = strconv.AppendInt(into, int64(errors), 10)
		into = append(into, 'E')
	}
	if warnings > 0 {
		if errors > 0 {
			into = append(into, ' ')
		}
		into = strconv.AppendInt(into, int64(warnings), 10)
		into = append(into, 'W')
	}

	return append(into, ']')
}

// NoteDiagnostic keeps the box in the corner over the worst thing said about
// the cursor's line, called every frame so that it lasts exactly as long as
// the cursor sits there and goes the moment it leaves.
func NoteDiagnostic(e *state.Editor) {
	note, ok := diag.At(e.SourceFile, e.Row)
	if !ok {
		e.Note.ClearDiagnostic()

		return
	}

	e.Note.ShowDiagnostic(severityTitle(note.Severity), note.Message)
}

func severityTitle(severity diag.Severity) string {
	switch severity {
	case diag.Error:
		return "error"
	case diag.Warning:
		return "warning"
	case diag.Info:
		return "info"
	default:
		return "hint"
	}
}

// The status bar is rebuilt on every frame, so its two halves are assembled in
// buffers that outlive the frame rather than in fresh strings.
var lineScratch, rightScratch []byte

func Scroll(e *state.Editor) {
	textCols := e.Cols - gutter.Width(e.Buf.LineCount())

	if e.Row < e.OffsetRow {
		e.OffsetRow = e.Row
	}

	if e.Col < e.OffsetCol {
		e.OffsetCol = e.Col
	}

	if e.Row >= e.OffsetRow+e.Rows {
		e.OffsetRow = e.Row - e.Rows + 1
	}

	if e.Col >= e.OffsetCol+textCols {
		e.OffsetCol = e.Col - textCols + 1
	}
}

var tabs tabbar.Bar

// Reset drops the buffer line's own scroll, so that a test starts from the
// editor as it is before any file has been opened.
func Reset() { tabs = tabbar.Bar{} }

// Windows draws every window through the one renderer, pointing the editor at
// each in turn. Drawing changes nothing the editor is doing, so the current
// window's own state is put back at the end.
func Windows(e *state.Editor) {
	view.SyncWindow(e)
	live := e.Mode

	for _, w := range view.List(e) {
		// only the window being worked in draws its mode: a selection belongs
		// to the window it was made in, not to every view of the buffer
		e.Mode = live
		if w != view.Focused() {
			e.Mode = state.ReadMode
		}

		view.ShowWindow(e, w)
		if e.ExplorerOpen && w == view.Focused() {
			explorer.Draw(e)
			continue
		}
		Scroll(e)
		Text(e)
		w.OffsetRow, w.OffsetCol = e.OffsetRow, e.OffsetCol
	}

	e.Mode = live
	view.ShowWindow(e, view.Focused())
	separators(e)
}

func separators(e *state.Editor) {
	for _, s := range view.Separators() {
		ch := '─'
		if s.Vertical {
			ch = '│'
		}
		for i := range s.Length {
			row, col := s.Row, s.Col
			if s.Vertical {
				row += i
			} else {
				col += i
			}
			termbox.SetCell(col, row, ch, e.Palette.Separator, e.Palette.Background)
		}
	}
}

func BufferLine(e *state.Editor) {
	tabs.Draw(0, e.ScreenCols, Tabs(e), view.Index(), &e.Palette)
}

func Tabs(e *state.Editor) []tabbar.Tab {
	view.SyncBuffer(e)

	open := make([]tabbar.Tab, 0, len(view.Buffers()))
	for _, entry := range view.Buffers() {
		open = append(open, tabbar.Tab{Name: filepath.Base(entry.Path), Modified: entry.Modified})
	}

	return open
}
