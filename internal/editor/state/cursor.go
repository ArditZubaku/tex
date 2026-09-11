package state

import (
	"log/slog"
	"os"

	"github.com/ArditZubaku/tex/internal/chars"
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
	e.Center()
}

// Center is the window moved so that the cursor's line is the middle row of it,
// and nothing else. It is separate from CenterView because a jump that lands
// off screen centres on where it landed: reading the count there would take
// '3gd' — three jumps to a declaration — as a jump to line three.
func (e *Editor) Center() {
	e.OffsetRow = max(e.Row-e.Rows/2, 0)
}

func (e *Editor) CenterIfOffScreen() {
	if e.Row < e.OffsetRow || e.Row >= e.OffsetRow+e.Rows {
		e.Center()
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

// EndOfLine is VIM's '$', landing on the last rune of the line rather than the
// gap past it that GoToEndOfLine's Insert-mode append needs.
func (e *Editor) EndOfLine() {
	e.Col = e.MaxCol(e.Row)
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

// MatchBracket is VIM's '%': to the match of the bracket under the cursor, or
// of the first one after it on the line. A count in front of it means VIM's
// other '%' entirely — the line that far through the file — which is why the
// count is read here rather than the command being run that many times.
func (e *Editor) MatchBracket() {
	if e.HadCount {
		e.toPercentOfFile()

		return
	}

	row, col, ok := motion.MatchFrom(e.Buf, e.Row, e.Col)
	if !ok {
		return
	}

	e.PushJump()
	e.Row, e.Col = row, col
}

// A percentage past a hundred is no part of the file, which VIM refuses rather
// than rounds down to the end of it.
func (e *Editor) toPercentOfFile() {
	count := e.Count()
	if count > 100 {
		return
	}

	e.PushJump()
	lines := e.Buf.LineCount()
	e.Row = min((count*lines+99)/100, lines) - 1
	e.Col = firstNonBlank(e.Buf.Line(e.Row))
}

func firstNonBlank(line []rune) int {
	for at, ch := range line {
		if !chars.IsSpace(ch) {
			return at
		}
	}

	return 0
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
