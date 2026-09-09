package edit_test

import (
	"testing"

	"github.com/ArditZubaku/tex/internal/editor/edit"
	"github.com/ArditZubaku/tex/internal/editor/edtest"
	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/ArditZubaku/tex/internal/syntax"
)

func TestToggleCommentLineRoundTrips(t *testing.T) {
	e := state.New()

	b := edtest.AtCursor(t, e, "foo\n", 0, 0)
	e.Lang = syntax.Detect("x.go")

	edit.ToggleCommentLine(e)
	edtest.WantLines(t, b, "// foo")

	edit.ToggleCommentLine(e)
	edtest.WantLines(t, b, "foo")
}

func TestToggleCommentLinePreservesIndentation(t *testing.T) {
	e := state.New()

	b := edtest.AtCursor(t, e, "  foo\n", 0, 0)
	e.Lang = syntax.Detect("x.go")

	edit.ToggleCommentLine(e)
	edtest.WantLines(t, b, "  // foo")

	edit.ToggleCommentLine(e)
	edtest.WantLines(t, b, "  foo")
}

// A '#' language's own leader is used instead, and a lone '#' with nothing
// after it — no trailing space to have added — still comes off cleanly.
func TestToggleCommentLineUsesTheLanguagesOwnLeader(t *testing.T) {
	e := state.New()

	b := edtest.AtCursor(t, e, "#\n", 0, 0)
	e.Lang = syntax.Detect("x.py")

	edit.ToggleCommentLine(e)
	edtest.WantLines(t, b, "")
}

func TestToggleCommentLineWithNoKnownSyntaxIsANoOp(t *testing.T) {
	e := state.New()

	b := edtest.AtCursor(t, e, "foo\n", 0, 0)

	edit.ToggleCommentLine(e)
	edtest.WantLines(t, b, "foo")
	if e.Modified {
		t.Error("modified = true for a language with no comment syntax")
	}
}

func TestToggleCommentLinesSkipsBlankLinesEitherWay(t *testing.T) {
	e := state.New()

	b := edtest.AtCursor(t, e, "a\n\nb\n", 0, 0)
	e.Lang = syntax.Detect("x.go")

	edit.ToggleCommentLines(e, 0, 3)
	edtest.WantLines(t, b, "// a", "", "// b")

	edit.ToggleCommentLines(e, 0, 3)
	edtest.WantLines(t, b, "a", "", "b")
}

// A block that is nothing but blank lines has nothing to comment out, so
// toggling it leaves every line exactly as it was.
func TestToggleCommentLinesOnAllBlankLinesIsANoOp(t *testing.T) {
	e := state.New()

	b := edtest.AtCursor(t, e, "\n\n", 0, 0)
	e.Lang = syntax.Detect("x.go")

	edit.ToggleCommentLines(e, 0, 2)
	edtest.WantLines(t, b, "", "")
}

// A block that is only partly commented reads as needing comments, and the
// line that already has one is left alone rather than commented twice.
func TestToggleCommentLinesOnAMixedBlockCommentsWhatIsMissing(t *testing.T) {
	e := state.New()

	b := edtest.AtCursor(t, e, "// a\nb\n", 0, 0)
	e.Lang = syntax.Detect("x.go")

	edit.ToggleCommentLines(e, 0, 2)
	edtest.WantLines(t, b, "// a", "// b")
}

func TestCountedToggleCommentCoversTheLinesBelow(t *testing.T) {
	e := state.New()

	b := edtest.InReadMode(t, e, "a\nb\nc\n", 0, 0)
	e.Lang = syntax.Detect("x.go")

	edtest.Press(t, e, "3gcc")
	edtest.WantLines(t, b, "// a", "// b", "// c")
	if e.Row != 0 {
		t.Errorf("currentRow = %d, want 0", e.Row)
	}

	edtest.Press(t, e, "3gcc")
	edtest.WantLines(t, b, "a", "b", "c")
}

func TestToggleCommentUndoesInOneStep(t *testing.T) {
	e := state.New()

	b := edtest.InReadMode(t, e, "a\nb\n", 0, 0)
	e.Lang = syntax.Detect("x.go")

	edtest.Press(t, e, "2gcc")
	edtest.WantLines(t, b, "// a", "// b")

	edtest.Press(t, e, "u")
	edtest.WantLines(t, b, "a", "b")
}

func TestVisualToggleCommentCoversEveryLineTheSelectionTouches(t *testing.T) {
	e := state.New()

	b := edtest.InReadMode(t, e, "a\nb\nc\n", 0, 0)
	e.Lang = syntax.Detect("x.go")

	edtest.Press(t, e, "vjgc")
	edtest.WantLines(t, b, "// a", "// b", "c")
	if e.Mode != state.ReadMode {
		t.Errorf("mode = %v, want ReadMode", e.Mode)
	}

	edtest.Press(t, e, "Vjgc")
	edtest.WantLines(t, b, "a", "b", "c")
}
