package edit_test

import (
	"testing"

	"github.com/ArditZubaku/tex/internal/editor/edit"
	"github.com/ArditZubaku/tex/internal/editor/edtest"
	"github.com/ArditZubaku/tex/internal/editor/state"
)

func TestInsertTextSplicesASingleLineIntoTheCursor(t *testing.T) {
	e := state.New()

	b := edtest.InReadMode(t, e, "ac\n", 0, 1)
	e.Mode = state.EditMode

	edit.InsertText(e, "b")

	edtest.WantLines(t, b, "abc")
	edtest.WantCursor(t, e, 0, 2)
}

// The whole point: a multi-line, already-indented, already-bracketed paste
// lands exactly as it came in rather than compounding indent or brackets
// meant for a human typing it one key at a time.
func TestInsertTextLandsAMultiLinePasteVerbatim(t *testing.T) {
	e := state.New()

	b := edtest.InReadMode(t, e, "\n", 0, 0)
	e.Mode = state.EditMode

	edit.InsertText(e, "fn main() {\n    let city = String::new();\n}")

	edtest.WantLines(t, b, "fn main() {", "    let city = String::new();", "}")
	edtest.WantCursor(t, e, 2, 1)
}

func TestInsertTextSplitsTheLineItLandsOn(t *testing.T) {
	e := state.New()

	b := edtest.InReadMode(t, e, "ad\n", 0, 1)
	e.Mode = state.EditMode

	edit.InsertText(e, "b\nc")

	edtest.WantLines(t, b, "ab", "cd")
	edtest.WantCursor(t, e, 1, 1)
}

func TestInsertTextWithNoTextDoesNothing(t *testing.T) {
	e := state.New()

	b := edtest.InReadMode(t, e, "abc\n", 0, 1)
	e.Mode = state.EditMode

	edit.InsertText(e, "")

	edtest.WantLines(t, b, "abc")
	if e.Modified {
		t.Error("modified with nothing pasted")
	}
}
