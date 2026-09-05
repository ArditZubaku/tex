// Package fuzzy is the subsequence match the picker narrows its listings with.
package fuzzy

import (
	"path/filepath"
	"strings"
	"unicode"

	"github.com/ArditZubaku/tex/internal/chars"
)

// Score matches the query as a subsequence of the path and scores what it
// found the way a file picker is usually meant: the letters together, at the
// start of a word, and in the name rather than the directories leading to it.
// The second result says whether it matched at all.
func Score(path string, query []rune) (int, bool) {
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

	// a tie between two paths goes to the shorter, which is the one with less
	// around what was typed
	return score - len(text)/8, true
}
