package find_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/ArditZubaku/tex/internal/buffer"
	"github.com/ArditZubaku/tex/internal/editor/edtest"
	"github.com/ArditZubaku/tex/internal/editor/find"
	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/ArditZubaku/tex/internal/search"
)

func TestSearchJumpsToTheStartOfTheMatch(t *testing.T) {
	e := state.New()

	edtest.InReadMode(t, e, "package main\nfunc needle() {}\n", 0, 0)

	edtest.Press(t, e, "/needle\n")

	edtest.WantCursor(t, e, 1, 5)
}

func TestSearchStartsAfterTheCursor(t *testing.T) {
	e := state.New()

	edtest.InReadMode(t, e, "foo foo foo\n", 0, 0)

	edtest.Press(t, e, "/foo\n")

	edtest.WantCursor(t, e, 0, 4)
}

func TestSearchWrapsAroundTheEndOfTheBuffer(t *testing.T) {
	e := state.New()

	edtest.InReadMode(t, e, "needle\nsomething\nelse\n", 2, 0)

	edtest.Press(t, e, "/needle\n")

	edtest.WantCursor(t, e, 0, 0)
}

func TestSearchWrapsBackOntoTheStartingLine(t *testing.T) {
	e := state.New()

	edtest.InReadMode(t, e, "aa needle bb\n", 0, 9)

	edtest.Press(t, e, "/needle\n")

	edtest.WantCursor(t, e, 0, 3)
}

func TestBackwardSearchFindsTheMatchBeforeTheCursor(t *testing.T) {
	e := state.New()

	edtest.InReadMode(t, e, "one two\nthree two four\n", 1, 10)

	edtest.Press(t, e, "?two\n")

	edtest.WantCursor(t, e, 1, 6)
}

func TestBackwardSearchWrapsToTheEnd(t *testing.T) {
	e := state.New()

	edtest.InReadMode(t, e, "alpha\nbeta\nneedle\n", 0, 0)

	edtest.Press(t, e, "?needle\n")

	edtest.WantCursor(t, e, 2, 0)
}

func TestNextMatchRepeatsTheSearch(t *testing.T) {
	e := state.New()

	edtest.InReadMode(t, e, "x\nhit\nhit\nhit\n", 0, 0)

	edtest.Press(t, e, "/hit\nn")

	edtest.WantCursor(t, e, 2, 0)
}

func TestPreviousMatchRunsAgainstTheSearchDirection(t *testing.T) {
	e := state.New()

	edtest.InReadMode(t, e, "hit\nhit\nhit\n", 0, 0)

	edtest.Press(t, e, "/hit\nnN")

	edtest.WantCursor(t, e, 1, 0)
}

func TestNextMatchAfterABackwardSearchKeepsGoingBackwards(t *testing.T) {
	e := state.New()

	edtest.InReadMode(t, e, "hit\nhit\nhit\n", 2, 0)

	edtest.Press(t, e, "?hit\nn")

	edtest.WantCursor(t, e, 0, 0)
}

func TestCountedNextMatch(t *testing.T) {
	e := state.New()

	edtest.InReadMode(t, e, "hit\nhit\nhit\nhit\n", 0, 0)

	edtest.Press(t, e, "/hit\n2n")

	edtest.WantCursor(t, e, 3, 0)
}

func TestSearchWithNoMatchLeavesTheCursorAndReports(t *testing.T) {
	e := state.New()

	edtest.InReadMode(t, e, "one\ntwo\n", 1, 1)

	edtest.Press(t, e, "/three\n")

	edtest.WantCursor(t, e, 1, 1)
	if e.StatusMsg != "Pattern not found: three" {
		t.Errorf("statusMsg = %q", e.StatusMsg)
	}
}

func TestSearchSpansSeveralWords(t *testing.T) {
	e := state.New()

	edtest.InReadMode(t, e, "a\nthe quick brown fox\n", 0, 0)

	edtest.Press(t, e, "/quick brown\n")

	edtest.WantCursor(t, e, 1, 4)
}

func TestSearchFindsOverlappingMatches(t *testing.T) {
	e := state.New()

	edtest.InReadMode(t, e, "aaaa\n", 0, 0)

	edtest.Press(t, e, "/aa\nn")

	edtest.WantCursor(t, e, 0, 2)
}

func TestSearchCountsColumnsInRunesNotBytes(t *testing.T) {
	e := state.New()

	edtest.InReadMode(t, e, "héllo мир needle\n", 0, 0)

	edtest.Press(t, e, "/needle\n")

	edtest.WantCursor(t, e, 0, 10)
}

func TestSearchIsCaseSensitive(t *testing.T) {
	e := state.New()

	edtest.InReadMode(t, e, "Needle\nneedle\n", 0, 0)

	edtest.Press(t, e, "/needle\n")

	edtest.WantCursor(t, e, 1, 0)
}

func TestSearchSeesLinesEditedInTheOverlay(t *testing.T) {
	e := state.New()

	b := edtest.InReadMode(t, e, "one\ntwo\n", 0, 0)
	b.SetLine(1, []rune("a needle here"))

	edtest.Press(t, e, "/needle\n")

	edtest.WantCursor(t, e, 1, 2)
}

func TestSearchReachesPastTheReadWindow(t *testing.T) {
	e := state.New()

	filler := strings.Repeat("filler line\n", buffer.WindowBytes/6)
	edtest.InReadMode(t, e, filler+"the needle\n", 0, 0)
	want := e.Buf.LineCount() - 1

	edtest.Press(t, e, "/needle\n")

	edtest.WantCursor(t, e, want, 4)
}

func TestEmptyPatternRepeatsTheLastSearch(t *testing.T) {
	e := state.New()

	edtest.InReadMode(t, e, "hit\nhit\n", 0, 0)

	edtest.Press(t, e, "/hit\n")
	edtest.Press(t, e, "/\n")

	edtest.WantCursor(t, e, 0, 0)
}

func TestEmptyPatternTakesTheDirectionItWasTypedWith(t *testing.T) {
	e := state.New()

	edtest.InReadMode(t, e, "hit\nhit\nhit\n", 0, 0)

	edtest.Press(t, e, "/hit\n")
	edtest.Press(t, e, "?\n")

	edtest.WantCursor(t, e, 0, 0)
}

func TestEscCancelsTheSearchPrompt(t *testing.T) {
	e := state.New()

	edtest.InReadMode(t, e, "one\nneedle\n", 0, 1)

	edtest.Press(t, e, "/needle")
	edtest.Press(t, e, string(rune(27)))

	if e.Mode != state.ReadMode {
		t.Errorf("mode = %v, want ReadMode", e.Mode)
	}
	edtest.WantCursor(t, e, 0, 1)
}

func TestBackspaceOnAnEmptyPromptCancelsTheSearch(t *testing.T) {
	e := state.New()

	edtest.InReadMode(t, e, "one\nneedle\n", 0, 1)

	edtest.Press(t, e, "/a\b\b")

	if e.Mode != state.ReadMode {
		t.Errorf("mode = %v, want ReadMode", e.Mode)
	}
	edtest.WantCursor(t, e, 0, 1)
}

func TestBackspaceEditsThePattern(t *testing.T) {
	e := state.New()

	edtest.InReadMode(t, e, "one\nneedle\n", 0, 0)

	edtest.Press(t, e, "/needlex\b\n")

	edtest.WantCursor(t, e, 1, 0)
}

func TestThePromptShowsWhatIsBeingTyped(t *testing.T) {
	e := state.New()

	edtest.InReadMode(t, e, "one\n", 0, 0)

	edtest.Press(t, e, "/nee")

	if txt, ok := e.PromptStatus(); !ok || txt != "/nee" {
		t.Errorf("promptStatus = %q,%v, want \"/nee\",true", txt, ok)
	}

	edtest.Press(t, e, string(rune(27)))
	edtest.Press(t, e, "?nee")

	if txt, _ := e.PromptStatus(); txt != "?nee" {
		t.Errorf("promptStatus = %q, want %q", txt, "?nee")
	}
}

func TestNextMatchWithoutASearchDoesNothing(t *testing.T) {
	e := state.New()

	edtest.InReadMode(t, e, "one\ntwo\n", 0, 1)

	edtest.Press(t, e, "nN")

	edtest.WantCursor(t, e, 0, 1)
	if e.StatusMsg != "" {
		t.Errorf("statusMsg = %q, want empty", e.StatusMsg)
	}
}

func TestMatchesAreHighlightedUntilEsc(t *testing.T) {
	e := state.New()

	edtest.InReadMode(t, e, "hit and hit\n", 0, 0)

	edtest.Press(t, e, "/hit\n")

	hits := find.LineHits(e, 0)
	var lit []int
	for col := range 11 {
		if hits.Covers(col) {
			lit = append(lit, col)
		}
	}
	if want := []int{0, 1, 2, 8, 9, 10}; !slices.Equal(lit, want) {
		t.Errorf("highlighted columns %v, want %v", lit, want)
	}

	edtest.Esc(t, e)
	if hits := find.LineHits(e, 0); len(hits.Cols()) != 0 {
		t.Errorf("still highlighting %v after Esc", hits.Cols())
	}

	edtest.Press(t, e, "n")
	edtest.WantCursor(t, e, 0, 0)
}

// referenceMatches is the brute-force answer the windowed search is checked
// against: every (row, col) the pattern occurs at, over fully decoded lines.
func referenceMatches(lines [][]rune, pat []rune) [][2]int {
	var want [][2]int
	for row, line := range lines {
		for col := 0; col+len(pat) <= len(line); col++ {
			if slices.Equal(line[col:col+len(pat)], pat) {
				want = append(want, [2]int{row, col})
			}
		}
	}

	return want
}

func TestSearchOverBigFileMatchesFullDecode(t *testing.T) {
	e := state.New()

	path := edtest.BigFile(t, 3000)
	want := referenceMatches(edtest.FullDecode(t, path), []rune("ünïcödé"))
	if len(want) < 30 {
		t.Fatalf("test file holds %d matches, too few to be interesting", len(want))
	}

	b := buffer.Open(path)
	t.Cleanup(b.Close)
	e.Buf = b

	got := make([][2]int, 0, len(want))
	row, col := 0, -1
	for range want {
		r, c, ok := search.Find(e.Buf, search.New([]rune("ünïcödé")), row, col, false)
		if !ok {
			t.Fatalf("search stopped after %d of %d matches", len(got), len(want))
		}
		got = append(got, [2]int{r, c})
		row, col = r, c
	}

	if !slices.Equal(got, want) {
		for i := range got {
			if got[i] != want[i] {
				t.Fatalf("match %d at %v, want %v", i, got[i], want[i])
			}
		}
	}
}
