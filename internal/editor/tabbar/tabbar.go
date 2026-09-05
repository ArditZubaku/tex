// Package tabbar is the buffer line: the open buffers along one row, with the
// one being edited marked and kept on screen.
package tabbar

import (
	"github.com/mattn/go-runewidth"
	"github.com/nsf/termbox-go"

	"github.com/ArditZubaku/tex/internal/theme"
)

// ModifiedMark is the dot an unsaved buffer carries, on the buffer line and in
// ':ls' alike.
const ModifiedMark = '●'

type Tab struct {
	Name     string
	Modified bool
}

// A cell of the buffer line, laid out before anything is drawn so that
// scrolling the row to keep the current buffer on screen is one window over it.
// A zero rune is the second half of a wide one, which termbox draws itself.
type cell struct {
	ch     rune
	fg, bg termbox.Attribute
}

// A Bar keeps how far the row is scrolled, which is what a buffer opened off
// its right-hand edge moves.
type Bar struct {
	offset int
}

func (b *Bar) Draw(row, width int, tabs []Tab, current int, palette *theme.Palette) {
	cells, from, to := lay(tabs, current, palette)
	b.scroll(len(cells), from, to, width)

	for col := range width {
		ch, fg, bg := ' ', palette.TabFg, palette.TabBarBg
		if i := col + b.offset; i >= 0 && i < len(cells) {
			ch, fg, bg = cells[i].ch, cells[i].fg, cells[i].bg
		}
		if ch == 0 {
			continue
		}
		termbox.SetCell(col, row, ch, fg, bg)
	}
}

func lay(tabs []Tab, current int, palette *theme.Palette) (cells []cell, activeFrom, activeTo int) {
	for i, tab := range tabs {
		fg, bg := palette.TabFg, palette.TabBarBg
		if i == current {
			fg, bg, activeFrom = palette.TabActiveFg, palette.TabActiveBg, len(cells)
		}

		cells = appendTab(cells, tab, fg, bg, palette)
		if i == current {
			activeTo = len(cells)
		}
	}

	return cells, activeFrom, activeTo
}

func appendTab(cells []cell, tab Tab, fg, bg termbox.Attribute, palette *theme.Palette) []cell {
	label := " " + tab.Name + " "
	if tab.Modified {
		label += string(ModifiedMark) + " "
	}

	for _, ch := range label {
		markFg := fg
		if ch == ModifiedMark {
			markFg = palette.TabModified
		}
		cells = append(cells, cell{ch: ch, fg: markFg, bg: bg})
		for range runewidth.RuneWidth(ch) - 1 {
			cells = append(cells, cell{bg: bg})
		}
	}

	return cells
}

func (b *Bar) scroll(laid, from, to, width int) {
	if to-from >= width {
		b.offset = from
		return
	}

	if from < b.offset {
		b.offset = from
	}
	if to > b.offset+width {
		b.offset = to - width
	}
	b.offset = max(min(b.offset, laid-width), 0)
}
