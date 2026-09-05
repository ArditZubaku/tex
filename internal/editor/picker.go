package editor

import (
	"path/filepath"

	"github.com/ArditZubaku/tex/internal/editor/picker"
	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/ArditZubaku/tex/internal/editor/view"
	"github.com/ArditZubaku/tex/internal/project"
	"github.com/nsf/termbox-go"
)

func openPicker() {
	root := project.Root(ed.SourceFile)
	files, err := project.List(root)
	if err != nil {
		ed.StatusMsg = "E484: Can't open file " + root
		return
	}

	entries := make([]picker.Entry, 0, len(files))
	for _, rel := range files {
		entries = append(entries, picker.Entry{Label: rel, Path: filepath.Join(root, rel), Row: -1})
	}

	showPicker(filepath.Base(root), entries)
}

func showPicker(title string, entries []picker.Entry) {
	ed.Mode = state.PickerMode
	ed.Pick.Show(title, entries)
}

func closePicker() {
	ed.Mode = state.ReadMode
	ed.Pick.Close()
	ed.ClampCol()
}

func openPicked() {
	entry, ok := ed.Pick.Selected()
	if !ok {
		return
	}

	closePicker()
	ed.PushJump()
	if !project.Same(entry.Path, ed.SourceFile) {
		view.Open(ed, entry.Path)
	}
	if entry.Row >= 0 {
		ed.Row, ed.Col = min(entry.Row, ed.Buf.LineCount()-1), entry.Col
		ed.ClampCol()
		ed.CenterIfOffScreen()
	}
}

func handlePickerKey(event termbox.Event) {
	switch ed.Pick.Key(event) {
	case picker.Closed:
		closePicker()
	case picker.Chosen:
		openPicked()
	}
}

func displayPicker() { ed.Pick.Draw(ed.ScreenArea(), &ed.Palette) }
