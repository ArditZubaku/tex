package main

import (
	"cmp"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"

	"github.com/mattn/go-runewidth"
	"github.com/nsf/termbox-go"
)

// The picker is LazyVim's '<leader><leader>': a popup over whatever is on
// screen, listing the files under the project root and narrowing them to what
// is typed, with Enter opening the one settled on as a buffer.
var (
	pickerOpen    bool
	pickerRoot    string
	pickerFiles   []string
	pickerMatches []pickerMatch
	pickerQuery   []rune
	pickerSel     int
	pickerOffset  int
)

type pickerMatch struct {
	path  string
	score int
}

// A walk of a large tree costs more than a picker is worth, so it stops once it
// has more files than anybody scrolls through and lets the filter do the rest.
const maxPickerFiles = 20000

// The popup is drawn at these at most, and shrinks to whatever the screen has.
const (
	pickerCols = 80
	pickerRows = 15
)

func openPicker() {
	root := projectRoot()
	files, err := listFiles(root)
	if err != nil {
		statusMsg = "E484: Can't open file " + root
		return
	}

	pickerOpen, mode = true, PickerMode
	pickerRoot, pickerFiles = root, files
	pickerQuery = pickerQuery[:0]
	filterPicker()
}

func closePicker() {
	pickerOpen, mode = false, ReadMode
	pickerFiles, pickerMatches = nil, nil
	clampCol()
}

// projectRoot is the directory the files are listed from: the repository the
// file being edited sits in, or its own directory when it is in none.
func projectRoot() string {
	dir, err := filepath.Abs(filepath.Dir(sourceFile))
	if err != nil {
		return "."
	}

	for at := dir; ; {
		if info, err := os.Stat(filepath.Join(at, ".git")); err == nil && info.IsDir() {
			return at
		}
		parent := filepath.Dir(at)
		if parent == at {
			return dir
		}
		at = parent
	}
}

// listFiles walks the tree below root, leaving out what is hidden: a picker is
// for the files being worked on, and '.git' alone holds more than all of them.
func listFiles(root string) ([]string, error) {
	files := make([]string, 0, 256)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if name := entry.Name(); strings.HasPrefix(name, ".") && path != root {
			if entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		if len(files) >= maxPickerFiles {
			return fs.SkipAll
		}

		if rel, err := filepath.Rel(root, path); err == nil {
			files = append(files, rel)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return files, nil
}

func filterPicker() {
	pickerMatches = pickerMatches[:0]
	for _, path := range pickerFiles {
		if score, ok := fuzzyScore(path, pickerQuery); ok {
			pickerMatches = append(pickerMatches, pickerMatch{path: path, score: score})
		}
	}

	slices.SortStableFunc(pickerMatches, func(a, b pickerMatch) int {
		if a.score != b.score {
			return b.score - a.score
		}

		return cmp.Compare(a.path, b.path)
	})
	pickerSel, pickerOffset = 0, 0
}

// fuzzyScore matches the query as a subsequence of the path and scores what it
// found the way a file picker is usually meant: the letters together, at the
// start of a word, and in the name rather than the directories leading to it.
func fuzzyScore(path string, query []rune) (int, bool) {
	if len(query) == 0 {
		return 0, true
	}

	text := []rune(strings.ToLower(path))
	nameAt := 0
	for i, ch := range text {
		if ch == filepath.Separator {
			nameAt = i + 1
		}
	}

	score, at, run := 0, 0, 0
	for _, want := range query {
		want = unicode.ToLower(want)

		i := at
		for i < len(text) && text[i] != want {
			i++
		}
		if i == len(text) {
			return 0, false
		}

		run = 0
		if i == at && at > 0 {
			run = 4
		}
		score += 1 + run
		if i >= nameAt {
			score += 3
		}
		if i == nameAt || (i > 0 && classOf(text[i-1]) != ClassWord) {
			score += 5
		}
		at = i + 1
	}

	// a tie between two paths goes to the shorter, which is the one with less
	// around what was typed
	return score - len(text)/8, true
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

	path := filepath.Join(pickerRoot, pickerMatches[pickerSel].path)
	closePicker()
	openInBuffer(path)
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

	printMessage(col+1, row+1, active.plain, active.background,
		padTo(" > "+string(pickerQuery), cols-2))

	for i := range listRows {
		at := pickerOffset + i
		if at >= len(pickerMatches) {
			break
		}

		foreground, background := active.plain, active.background
		if at == pickerSel {
			foreground, background = active.tabActiveFg, active.tabActiveBg
		}
		printMessage(col+1, row+3+i, foreground, background,
			padTo(" "+truncate(pickerMatches[at].path, cols-3), cols-2))
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
	title := " Files "
	if pickerRoot != "" {
		title = " " + filepath.Base(pickerRoot) + " "
	}

	top := []rune("┌" + strings.Repeat("─", cols-2) + "┐")
	copy(top[2:], []rune(title))
	count := fmt.Sprintf(" %d/%d ", len(pickerMatches), len(pickerFiles))
	copy(top[cols-2-len([]rune(count)):], []rune(count))

	printMessage(col, row, active.separator, active.background, string(top))
	printMessage(col, row+2, active.separator, active.background,
		"├"+strings.Repeat("─", cols-2)+"┤")
	printMessage(col, row+rows-1, active.separator, active.background,
		"└"+strings.Repeat("─", cols-2)+"┘")

	for i := 1; i < rows-1; i++ {
		if i == 2 {
			continue
		}
		printMessage(col, row+i, active.separator, active.background, "│")
		printMessage(col+1, row+i, active.plain, active.background, strings.Repeat(" ", cols-2))
		printMessage(col+cols-1, row+i, active.separator, active.background, "│")
	}
}

func truncate(txt string, width int) string {
	runes := []rune(txt)
	if len(runes) <= width {
		return txt
	}

	return "…" + string(runes[len(runes)-width+1:])
}
