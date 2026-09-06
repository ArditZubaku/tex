package fuzzy

import (
	"path/filepath"
	"strings"
	"testing"
	"unicode"

	"github.com/ArditZubaku/tex/internal/chars"
)

// referenceScore is the decoding version Score replaced, kept as the thing its
// answers are checked against.
func referenceScore(path string, query []rune) (int, bool) {
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
		if i == nameAt || (i > 0 && chars.ClassOf(text[i-1]) != chars.Word) {
			score += 5
		}
		at = i + 1
	}

	return score - len(text)/8, true
}

var scorePaths = []string{
	"",
	"a",
	"README.md",
	"internal/buffer/buffer.go",
	"internal/editor/render/render.go",
	"Internal/Editor/Render/RENDER.GO",
	"vendor/x/y/picker_1234.go",
	"src/ünïcödé/pläin.txt",
	"日本語/ファイル.md",
	"weird//double//slashes.go",
	"/leading/slash.go",
	"trailing/slash/",
	"_under_scored/na-me.go",
	strings.Repeat("deep/", 12) + "leaf.go",
}

var scoreQueries = []string{
	"", "a", "r", "go", "rr", "rgo", "render", "RENDER", "rndr", "bufgo",
	"zz", "//", "1234", "ü", "ünï", "日本", "_", "-", ".", "leaf", "deepleaf",
}

func TestScoreMatchesTheDecodingVersion(t *testing.T) {
	for _, path := range scorePaths {
		for _, query := range scoreQueries {
			runes := []rune(query)

			wantScore, wantOK := referenceScore(path, runes)
			gotScore, gotOK := Score(path, runes)

			if gotOK != wantOK || gotScore != wantScore {
				t.Errorf("Score(%q, %q) = %d, %v, want %d, %v",
					path, query, gotScore, gotOK, wantScore, wantOK)
			}
		}
	}
}

func TestScoreDoesNotAllocate(t *testing.T) {
	query := []rune("rndr")
	got := testing.AllocsPerRun(100, func() {
		for _, path := range scorePaths {
			_, _ = Score(path, query)
		}
	})

	if got != 0 {
		t.Errorf("Score allocated %v times per run, want 0", got)
	}
}
