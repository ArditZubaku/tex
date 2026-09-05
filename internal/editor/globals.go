package editor

import (
	"time"

	"github.com/ArditZubaku/txi/internal/buffer"
	"github.com/ArditZubaku/txi/internal/syntax"
	"github.com/ArditZubaku/txi/internal/theme"
)

var (
	ROWS, COLS             int
	offsetRow, offsetCol   int
	currentRow, currentCol int
	buf                    *buffer.Buffer
	sourceFile             string
	lang                   *syntax.Syntax
	mode                   Mode
	modified               bool
	quitting               bool
)

// pendingCount is the count typed so far, cmdCount the one the running command
// was given. maxCount keeps a fat-fingered "99999999p" from hanging the editor.
var (
	pendingCount int
	cmdCount     int
	hadCount     bool // a count of 1 and no count at all mean different things to zz
)

const maxCount = 9999

// defaultFileName is what an unnamed buffer writes to, the editor's own stand-in
// for VIM's "[No Name]".
const defaultFileName = "out.txt"

// count is how much of itself the running command should do, defaulting to
// once when no count was typed.
func count() int {
	return max(cmdCount, 1)
}

// chordTimeout bounds how long the keys typed so far towards a chord (e.g.
// "gg", or "<leader>bd") stay pending before they're treated as fresh,
// unrelated keypresses.
const chordTimeout = 500 * time.Millisecond

// pendingKeys is the chord being typed: every key of it that has not resolved
// to a command yet, held until the next one either names one or cannot.
var (
	pendingKeys []rune
	pendingTime time.Time
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

// active is the palette everything on screen is drawn in, which ':theme' swaps
// for another.
var active = theme.Default()
