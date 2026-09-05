package state

import (
	"log/slog"
	"os"

	"github.com/ArditZubaku/tex/internal/motion"
)

// ANSI DECSCUSR escape sequences for cursor styles
const (
	CursorDefault       = "\033[0 q" // Terminal default
	CursorBlinkingBlock = "\033[1 q"
	CursorSteadyBlock   = "\033[2 q"
	CursorBlinkingUnder = "\033[3 q"
	CursorSteadyUnder   = "\033[4 q"
	CursorBlinkingBar   = "\033[5 q" // Blinking vertical line
	CursorSteadyBar     = "\033[6 q" // Steady vertical line
)

func SetCursorShape(shape string) {
	_, err := os.Stdout.WriteString(shape)
	if err != nil {
		slog.Error("Failed to update cursor's shape", "error", err)
	}
}

func (e *Editor) Up() {
	if e.Row != 0 {
		e.Row--
	}
}

func (e *Editor) Down() {
	if e.Row < e.Buf.LineCount()-1 {
		e.Row++
	}
}

// Left and Right stay on their line in Read mode, the way VIM's h and l do.
// Only Edit mode wraps, so that typing can run off one line onto the next.
func (e *Editor) Left() {
	if e.Col != 0 {
		e.Col--
		return
	}
	if e.Mode == EditMode && e.Row > 0 {
		e.Row--
		e.Col = e.MaxCol(e.Row)
	}
}

func (e *Editor) Right() {
	if e.Col < e.MaxCol(e.Row) {
		e.Col++
		return
	}
	if e.Mode == EditMode && e.Row < e.Buf.LineCount()-1 {
		e.Row++
		e.Col = 0
	}
}

func (e *Editor) PageUp() {
	if (e.Row - e.Rows/2) > 0 {
		e.Row -= e.Rows / 2
	} else {
		e.Row = 0
	}
}

func (e *Editor) PageDown() {
	if (e.Row + e.Rows/2) < e.Buf.LineCount()-1 {
		e.Row += e.Rows / 2
	} else {
		e.Row = e.Buf.LineCount() - 1
	}
}

// CenterView is VIM's zz: the cursor's line is redrawn in the middle of the
// window, keeping its column. A count names the line to centre on instead.
// Near the bottom of the buffer the window is left hanging past the last line,
// the way VIM does rather than pinning the last line to the bottom row.
func (e *Editor) CenterView() {
	if e.HadCount {
		e.Row = min(e.Count()-1, e.Buf.LineCount()-1)
		e.ClampCol()
	}
	e.OffsetRow = max(e.Row-e.Rows/2, 0)
}

func (e *Editor) CenterIfOffScreen() {
	if e.Row < e.OffsetRow || e.Row >= e.OffsetRow+e.Rows {
		e.CenterView()
	}
}

func (e *Editor) GoToTop() {
	e.Row, e.Col = 0, 0
}

func (e *Editor) GoToBottom() {
	e.Row, e.Col = e.Buf.LineCount()-1, 0
}

func (e *Editor) GoToEndOfLine() {
	e.Col = e.Buf.RuneLen(e.Row)
	e.EnterEditMode()
}

func (e *Editor) GoToStartOfLine() {
	e.Col = 0
	e.EnterEditMode()
}

func (e *Editor) EditAfterWord() {
	e.Col++
	e.EnterEditMode()
}

func (e *Editor) EditBeforeWord() {
	e.EnterEditMode()
}

func (e *Editor) EnterEditMode() {
	e.BeginChange()
	e.Mode = EditMode
	SetCursorShape(CursorBlinkingBar)
}

func (e *Editor) NextWord() {
	e.Row, e.Col = motion.NextWordFrom(e.Buf, e.Row, e.Col)
}

func (e *Editor) EndOfWord() {
	e.Row, e.Col = motion.EndOfWordFrom(e.Buf, e.Row, e.Col)
}

func (e *Editor) PrevWord() {
	e.Row, e.Col = motion.PrevWordFrom(e.Buf, e.Row, e.Col)
}
