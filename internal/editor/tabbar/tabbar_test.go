package tabbar

import (
	"strings"
	"testing"

	"github.com/ArditZubaku/tex/internal/theme"
)

func text(cells []cell) string {
	out := make([]rune, 0, len(cells))
	for _, one := range cells {
		if one.ch != 0 {
			out = append(out, one.ch)
		}
	}

	return string(out)
}

func TestTheCurrentTabAndTheUnsavedMarkGetTheirOwnColours(t *testing.T) {
	palette := theme.Default()
	cells, from, to := lay([]Tab{{Name: "f.txt", Modified: true}, {Name: "a.txt"}}, 1, &palette)

	if got := text(cells); got != " f.txt ●  a.txt " {
		t.Errorf("buffer line = %q", got)
	}
	if got := text(cells[from:to]); got != " a.txt " {
		t.Errorf("current tab = %q, want %q", got, " a.txt ")
	}

	for i, one := range cells {
		want := palette.TabFg
		switch {
		case i >= from && i < to:
			want = palette.TabActiveFg
		case one.ch == ModifiedMark:
			want = palette.TabModified
		}
		if one.fg != want {
			t.Errorf("cell %d (%q) fg = %v, want %v", i, one.ch, one.fg, want)
		}
	}
}

func TestScrollingBringsTheCurrentTabIntoView(t *testing.T) {
	palette := theme.Default()
	open := make([]Tab, 0, 20)
	for i := range 20 {
		open = append(open, Tab{Name: string(rune('a'+i)) + ".txt"})
	}

	const width = 40
	cells, from, to := lay(open, len(open)-1, &palette)
	if len(cells) <= width {
		t.Fatalf("buffer line is %d cells wide, want wider than the window", len(cells))
	}

	var bar Bar
	bar.scroll(len(cells), from, to, width)

	if from < bar.offset || to > bar.offset+width {
		t.Errorf("current tab at [%d,%d) is outside the window at %d", from, to, bar.offset)
	}
}

func TestAWideTabIsNotCutInHalf(t *testing.T) {
	palette := theme.Default()
	cells, _, _ := lay([]Tab{{Name: "日.txt"}}, 0, &palette)

	if got := text(cells); !strings.Contains(got, "日") {
		t.Errorf("buffer line = %q, want the wide name in it", got)
	}
	if len(cells) != len([]rune(" 日.txt "))+1 {
		t.Errorf("%d cells, want one more than the runes for the wide one", len(cells))
	}
}
