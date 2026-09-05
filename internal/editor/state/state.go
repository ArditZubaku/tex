// Package state is the editor's live state: the file being edited, where the
// cursor is in it, what mode the keys are read in, and the room there is to
// draw it. Everything the editor does is done to one of these.
package state

import (
	"time"

	"github.com/ArditZubaku/tex/internal/buffer"
	"github.com/ArditZubaku/tex/internal/editor/filetree"
	"github.com/ArditZubaku/tex/internal/editor/history"
	"github.com/ArditZubaku/tex/internal/editor/picker"
	"github.com/ArditZubaku/tex/internal/editor/prompt"
	"github.com/ArditZubaku/tex/internal/editor/register"
	"github.com/ArditZubaku/tex/internal/layout"
	"github.com/ArditZubaku/tex/internal/search"
	"github.com/ArditZubaku/tex/internal/syntax"
	"github.com/ArditZubaku/tex/internal/theme"
)

type Mode int

const (
	ReadMode Mode = iota
	EditMode
	PromptMode
	VisualMode
	ExplorerMode
	PickerMode
)

// DefaultFileName is what an unnamed buffer writes to, the editor's own stand-in
// for VIM's "[No Name]".
const DefaultFileName = "out.txt"

const NoWriteSinceChange = "E37: No write since last change (add ! to override)"

// TabBarRows is the first row of the screen, which the buffer line takes;
// everything the buffer and the explorer draw sits below it.
const TabBarRows = 1

// MaxCount keeps a fat-fingered "99999999p" from hanging the editor.
const MaxCount = 9999

// ChordTimeout bounds how long the keys typed so far towards a chord (e.g.
// "gg", or "<leader>bd") stay pending before they're treated as fresh,
// unrelated keypresses.
const ChordTimeout = 500 * time.Millisecond

// A Jump is what Ctrl-O steps back through: where the cursor was before a jump
// took it somewhere else, the file it was in included.
type Jump struct {
	Path string
	Row  int
	Col  int
}

type Editor struct {
	Buf        *buffer.Buffer
	SourceFile string
	Lang       *syntax.Syntax
	Modified   bool

	Row, Col             int
	OffsetRow, OffsetCol int

	// Rows and Cols are the window being drawn; ScreenRows and ScreenCols the
	// area every window shares, and WinRow and WinCol where this one starts in
	// it.
	Rows, Cols             int
	WinRow, WinCol         int
	ScreenRows, ScreenCols int

	Mode      Mode
	StatusMsg string
	Quitting  bool

	// PendingCount is the count typed so far, CmdCount the one the running
	// command was given. A count of 1 and no count at all mean different things
	// to zz, which is what HadCount is for.
	PendingCount int
	CmdCount     int
	HadCount     bool

	// PendingKeys is the chord being typed: every key of it that has not
	// resolved to a command yet, held until the next one either names one or
	// cannot.
	PendingKeys []rune
	PendingTime time.Time

	Palette theme.Palette

	Hist history.History
	Clip register.Register

	Prompt       prompt.Line
	Pick         picker.Picker
	Exp          filetree.Tree
	ExplorerOpen bool

	Jumps []Jump

	// SearchPat is the pattern n and N repeat, SearchBack the direction it was
	// last run in, and HlSearch whether the matches are still lit.
	SearchPat  search.Pattern
	SearchBack bool
	HlSearch   bool

	// Visual mode's anchor: the end of the selection a motion does not drag.
	VisualLine           bool
	AnchorRow, AnchorCol int
}

func New() *Editor {
	return &Editor{Palette: theme.Default()}
}

// Count is how much of itself the running command should do, defaulting to
// once when no count was typed.
func (e *Editor) Count() int {
	return max(e.CmdCount, 1)
}

// MaxCol is the rightmost column the cursor may occupy on a row. Outside Edit
// mode the cursor sits *on* a rune, the way VIM's normal mode does, so it stops
// one short of the gap past the last one that Edit mode needs in order to
// append.
func (e *Editor) MaxCol(row int) int {
	lineLen := e.Buf.RuneLen(row)
	if e.Mode != EditMode && lineLen > 0 {
		return lineLen - 1
	}

	return lineLen
}

func (e *Editor) ClampCol() {
	if m := e.MaxCol(e.Row); e.Col > m {
		e.Col = m
	}
}

// LineBytes is the line as it is held: the raw bytes of one still on disk, and
// the runes of one the overlay has edited.
func (e *Editor) LineBytes(row int) []byte {
	if line, ok := e.Buf.EditedLine(row); ok {
		return []byte(string(line))
	}

	return e.Buf.Raw(row)
}

func (e *Editor) ScreenRow(row int) int { return e.WinRow + row }
func (e *Editor) ScreenCol(col int) int { return e.WinCol + col }

func (e *Editor) StatusRow() int { return e.ScreenRows + TabBarRows }

// WindowArea is the room the window being drawn has to itself; ScreenArea is
// everything below the buffer line, which is what a popup centres in.
func (e *Editor) WindowArea() layout.Rect {
	return layout.Rect{Row: e.WinRow, Col: e.WinCol, Rows: e.Rows, Cols: e.Cols}
}

func (e *Editor) ScreenArea() layout.Rect {
	return layout.Rect{Row: TabBarRows, Rows: e.ScreenRows, Cols: e.ScreenCols}
}

// Close lets the editor's loop fall out and shut the terminal down on its way,
// so quitting runs the same path whether it was 'q' or ':q' that asked.
func (e *Editor) StartPrompt(delimiter rune) {
	e.Mode = PromptMode
	e.Prompt.Start(delimiter)
}

func (e *Editor) Close() { e.Quitting = true }

func (e *Editor) PushJump() {
	e.Jumps = append(e.Jumps, Jump{Path: e.SourceFile, Row: e.Row, Col: e.Col})
}

func (e *Editor) PopJump() (Jump, bool) {
	if len(e.Jumps) == 0 {
		return Jump{}, false
	}

	back := e.Jumps[len(e.Jumps)-1]
	e.Jumps = e.Jumps[:len(e.Jumps)-1]

	return back, true
}
