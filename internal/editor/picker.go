package editor

import (
	"cmp"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ArditZubaku/tex/internal/editor/screen"
	"github.com/ArditZubaku/tex/internal/fuzzy"
	"github.com/ArditZubaku/tex/internal/project"
	"github.com/mattn/go-runewidth"
	"github.com/nsf/termbox-go"
)

// The picker is a popup over whatever is on screen: a list of places to go,
// narrowed to what is typed, with Enter going to the one settled on. The files
// under the project root are one such list ('<leader><leader>'), the references
// to an identifier another ('gr').
var (
	pickerOpen    bool
	pickerTitle   string
	pickerEntries []pickerEntry
	pickerMatches []pickerMatch
	pickerQuery   []rune
	pickerSel     int
	pickerOffset  int
)

// A row of the popup: what is drawn and matched, and where Enter goes. A
// negative row means the file itself, wherever the cursor was last left in it.
type pickerEntry struct {
	label string
	path  string
	row   int
	col   int
}

type pickerMatch struct {
	at    int
	score int
}

// The popup is drawn at these at most, and shrinks to whatever the screen has.
const (
	pickerCols = 80
	pickerRows = 15
)

func openPicker() {
	root := project.Root(sourceFile)
	files, err := project.List(root)
	if err != nil {
		statusMsg = "E484: Can't open file " + root
		return
	}

	entries := make([]pickerEntry, 0, len(files))
	for _, rel := range files {
		entries = append(entries, pickerEntry{label: rel, path: filepath.Join(root, rel), row: -1})
	}

	showPicker(filepath.Base(root), entries)
}

func showPicker(title string, entries []pickerEntry) {
	pickerOpen, mode = true, PickerMode
	pickerTitle, pickerEntries = title, entries
	pickerQuery = pickerQuery[:0]
	filterPicker()
}

func closePicker() {
	pickerOpen, mode = false, ReadMode
	pickerEntries, pickerMatches = nil, nil
	clampCol()
}

func filterPicker() {
	pickerMatches = pickerMatches[:0]
	for at, entry := range pickerEntries {
		if score, ok := fuzzy.Score(entry.label, pickerQuery); ok {
			pickerMatches = append(pickerMatches, pickerMatch{at: at, score: score})
		}
	}

	// a list already in the order it means something in — the references down a
	// file — keeps it until something is typed
	if len(pickerQuery) > 0 {
		slices.SortStableFunc(pickerMatches, func(a, b pickerMatch) int {
			if a.score != b.score {
				return b.score - a.score
			}

			return cmp.Compare(pickerEntries[a.at].label, pickerEntries[b.at].label)
		})
	}
	pickerSel, pickerOffset = 0, 0
}

func pickerDown() { pickerMove(1) }
func pickerUp()   { pickerMove(-1) }

func pickerMove(delta int) {
	pickerSel = max(min(pickerSel+delta, len(pickerMatches)-1), 0)
}

func openPicked() {
	if len(pickerMatches) == 0 {
		return
	}

	entry := pickerEntries[pickerMatches[pickerSel].at]
	closePicker()
	pushJump()
	if !project.Same(entry.path, sourceFile) {
		openInBuffer(entry.path)
	}
	if entry.row >= 0 {
		currentRow, currentCol = min(entry.row, buf.LineCount()-1), entry.col
		clampCol()
		centerIfOffScreen()
	}
}

var pickerKeys = map[termbox.Key]func(){
	termbox.KeyEsc:       closePicker,
	termbox.KeyEnter:     openPicked,
	termbox.KeyArrowDown: pickerDown,
	termbox.KeyArrowUp:   pickerUp,
	termbox.KeyCtrlN:     pickerDown,
	termbox.KeyCtrlP:     pickerUp,
	termbox.KeyCtrlJ:     pickerDown,
	termbox.KeyCtrlK:     pickerUp,
}

func handlePickerKey(event termbox.Event) {
	if action, ok := pickerKeys[event.Key]; ok {
		action()
		return
	}

	switch {
	case event.Key == termbox.KeyBackspace, event.Key == termbox.KeyBackspace2:
		if len(pickerQuery) == 0 {
			closePicker()
			return
		}
		pickerQuery = pickerQuery[:len(pickerQuery)-1]
	case event.Key == termbox.KeyCtrlU:
		pickerQuery = pickerQuery[:0]
	case event.Key == termbox.KeySpace:
		pickerQuery = append(pickerQuery, ' ')
	case event.Ch != 0:
		pickerQuery = append(pickerQuery, event.Ch)
	default:
		return
	}

	filterPicker()
}

// The popup is drawn over the windows rather than inside one: it belongs to the
// screen the way the status line does, so it stays where it is however the
// screen is split.
func displayPicker() {
	if !pickerOpen {
		return
	}

	cols, rows := pickerSize()
	row, col := pickerRow(), pickerCol()
	listRows := rows - pickerChromeRows

	scrollPicker(listRows)
	drawPickerFrame(row, col, rows, cols)

	screen.Print(col+1, row+1, active.Plain, active.Background,
		screen.Pad(" > "+string(pickerQuery), cols-2))

	for i := range listRows {
		at := pickerOffset + i
		if at >= len(pickerMatches) {
			break
		}

		foreground, background := active.Plain, active.Background
		if at == pickerSel {
			foreground, background = active.TabActiveFg, active.TabActiveBg
		}
		entry := pickerEntries[pickerMatches[at].at]
		screen.Print(col+1, row+3+i, foreground, background,
			screen.Pad(" "+screen.Truncate(entry.label, cols-3, entry.row < 0), cols-2))
	}
}

// The frame takes two rows for its own edges, one for the query and one for the
// rule under it, which is what the listing has to fit around.
const pickerChromeRows = 4

func pickerSize() (cols, rows int) {
	return min(pickerCols, screenCols-4), min(pickerRows, screenRows-2)
}

func pickerRow() int {
	_, rows := pickerSize()

	return tabBarRows + (screenRows-rows)/2
}

func pickerCol() int {
	cols, _ := pickerSize()

	return (screenCols - cols) / 2
}

func pickerCursorCol() int {
	return pickerCol() + 4 + runewidth.StringWidth(string(pickerQuery))
}

func scrollPicker(listRows int) {
	pickerSel = min(pickerSel, max(len(pickerMatches)-1, 0))
	pickerOffset = min(pickerOffset, pickerSel)
	if pickerSel >= pickerOffset+listRows {
		pickerOffset = pickerSel - listRows + 1
	}
}

func drawPickerFrame(row, col, rows, cols int) {
	title := " " + cmp.Or(pickerTitle, "Files") + " "

	top := []rune("┌" + strings.Repeat("─", cols-2) + "┐")
	copy(top[2:], []rune(title))
	count := fmt.Sprintf(" %d/%d ", len(pickerMatches), len(pickerEntries))
	copy(top[cols-2-len([]rune(count)):], []rune(count))

	screen.Print(col, row, active.Separator, active.Background, string(top))
	screen.Print(col, row+2, active.Separator, active.Background,
		"├"+strings.Repeat("─", cols-2)+"┤")
	screen.Print(col, row+rows-1, active.Separator, active.Background,
		"└"+strings.Repeat("─", cols-2)+"┘")

	for i := 1; i < rows-1; i++ {
		if i == 2 {
			continue
		}
		screen.Print(col, row+i, active.Separator, active.Background, "│")
		screen.Print(col+1, row+i, active.Plain, active.Background, strings.Repeat(" ", cols-2))
		screen.Print(col+cols-1, row+i, active.Separator, active.Background, "│")
	}
}
