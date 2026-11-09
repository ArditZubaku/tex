package find_test

import (
	"strings"
	"testing"

	"github.com/ArditZubaku/tex/internal/buffer"
	"github.com/ArditZubaku/tex/internal/editor/edtest"
	"github.com/ArditZubaku/tex/internal/editor/state"
)

// openReplace presses '<leader>fr' and clears whatever the box seeded itself
// with, so that a test types the pattern it means rather than editing the word
// the cursor happened to be on.
func openReplace(t *testing.T, e *state.Editor, content string, row, col int) *buffer.Buffer {
	t.Helper()

	b := edtest.InReadMode(t, e, content, row, col)
	edtest.SingleWindow(e, 24, 80)
	edtest.Press(t, e, " fr")

	if e.Mode != state.FormMode {
		t.Fatalf("mode = %v after '<leader>fr', want FormMode", e.Mode)
	}
	edtest.Press(t, e, "\x15") // Ctrl-U

	return b
}

// fill types the pattern into the first box and the replacement into the
// second, which is the Tab between them the form is for.
func fill(t *testing.T, e *state.Editor, pattern, with string) {
	t.Helper()

	edtest.Press(t, e, pattern+"\t"+with)
}

func TestLeaderFROpensTheTwoBoxesOnTheWordUnderTheCursor(t *testing.T) {
	e := state.New()
	edtest.InReadMode(t, e, "alpha needle beta\n", 0, 6)
	edtest.SingleWindow(e, 24, 80)

	edtest.Press(t, e, " fr")

	if e.Mode != state.FormMode {
		t.Fatalf("mode = %v, want FormMode", e.Mode)
	}
	if got := string(e.Form.Value(0)); got != "needle" {
		t.Errorf("the Search box holds %q, want %q", got, "needle")
	}
	if got := string(e.Form.Value(1)); got != "" {
		t.Errorf("the Replace box holds %q, want it empty", got)
	}
}

// The pattern is the '/' search's own, which is what lights the matches: the
// renderer has no second idea of what a match is.
func TestTypingThePatternLightsItsMatches(t *testing.T) {
	e := state.New()
	openReplace(t, e, "foo bar foo\n", 0, 0)

	edtest.Press(t, e, "foo")

	if got := string(e.SearchPat.Runes()); got != "foo" {
		t.Errorf("the search pattern is %q, want %q", got, "foo")
	}
	if !e.HlSearch {
		t.Error("the matches are not lit")
	}
}

func TestEnterReplacesEveryMatchInTheFile(t *testing.T) {
	e := state.New()
	b := openReplace(t, e, "foo bar\nbaz foo\nfoo foo\n", 0, 0)

	fill(t, e, "foo", "qux")
	edtest.Press(t, e, "\n")

	edtest.WantLines(t, b, "qux bar", "baz qux", "qux qux")
	if e.Mode != state.ReadMode {
		t.Errorf("mode = %v after Enter, want ReadMode", e.Mode)
	}
	if !e.Modified {
		t.Error("the buffer is not marked modified")
	}
}

func TestTheReplacementReportsWhatItDid(t *testing.T) {
	e := state.New()
	openReplace(t, e, "foo bar\nbaz foo\n", 0, 0)

	fill(t, e, "foo", "qux")
	edtest.Press(t, e, "\n")

	if got, want := e.StatusMsg, "2 substitutions on 2 lines"; got != want {
		t.Errorf("status = %q, want %q", got, want)
	}
}

func TestOneMatchIsReportedInTheSingular(t *testing.T) {
	e := state.New()
	openReplace(t, e, "foo bar\n", 0, 0)

	fill(t, e, "foo", "qux")
	edtest.Press(t, e, "\n")

	if got, want := e.StatusMsg, "1 substitution on 1 line"; got != want {
		t.Errorf("status = %q, want %q", got, want)
	}
}

// A pattern that overlaps itself matches twice over, and a replacement cannot:
// the second match reaches back into text the first one took.
func TestOverlappingMatchesAreReplacedLeftToRight(t *testing.T) {
	e := state.New()
	b := openReplace(t, e, "aaa\n", 0, 0)

	fill(t, e, "aa", "b")
	edtest.Press(t, e, "\n")

	edtest.WantLines(t, b, "ba")
}

func TestAnEmptyReplacementTakesTheMatchesOut(t *testing.T) {
	e := state.New()
	b := openReplace(t, e, "foo bar foo\n", 0, 0)

	edtest.Press(t, e, "foo \n")

	edtest.WantLines(t, b, "bar foo")
}

func TestTheWholeReplacementIsOneUndo(t *testing.T) {
	e := state.New()
	b := openReplace(t, e, "foo\nfoo\nfoo\n", 0, 0)

	fill(t, e, "foo", "qux")
	edtest.Press(t, e, "\n")
	edtest.Press(t, e, "u")

	edtest.WantLines(t, b, "foo", "foo", "foo")
}

func TestEscLeavesTheFileAsItWas(t *testing.T) {
	e := state.New()
	b := openReplace(t, e, "foo bar\n", 0, 0)

	fill(t, e, "foo", "qux")
	edtest.Press(t, e, "\x1b")

	edtest.WantLines(t, b, "foo bar")
	if e.Mode != state.ReadMode {
		t.Errorf("mode = %v after Esc, want ReadMode", e.Mode)
	}
	if e.Modified {
		t.Error("a cancelled replacement marked the buffer modified")
	}
}

func TestAPatternThatMatchesNothingIsReported(t *testing.T) {
	e := state.New()
	b := openReplace(t, e, "foo bar\n", 0, 0)

	fill(t, e, "missing", "qux")
	edtest.Press(t, e, "\n")

	edtest.WantLines(t, b, "foo bar")
	if !strings.Contains(e.StatusMsg, "Pattern not found") {
		t.Errorf("status = %q, want it to report the pattern was not found", e.StatusMsg)
	}
}

func TestAnEmptyPatternReplacesNothing(t *testing.T) {
	e := state.New()
	b := openReplace(t, e, "foo bar\n", 0, 0)

	edtest.Press(t, e, "\tqux\n")

	edtest.WantLines(t, b, "foo bar")
	if got, want := e.StatusMsg, "E471: Argument required"; got != want {
		t.Errorf("status = %q, want %q", got, want)
	}
}

// Replacing a match with something shorter can leave the cursor past the end of
// the line it was on, which Read mode never allows.
func TestTheCursorStaysOnTheLineAfterItShrinks(t *testing.T) {
	e := state.New()
	openReplace(t, e, "hello world\nfoofoofoo\n", 1, 8)

	fill(t, e, "foo", "x")
	edtest.Press(t, e, "\n")

	edtest.WantCursor(t, e, 1, 2)
}
