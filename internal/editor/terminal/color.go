package terminal

import (
	"github.com/hinshun/vt10x"
	"github.com/nsf/termbox-go"
)

// A Cell is one glyph of the terminal's own screen, at the colour the shell
// drew it in.
type Cell struct {
	Ch     rune
	Fg, Bg termbox.Attribute
}

// vt10x's Mode bits are unexported, so the layout is hardcoded here rather
// than read off a name; attrReverse is deliberately left out of it, since
// vt10x already resolves reverse video into the stored FG/BG themselves at
// the moment a glyph is written (state.go's setChar) — translating the bit
// again here would invert an already-inverted cell.
const (
	attrUnderline = 1 << 1
	attrBold      = 1 << 2
	attrItalic    = 1 << 4
	attrBlink     = 1 << 5
)

func cellOf(g vt10x.Glyph) Cell {
	return Cell{Ch: g.Char, Fg: attrOf(g.Mode) | colorOf(g.FG), Bg: colorOf(g.BG)}
}

func attrOf(mode int16) termbox.Attribute {
	var attr termbox.Attribute
	if mode&attrBold != 0 {
		attr |= termbox.AttrBold
	}
	if mode&attrUnderline != 0 {
		attr |= termbox.AttrUnderline
	}
	if mode&attrItalic != 0 {
		attr |= termbox.AttrCursive
	}
	if mode&attrBlink != 0 {
		attr |= termbox.AttrBlink
	}

	return attr
}

// colorOf is a vt10x colour as termbox's own Output256 attribute: 0 for
// default, or a 256-colour index one above its value so it never collides
// with 0. A truecolour value (set by a 38;2;r;g;b escape, which vt10x parses
// and packs into the same uint32) has no 256-colour equivalent of its own, so
// it is quantized to the nearest one on xterm's cube-plus-greyscale ramp.
func colorOf(c vt10x.Color) termbox.Attribute {
	switch {
	case c >= vt10x.DefaultFG:
		return 0
	case c < 256:
		return termbox.Attribute(c + 1)
	default:
		return termbox.Attribute(nearest256(byte(c>>16), byte(c>>8), byte(c)) + 1)
	}
}

var cubeLevels = [6]int{0, 95, 135, 175, 215, 255}

func nearest256(r, g, b byte) int {
	cr, cg, cb := nearestLevel(r), nearestLevel(g), nearestLevel(b)
	cube := 16 + 36*cr + 6*cg + cb
	cubeErr := errSq(r, g, b, cubeLevels[cr], cubeLevels[cg], cubeLevels[cb])

	gray := nearestGray(r, g, b)
	grayLevel := 8 + 10*(gray-232)
	grayErr := errSq(r, g, b, grayLevel, grayLevel, grayLevel)

	if grayErr < cubeErr {
		return gray
	}

	return cube
}

func nearestLevel(v byte) int {
	best, bestDiff := 0, 256
	for i, level := range cubeLevels {
		if diff := abs(int(v) - level); diff < bestDiff {
			best, bestDiff = i, diff
		}
	}

	return best
}

func nearestGray(r, g, b byte) int {
	avg := (int(r) + int(g) + int(b)) / 3

	return 232 + max(0, min(23, (avg-8)/10))
}

func errSq(r, g, b byte, cr, cg, cb int) int {
	dr, dg, db := int(r)-cr, int(g)-cg, int(b)-cb

	return dr*dr + dg*dg + db*db
}

func abs(n int) int {
	if n < 0 {
		return -n
	}

	return n
}
