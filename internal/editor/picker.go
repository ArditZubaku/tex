package editor

import (
	"path/filepath"

	"github.com/ArditZubaku/tex/internal/editor/picker"
	"github.com/ArditZubaku/tex/internal/layout"
	"github.com/ArditZubaku/tex/internal/project"
	"github.com/nsf/termbox-go"
)

var pick picker.Picker

func openPicker() {
	root := project.Root(sourceFile)
	files, err := project.List(root)
	if err != nil {
		statusMsg = "E484: Can't open file " + root
		return
	}

	entries := make([]picker.Entry, 0, len(files))
	for _, rel := range files {
		entries = append(entries, picker.Entry{Label: rel, Path: filepath.Join(root, rel), Row: -1})
	}

	showPicker(filepath.Base(root), entries)
}

func showPicker(title string, entries []picker.Entry) {
	mode = PickerMode
	pick.Show(title, entries)
}

func closePicker() {
	mode = ReadMode
	pick.Close()
	clampCol()
}

func openPicked() {
	entry, ok := pick.Selected()
	if !ok {
		return
	}

	closePicker()
	pushJump()
	if !project.Same(entry.Path, sourceFile) {
		openInBuffer(entry.Path)
	}
	if entry.Row >= 0 {
		currentRow, currentCol = min(entry.Row, buf.LineCount()-1), entry.Col
		clampCol()
		centerIfOffScreen()
	}
}

func handlePickerKey(event termbox.Event) {
	switch pick.Key(event) {
	case picker.Closed:
		closePicker()
	case picker.Chosen:
		openPicked()
	}
}

func displayPicker() { pick.Draw(screenArea(), &active) }

// screenArea is everything below the buffer line: what the popup centres in,
// which is the screen rather than the window being worked in.
func screenArea() layout.Rect {
	return layout.Rect{Row: tabBarRows, Rows: screenRows, Cols: screenCols}
}
