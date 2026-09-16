package ngram

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/ArditZubaku/tex/internal/buffer"
)

func build(t *testing.T, content string) *Model {
	t.Helper()

	path := filepath.Join(t.TempDir(), "f.txt")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	b := buffer.Open(path)
	t.Cleanup(b.Close)

	return Build(b)
}

func TestAWordThatFollowedTheSameWordBeforeRanksFirst(t *testing.T) {
	m := build(t, "quick fox\nquick brown\nquick brown\n")

	got := m.Rank("quick", "", 10)
	if len(got) == 0 || got[0] != "brown" {
		t.Errorf("Rank(quick) = %v, want brown first (it followed quick twice, fox only once)", got)
	}
}

func TestAWordNeverSeenAfterThePreviousWordStillRanksByHowOftenItAppearsAtAll(t *testing.T) {
	m := build(t, "alpha beta\ngamma gamma gamma\n")

	got := m.Rank("zzz", "", 10)
	if len(got) == 0 || got[0] != "gamma" {
		t.Errorf("Rank(zzz) = %v, want gamma first (no bigram context, most frequent word wins)", got)
	}
}

func TestRankIsNarrowedToThePrefixTyped(t *testing.T) {
	m := build(t, "format function fold foo\n")

	got := m.Rank("", "fo", 10)
	for _, word := range got {
		if len(word) < 2 || word[:2] != "fo" {
			t.Errorf("Rank(prefix=fo) returned %q, which does not start with it", word)
		}
	}
	if len(got) != 3 {
		t.Errorf("Rank(prefix=fo) = %v, want the 3 words starting with it", got)
	}
}

func TestRankIsCappedAtTheLimit(t *testing.T) {
	m := build(t, "one two three four five\n")

	if got := m.Rank("", "", 2); len(got) != 2 {
		t.Errorf("Rank returned %d candidates, want the limit of 2", len(got))
	}
}

func TestABigramCarriesAcrossALineBreak(t *testing.T) {
	m := build(t, "zero alpha\nbravo\n")

	got := m.Rank("alpha", "", 10)
	if len(got) == 0 || got[0] != "bravo" {
		t.Errorf("Rank(alpha) = %v, want bravo first (alpha/bravo crosses the line the buffer wrapped it at)", got)
	}
}

func TestTiedCandidatesBreakAlphabeticallyForADeterministicOrder(t *testing.T) {
	m := build(t, "zeta zeta alpha alpha\n")

	got := m.Rank("", "", 10)
	if !slices.IsSorted(got[:2]) {
		t.Errorf("Rank() = %v, want the tied pair (alpha, zeta) in alphabetical order", got)
	}
}

func TestAnEmptyBufferRanksNothing(t *testing.T) {
	m := build(t, "")

	if got := m.Rank("", "", 10); len(got) != 0 {
		t.Errorf("Rank() over an empty buffer = %v, want none", got)
	}
}

func TestANonPositiveLimitRanksNothing(t *testing.T) {
	m := build(t, "one two three\n")

	if got := m.Rank("", "", 0); got != nil {
		t.Errorf("Rank(limit=0) = %v, want nil", got)
	}
}
