// Package hover is what a language server knows about the identifier under the
// cursor, in a box beside it: 'K', the way VIM's own 'K' opens a page about the
// word it is pressed on.
package hover

import (
	"strings"

	"github.com/mattn/go-runewidth"

	"github.com/ArditZubaku/tex/internal/editor/screen"
	"github.com/ArditZubaku/tex/internal/layout"
	"github.com/ArditZubaku/tex/internal/theme"
)

// The box is drawn this wide and this tall at most, and shrinks to whatever the
// screen has. A doc comment longer than the rows is cut rather than scrolled:
// the file it belongs to is a 'gd' away, and that is where it is read properly.
// Under minCols there is no room for a frame with anything inside it.
const (
	maxCols = 72
	maxRows = 16
	minCols = 12
)

// A Box is what the server said, or nothing: the editor holds one, and asking
// about another identifier replaces what it held. Nothing expires it — unlike
// the error box in the corner, this one was asked for, so it stays until it is
// dismissed or another one takes its place.
type Box struct {
	lines []string
}

// Show is what the server said, as the lines it is drawn on: the markup is
// flattened here rather than at drawing time, since the box's own width is what
// it was wrapped to.
func (b *Box) Show(markup string, within layout.Rect) {
	b.lines = Lines(markup, min(maxCols, within.Cols)-4)
	if len(b.lines) > maxRows-2 {
		b.lines = b.lines[:maxRows-2]
	}
}

func (b *Box) Clear() { b.lines = nil }

func (b *Box) Showing() bool { return len(b.lines) > 0 }

// Text is what the box is showing, which is what a test reads it by.
func (b *Box) Text() string { return strings.Join(b.lines, "\n") }

// Draw puts the box under the cursor's own line, or above it when there is no
// room below: a box over the line being asked about hides the answer's subject.
func (b *Box) Draw(within layout.Rect, cursorRow, cursorCol int, palette *theme.Palette) {
	if !b.Showing() {
		return
	}

	frame := b.frame(within, cursorRow, cursorCol)
	if frame.Cols < minCols {
		return
	}

	inner := frame.Cols - 2
	screen.Print(frame.Col, frame.Row, palette.Separator, palette.Background,
		"┌"+strings.Repeat("─", inner)+"┐")
	for i, line := range b.lines {
		screen.Print(frame.Col, frame.Row+1+i, palette.Separator, palette.Background, "│")
		screen.Print(frame.Col+1, frame.Row+1+i, palette.Plain, palette.Background,
			screen.Pad(" "+line, inner))
		screen.Print(frame.Col+frame.Cols-1, frame.Row+1+i, palette.Separator, palette.Background, "│")
	}
	screen.Print(frame.Col, frame.Row+frame.Rows-1, palette.Separator, palette.Background,
		"└"+strings.Repeat("─", inner)+"┘")
}

func (b *Box) frame(within layout.Rect, cursorRow, cursorCol int) layout.Rect {
	widest := 0
	for _, line := range b.lines {
		widest = max(widest, runewidth.StringWidth(line))
	}

	rows := len(b.lines) + 2
	cols := min(widest+4, within.Cols)
	row := cursorRow + 1
	if row+rows > within.Row+within.Rows {
		row = max(cursorRow-rows, within.Row)
	}

	return layout.Rect{
		Row:  row,
		Col:  min(cursorCol, within.Col+within.Cols-cols),
		Rows: rows,
		Cols: cols,
	}
}

// Lines is a server's markup as lines of plain text. A terminal has no bold and
// no headings to draw, and the fences a server wraps a signature in say nothing
// once the signature is the only thing in the box — so they come off, and what
// is left is the text with its own line breaks kept and its long lines wrapped.
func Lines(markup string, width int) []string {
	var lines []string
	for _, para := range paragraphs(markup) {
		if para == "" {
			lines = append(lines, "")

			continue
		}
		lines = append(lines, screen.Wrap(para, width)...)
	}

	return trimmed(lines)
}

// A fenced block keeps the line breaks it was written with — it is code, and
// rewrapping a signature across a comma is worse than cutting it — while prose
// is joined into a paragraph and wrapped to the box.
func paragraphs(markup string) []string {
	var out []string
	fenced, prose := false, ""
	flush := func() {
		if prose != "" {
			out = append(out, prose)
			prose = ""
		}
	}

	for _, line := range strings.Split(markup, "\n") {
		switch {
		case strings.HasPrefix(strings.TrimSpace(line), "```"):
			flush()
			fenced = !fenced
			out = append(out, "")
		case fenced:
			out = append(out, line)
		case strings.TrimSpace(line) == "":
			flush()
			out = append(out, "")
		default:
			prose = strings.TrimSpace(prose + " " + strings.TrimSpace(line))
		}
	}
	flush()

	return out
}

// The fences and the blank lines around them leave runs of empty lines at both
// ends and in the middle, which would be most of a short box.
func trimmed(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if line == "" && (len(out) == 0 || out[len(out)-1] == "") {
			continue
		}
		out = append(out, line)
	}
	for len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}

	return out
}
