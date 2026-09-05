package find

import (
	"path/filepath"

	"github.com/nsf/termbox-go"

	"github.com/ArditZubaku/tex/internal/editor/picker"
	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/ArditZubaku/tex/internal/editor/view"
	"github.com/ArditZubaku/tex/internal/project"
)

func OpenFiles(e *state.Editor) {
	root := project.Root(e.SourceFile)
	files, err := project.List(root)
	if err != nil {
		e.StatusMsg = "E484: Can't open file " + root
		return
	}

	entries := make([]picker.Entry, 0, len(files))
	for _, rel := range files {
		entries = append(entries, picker.Entry{Label: rel, Path: filepath.Join(root, rel), Row: -1})
	}

	showPicker(e, filepath.Base(root), entries)
}

func showPicker(e *state.Editor, title string, entries []picker.Entry) {
	e.Mode = state.PickerMode
	e.Pick.Show(title, entries)
}

func closePicker(e *state.Editor) {
	e.Mode = state.ReadMode
	e.Pick.Close()
	e.ClampCol()
}

func openPicked(e *state.Editor) {
	entry, ok := e.Pick.Selected()
	if !ok {
		return
	}

	closePicker(e)
	e.PushJump()
	if !project.Same(entry.Path, e.SourceFile) {
		view.Open(e, entry.Path)
	}
	if entry.Row >= 0 {
		e.Row, e.Col = min(entry.Row, e.Buf.LineCount()-1), entry.Col
		e.ClampCol()
		e.CenterIfOffScreen()
	}
}

func PickerKey(e *state.Editor, event termbox.Event) {
	switch e.Pick.Key(event) {
	case picker.Closed:
		closePicker(e)
	case picker.Chosen:
		openPicked(e)
	}
}

func DrawPicker(e *state.Editor) { e.Pick.Draw(e.ScreenArea(), &e.Palette) }
