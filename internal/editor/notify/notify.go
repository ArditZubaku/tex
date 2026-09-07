// Package notify is the box in the top-right corner: one note, drawn over
// whatever is on screen, for something away from the status line — a
// formatter refusing the file a save has just written, the language server
// going away, or a diagnostic on the line the cursor sits on.
package notify

import (
	"strings"
	"time"

	"github.com/mattn/go-runewidth"

	"github.com/ArditZubaku/tex/internal/editor/screen"
	"github.com/ArditZubaku/tex/internal/layout"
	"github.com/ArditZubaku/tex/internal/theme"
)

// The box is drawn this wide at most and shrinks to whatever the screen has; a
// message too long for the rows is cut rather than pushed off the bottom.
// Under minCols there is no room for a frame with anything inside it.
const (
	maxCols = 52
	maxRows = 6
	minCols = 8
)

// Long enough to be read at a glance, short enough to be gone by the time the
// line it complains about has been fixed. Nothing wakes the editor, so a note
// outlives this until the next key redraws the screen — which is what leaves it
// up for as long as nobody is typing.
const ttl = 6 * time.Second

// now is the clock the expiry is measured against, which the tests replace.
var now = time.Now

// A Note is the box, or nothing: the editor holds one, and raising another
// replaces what it held.
type Note struct {
	title  string
	text   string
	raised time.Time
	auto   bool
}

func (n *Note) Show(title, text string) {
	*n = Note{title: title, text: text, raised: now()}
}

// ShowDiagnostic is the box's other source: the diagnostic on the cursor's own
// line, meant to be called every frame so it lasts exactly as long as the
// cursor sits there. It never interrupts a note something explicit just
// raised.
func (n *Note) ShowDiagnostic(title, text string) {
	if n.Showing() && !n.auto {
		return
	}
	*n = Note{title: title, text: text, raised: now(), auto: true}
}

func (n *Note) Clear() { *n = Note{} }

// ClearDiagnostic drops the box the moment the cursor leaves the line that
// raised it, leaving an explicit note to run out its own time undisturbed.
func (n *Note) ClearDiagnostic() {
	if n.auto {
		n.Clear()
	}
}

// Text is what the box is showing, which is what a test reads it by.
func (n *Note) Text() string { return n.text }

// Showing is both a note having been raised and its time not having run out.
func (n *Note) Showing() bool {
	return n.text != "" && now().Sub(n.raised) < ttl
}

func (n *Note) Draw(within layout.Rect, palette *theme.Palette) {
	if !n.Showing() {
		n.Clear() // one whose time has run out is dropped rather than asked again
		return
	}

	lines, frame := n.frame(within)
	if frame.Cols < minCols || len(lines) == 0 {
		return
	}

	inner := frame.Cols - 2
	top := []rune("┌" + strings.Repeat("─", inner) + "┐")
	copy(top[2:len(top)-1], []rune(screen.Truncate(" "+n.title+" ", inner-2, false)))

	screen.Print(frame.Col, frame.Row, palette.Error, palette.Background, string(top))
	for i, line := range lines {
		screen.Print(frame.Col, frame.Row+1+i, palette.Error, palette.Background,
			"│"+screen.Pad(" "+line, inner)+"│")
	}
	screen.Print(frame.Col, frame.Row+frame.Rows-1, palette.Error, palette.Background,
		"└"+strings.Repeat("─", inner)+"┘")
}

// frame is where the box goes and what it holds: the top-right corner of the
// area, sized to the message rather than to the screen, since an error of a few
// words has no business covering a corner's worth of the file.
func (n *Note) frame(within layout.Rect) ([]string, layout.Rect) {
	lines := screen.Wrap(n.text, min(maxCols, within.Cols)-4) // the frame's edges and a space either side
	if len(lines) > maxRows-2 {
		lines = lines[:maxRows-2]
	}

	widest := runewidth.StringWidth(n.title) + 2
	for _, line := range lines {
		widest = max(widest, runewidth.StringWidth(line))
	}
	cols := min(widest+4, within.Cols)

	return lines, layout.Rect{
		Row:  within.Row,
		Col:  within.Col + within.Cols - cols,
		Rows: len(lines) + 2,
		Cols: cols,
	}
}
