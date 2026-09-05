package editor

import (
	"slices"
	"strings"
	"testing"

	"github.com/ArditZubaku/tex/internal/buffer"
	"github.com/ArditZubaku/tex/internal/search"
)

func wantCursor(t *testing.T, row, col int) {
	t.Helper()

	if currentRow != row || currentCol != col {
		t.Errorf("cursor at %d,%d, want %d,%d", currentRow, currentCol, row, col)
	}
}

func TestSearchJumpsToTheStartOfTheMatch(t *testing.T) {
	inReadMode(t, "package main\nfunc needle() {}\n", 0, 0)

	press(t, "/needle\n")

	wantCursor(t, 1, 5)
}

func TestSearchStartsAfterTheCursor(t *testing.T) {
	inReadMode(t, "foo foo foo\n", 0, 0)

	press(t, "/foo\n")

	wantCursor(t, 0, 4)
}

func TestSearchWrapsAroundTheEndOfTheBuffer(t *testing.T) {
	inReadMode(t, "needle\nsomething\nelse\n", 2, 0)

	press(t, "/needle\n")

	wantCursor(t, 0, 0)
}

func TestSearchWrapsBackOntoTheStartingLine(t *testing.T) {
	inReadMode(t, "aa needle bb\n", 0, 9)

	press(t, "/needle\n")

	wantCursor(t, 0, 3)
}

func TestBackwardSearchFindsTheMatchBeforeTheCursor(t *testing.T) {
	inReadMode(t, "one two\nthree two four\n", 1, 10)

	press(t, "?two\n")

	wantCursor(t, 1, 6)
}

func TestBackwardSearchWrapsToTheEnd(t *testing.T) {
	inReadMode(t, "alpha\nbeta\nneedle\n", 0, 0)

	press(t, "?needle\n")

	wantCursor(t, 2, 0)
}

func TestNextMatchRepeatsTheSearch(t *testing.T) {
	inReadMode(t, "x\nhit\nhit\nhit\n", 0, 0)

	press(t, "/hit\nn")

	wantCursor(t, 2, 0)
}

func TestPreviousMatchRunsAgainstTheSearchDirection(t *testing.T) {
	inReadMode(t, "hit\nhit\nhit\n", 0, 0)

	press(t, "/hit\nnN")

	wantCursor(t, 1, 0)
}

func TestNextMatchAfterABackwardSearchKeepsGoingBackwards(t *testing.T) {
	inReadMode(t, "hit\nhit\nhit\n", 2, 0)

	press(t, "?hit\nn")

	wantCursor(t, 0, 0)
}

func TestCountedNextMatch(t *testing.T) {
	inReadMode(t, "hit\nhit\nhit\nhit\n", 0, 0)

	press(t, "/hit\n2n")

	wantCursor(t, 3, 0)
}

func TestSearchWithNoMatchLeavesTheCursorAndReports(t *testing.T) {
	inReadMode(t, "one\ntwo\n", 1, 1)

	press(t, "/three\n")

	wantCursor(t, 1, 1)
	if statusMsg != "Pattern not found: three" {
		t.Errorf("statusMsg = %q", statusMsg)
	}
}

func TestSearchSpansSeveralWords(t *testing.T) {
	inReadMode(t, "a\nthe quick brown fox\n", 0, 0)

	press(t, "/quick brown\n")

	wantCursor(t, 1, 4)
}

func TestSearchFindsOverlappingMatches(t *testing.T) {
	inReadMode(t, "aaaa\n", 0, 0)

	press(t, "/aa\nn")

	wantCursor(t, 0, 2)
}

func TestSearchCountsColumnsInRunesNotBytes(t *testing.T) {
	inReadMode(t, "héllo мир needle\n", 0, 0)

	press(t, "/needle\n")

	wantCursor(t, 0, 10)
}

func TestSearchIsCaseSensitive(t *testing.T) {
	inReadMode(t, "Needle\nneedle\n", 0, 0)

	press(t, "/needle\n")

	wantCursor(t, 1, 0)
}

func TestSearchSeesLinesEditedInTheOverlay(t *testing.T) {
	b := inReadMode(t, "one\ntwo\n", 0, 0)
	b.SetLine(1, []rune("a needle here"))

	press(t, "/needle\n")

	wantCursor(t, 1, 2)
}

func TestSearchReachesPastTheReadWindow(t *testing.T) {
	filler := strings.Repeat("filler line\n", buffer.WindowBytes/6)
	inReadMode(t, filler+"the needle\n", 0, 0)
	want := buf.LineCount() - 1

	press(t, "/needle\n")

	wantCursor(t, want, 4)
}

func TestEmptyPatternRepeatsTheLastSearch(t *testing.T) {
	inReadMode(t, "hit\nhit\n", 0, 0)

	press(t, "/hit\n")
	press(t, "/\n")

	wantCursor(t, 0, 0)
}

func TestEmptyPatternTakesTheDirectionItWasTypedWith(t *testing.T) {
	inReadMode(t, "hit\nhit\nhit\n", 0, 0)

	press(t, "/hit\n")
	press(t, "?\n")

	wantCursor(t, 0, 0)
}

func TestEscCancelsTheSearchPrompt(t *testing.T) {
	inReadMode(t, "one\nneedle\n", 0, 1)

	press(t, "/needle")
	press(t, string(rune(27)))

	if mode != ReadMode {
		t.Errorf("mode = %v, want ReadMode", mode)
	}
	wantCursor(t, 0, 1)
}

func TestBackspaceOnAnEmptyPromptCancelsTheSearch(t *testing.T) {
	inReadMode(t, "one\nneedle\n", 0, 1)

	press(t, "/a\b\b")

	if mode != ReadMode {
		t.Errorf("mode = %v, want ReadMode", mode)
	}
	wantCursor(t, 0, 1)
}

func TestBackspaceEditsThePattern(t *testing.T) {
	inReadMode(t, "one\nneedle\n", 0, 0)

	press(t, "/needlex\b\n")

	wantCursor(t, 1, 0)
}

func TestThePromptShowsWhatIsBeingTyped(t *testing.T) {
	inReadMode(t, "one\n", 0, 0)

	press(t, "/nee")

	if txt, ok := promptStatus(); !ok || txt != "/nee" {
		t.Errorf("promptStatus = %q,%v, want \"/nee\",true", txt, ok)
	}

	press(t, string(rune(27)))
	press(t, "?nee")

	if txt, _ := promptStatus(); txt != "?nee" {
		t.Errorf("promptStatus = %q, want %q", txt, "?nee")
	}
}

func TestNextMatchWithoutASearchDoesNothing(t *testing.T) {
	inReadMode(t, "one\ntwo\n", 0, 1)

	press(t, "nN")

	wantCursor(t, 0, 1)
	if statusMsg != "" {
		t.Errorf("statusMsg = %q, want empty", statusMsg)
	}
}

func TestMatchesAreHighlightedUntilEsc(t *testing.T) {
	inReadMode(t, "hit and hit\n", 0, 0)

	press(t, "/hit\n")

	hits := lineHits(0)
	var lit []int
	for col := range 11 {
		if hits.covers(col) {
			lit = append(lit, col)
		}
	}
	if want := []int{0, 1, 2, 8, 9, 10}; !slices.Equal(lit, want) {
		t.Errorf("highlighted columns %v, want %v", lit, want)
	}

	esc()
	if hits := lineHits(0); len(hits.cols) != 0 {
		t.Errorf("still highlighting %v after Esc", hits.cols)
	}

	press(t, "n")
	wantCursor(t, 0, 0)
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
	path := bigFile(t, 3000)
	want := referenceMatches(fullDecode(t, path), []rune("ünïcödé"))
	if len(want) < 30 {
		t.Fatalf("test file holds %d matches, too few to be interesting", len(want))
	}

	b := buffer.Open(path)
	t.Cleanup(b.Close)
	buf = b

	got := make([][2]int, 0, len(want))
	row, col := 0, -1
	for range want {
		r, c, ok := search.Find(buf, search.New([]rune("ünïcödé")), row, col, false)
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
