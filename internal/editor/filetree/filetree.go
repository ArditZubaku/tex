// Package filetree is netrw as this editor needs it: one directory drawn over
// the whole window, with the buffer left where it is until a file is chosen.
package filetree

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ArditZubaku/tex/internal/editor/screen"
	"github.com/ArditZubaku/tex/internal/layout"
	"github.com/ArditZubaku/tex/internal/theme"
)

type Entry struct {
	Name  string
	IsDir bool
}

// ParentDir is the entry every listing but the root's opens with.
const ParentDir = ".."

// HeaderRows is the first row, which the directory being listed takes, so that
// it is on screen however far down the entries the selection has scrolled.
const HeaderRows = 1

// all is everything the directory holds; entries is what the filter has left of
// it, which is what the selection indexes and the screen shows.
type Tree struct {
	dir     string
	all     []Entry
	entries []Entry
	filter  string
	sel     int
	offset  int
	hidden  bool
}

func (e *Tree) Dir() string      { return e.dir }
func (e *Tree) Filtered() string { return e.filter }

func (e *Tree) Path(name string) string { return filepath.Join(e.dir, name) }

// Go is Enter for the moves that leave a directory behind, which drop the
// filter with it; rereading the same directory keeps it.
func (e *Tree) Go(dir, on string) error {
	e.filter = ""

	return e.Enter(dir, on)
}

// Enter lists dir and puts the selection on the entry named by on, so that
// stepping out of a directory leaves the cursor on the one just left.
func (e *Tree) Enter(dir, on string) error {
	entries, err := e.readDir(dir)
	if err != nil {
		return err
	}

	e.dir, e.all = dir, entries
	e.selectEntry(on)

	return nil
}

func (e *Tree) Leave() error {
	parent := filepath.Dir(e.dir)
	if parent == e.dir {
		return nil
	}

	return e.Go(parent, filepath.Base(e.dir))
}

func (e *Tree) Filter(text string) {
	on := e.SelectedName()
	e.filter = text
	e.selectEntry(on)
}

func (e *Tree) ToggleHidden() error {
	e.hidden = !e.hidden

	return e.Enter(e.dir, e.SelectedName())
}

// selectEntry re-runs the filter over the directory and puts the selection back
// on the entry named, or on the first one when it has been filtered away.
func (e *Tree) selectEntry(name string) {
	e.entries = matching(e.all, e.filter)
	e.sel, e.offset = 0, 0
	if i := slices.IndexFunc(e.entries, func(one Entry) bool { return one.Name == name }); i >= 0 {
		e.sel = i
	}
}

func matching(entries []Entry, filter string) []Entry {
	if filter == "" {
		return entries
	}

	needle := strings.ToLower(filter)
	out := make([]Entry, 0, len(entries))
	for _, one := range entries {
		if strings.Contains(strings.ToLower(one.Name), needle) {
			out = append(out, one)
		}
	}

	return out
}

// Entries is what the filter has left, Selection which of them is on, and
// Offset the first one on screen.
func (e *Tree) Entries() []Entry { return e.entries }
func (e *Tree) Selection() int   { return e.sel }
func (e *Tree) Offset() int      { return e.offset }

func (e *Tree) Selected() (Entry, bool) {
	if e.sel >= len(e.entries) {
		return Entry{}, false
	}

	return e.entries[e.sel], true
}

func (e *Tree) SelectedName() string {
	entry, ok := e.Selected()
	if !ok {
		return ""
	}

	return entry.Name
}

func (e *Tree) Move(delta int) {
	e.sel = max(min(e.sel+delta, len(e.entries)-1), 0)
}

func (e *Tree) Top()    { e.sel = 0 }
func (e *Tree) Bottom() { e.sel = max(len(e.entries)-1, 0) }

func (e *Tree) readDir(dir string) ([]Entry, error) {
	listing, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	entries := make([]Entry, 0, len(listing)+1)
	if parent := filepath.Dir(dir); parent != dir {
		entries = append(entries, Entry{Name: ParentDir, IsDir: true})
	}

	files := make([]Entry, 0, len(listing))
	for _, one := range listing {
		if !e.hidden && strings.HasPrefix(one.Name(), ".") {
			continue
		}
		files = append(files, Entry{Name: one.Name(), IsDir: one.IsDir()})
	}

	// os.ReadDir sorts by name, so sorting directories to the front stably
	// keeps that order inside each of the two groups
	slices.SortStableFunc(files, func(a, b Entry) int {
		switch {
		case a.IsDir == b.IsDir:
			return 0
		case a.IsDir:
			return -1
		default:
			return 1
		}
	})

	return append(entries, files...), nil
}

func (e *Tree) Draw(within layout.Rect, palette *theme.Palette) {
	e.scroll(within.Rows)
	screen.Print(within.Col, within.Row, palette.CursorLineNumber, palette.Background,
		screen.Pad(e.dir, within.Cols))

	for row := HeaderRows; row < within.Rows; row++ {
		at := row - HeaderRows + e.offset
		if at >= len(e.entries) {
			break
		}

		entry := e.entries[at]
		foreground, background := palette.Plain, palette.Background
		if entry.IsDir {
			foreground = palette.Function
		}
		if at == e.sel {
			background = palette.CursorLineBg
			screen.Fill(within.Col, within.Row+row, within.Cols, palette.Plain, palette.CursorLineBg)
		}

		screen.Print(within.Col, within.Row+row, foreground, background,
			screen.Pad(" "+Label(entry), within.Cols))
	}
}

func Label(e Entry) string {
	if e.IsDir {
		return e.Name + "/"
	}

	return e.Name
}

func (e *Tree) scroll(rows int) {
	rows -= HeaderRows
	if rows < 1 {
		return
	}

	if e.sel < e.offset {
		e.offset = e.sel
	}
	if e.sel >= e.offset+rows {
		e.offset = e.sel - rows + 1
	}
}

func (e *Tree) CursorRow(within layout.Rect) int {
	return within.Row + e.sel - e.offset + HeaderRows
}

func (e *Tree) Status() string {
	if e.filter != "" {
		return fmt.Sprintf(" EXPLORE: %s - %d/%d entries matching %q",
			filepath.Base(e.dir), len(e.entries), len(e.all), e.filter)
	}

	return fmt.Sprintf(" EXPLORE: %s - %d entries", filepath.Base(e.dir), len(e.all))
}
