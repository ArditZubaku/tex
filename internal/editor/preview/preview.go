// Package preview is glow rendering a markdown file for the split beside it:
// whether glow is on the PATH, which files are its to render, and turning its
// coloured output into cells the editor can draw straight onto the screen.
package preview

import (
	"bytes"
	"context"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/nsf/termbox-go"
)

// The editor's loop blocks on the keyboard, so a glow that hangs would hang
// the editor with it.
const timeout = 5 * time.Second

// style is fixed rather than read from the terminal: glow only colours its
// output at all when told a style outright, auto included, so leaving this
// unset would silently draw the file in plain text instead.
const style = "dark"

var markdownExt = map[string]bool{
	".md": true, ".markdown": true, ".mdown": true, ".mkd": true,
}

func IsMarkdown(path string) bool {
	return markdownExt[strings.ToLower(filepath.Ext(path))]
}

var (
	found  string
	looked bool
)

func Available() bool {
	return lookPath() != ""
}

func lookPath() string {
	if !looked {
		bin, err := exec.LookPath("glow")
		if err != nil {
			bin = ""
		}
		found, looked = bin, true
	}

	return found
}

// Reset drops what the lookup found, so that a test is not answered from
// another one's PATH.
func Reset() {
	found, looked = "", false
}

// A Cell is one glyph of glow's own rendering, at the colour glow drew it in.
type Cell struct {
	Ch     rune
	Fg, Bg termbox.Attribute
}

// Render is glow's rendering of a file, one row of cells per row it printed.
func Render(path string, width int) ([][]Cell, error) {
	bin := lookPath()
	if bin == "" {
		return nil, errNotAvailable
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, "-s", style, "-w", strconv.Itoa(max(width, 1)), path)

	var out, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &stderr
	if err := cmd.Run(); err != nil {
		return nil, &Error{Reason: reason(stderr.String(), err)}
	}

	return parse(out.Bytes()), nil
}

var errNotAvailable = &Error{Reason: "glow is not installed"}

type Error struct {
	Reason string
}

func (e *Error) Error() string { return "glow: " + e.Reason }

func reason(stderr string, err error) string {
	const maxReason = 90

	first, _, _ := strings.Cut(strings.TrimSpace(stderr), "\n")
	if first == "" {
		first = err.Error()
	}
	if len(first) > maxReason {
		first = first[:maxReason] + "…"
	}

	return first
}

// colorMask is the low bits termbox reads a 256-colour index from; every
// attribute flag (bold, cursive, ...) is a bit above them, so the two can be
// cleared and combined separately.
const colorMask = termbox.Attribute(1<<9 - 1)

// parse turns glow's ANSI into cells: a full SGR token before almost every
// glyph rather than one per run of colour, so this walks token by token
// instead of by line.
func parse(out []byte) [][]Cell {
	var rows [][]Cell
	var cur []Cell
	var fg, bg termbox.Attribute

	s := string(out)
	for i := 0; i < len(s); {
		switch {
		case s[i] == 0x1b && i+1 < len(s) && s[i+1] == '[':
			end := i + 2
			for end < len(s) && s[end] != 'm' {
				end++
			}
			if end >= len(s) {
				i = len(s)
				continue
			}
			applySGR(s[i+2:end], &fg, &bg)
			i = end + 1
		case s[i] == '\n':
			rows = append(rows, cur)
			cur = nil
			i++
		case s[i] == '\r':
			i++
		default:
			ch, size := utf8.DecodeRuneInString(s[i:])
			cur = append(cur, Cell{Ch: ch, Fg: fg, Bg: bg})
			i += size
		}
	}
	if len(cur) > 0 {
		rows = append(rows, cur)
	}

	return rows
}

// applySGR reads one escape's parameters, 38;5;N and 48;5;N included, onto
// the colours a cell is drawn in next.
func applySGR(params string, fg, bg *termbox.Attribute) {
	if params == "" {
		*fg, *bg = 0, 0
		return
	}

	fields := strings.Split(params, ";")
	for i := 0; i < len(fields); i++ {
		n, err := strconv.Atoi(fields[i])
		if err != nil {
			continue
		}

		switch n {
		case 0:
			*fg, *bg = 0, 0
		case 1:
			*fg |= termbox.AttrBold
		case 3:
			*fg |= termbox.AttrCursive
		case 22:
			*fg &^= termbox.AttrBold
		case 23:
			*fg &^= termbox.AttrCursive
		case 39:
			*fg &^= colorMask
		case 49:
			*bg &^= colorMask
		case 38, 48:
			if i+2 >= len(fields) || fields[i+1] != "5" {
				continue
			}
			idx, err := strconv.Atoi(fields[i+2])
			i += 2
			if err != nil {
				continue
			}
			if n == 38 {
				*fg = (*fg &^ colorMask) | termbox.Attribute(idx+1)
			} else {
				*bg = (*bg &^ colorMask) | termbox.Attribute(idx+1)
			}
		}
	}
}
