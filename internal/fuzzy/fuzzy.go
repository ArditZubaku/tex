// Package fuzzy is the subsequence match the picker narrows its listings with.
package fuzzy

import (
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/ArditZubaku/tex/internal/chars"
)

// Score matches the query as a subsequence of the path and scores what it
// found the way a file picker is usually meant: the letters together, at the
// start of a word, and in the name rather than the directories leading to it.
// The second result says whether it matched at all.
//
// Every position below is a byte offset into the path rather than an index into
// a decoded copy of it: the picker rescores its whole listing on every
// keystroke, and the comparisons here — adjacent, in the name, after a
// non-word character — read the same either way.
func Score(path string, query []rune) (int, bool) {
	if len(query) == 0 {
		return 0, true
	}

	nameAt, ascii := 0, true
	for i := range len(path) {
		switch {
		case path[i] == filepath.Separator:
			nameAt = i + 1
		case path[i] >= utf8.RuneSelf:
			ascii = false
		}
	}

	score, at := 0, 0
	for _, want := range query {
		i, size := findFold(path, at, unicode.ToLower(want), ascii)
		if i < 0 {
			return 0, false
		}

		run := 0
		if i == at && at > 0 {
			run = 4
		}
		score += 1 + run
		if i >= nameAt {
			score += 3
		}
		if i == nameAt || (i > 0 && chars.ClassOf(runeBefore(path, i)) != chars.Word) {
			score += 5
		}
		at = i + size
	}

	length := len(path)
	if !ascii {
		length = utf8.RuneCountInString(path)
	}

	// a tie between two paths goes to the shorter, which is the one with less
	// around what was typed
	return score - length/8, true
}

// findFold is where the first character at or after from that lowercases to
// want starts, and how many bytes it takes. An all-ASCII path is looked through
// for the two cases of want at once, which the runtime scans a word at a time;
// anything else is decoded, since a rune outside ASCII can still fold into it.
func findFold(path string, from int, want rune, ascii bool) (int, int) {
	if ascii && want < utf8.RuneSelf {
		at := indexEither(path[from:], byte(want), byte(unicode.ToUpper(want)))
		if at < 0 {
			return -1, 0
		}

		return from + at, 1
	}

	for i := from; i < len(path); {
		ch, size := utf8.DecodeRuneInString(path[i:])
		if unicode.ToLower(ch) == want {
			return i, size
		}
		i += size
	}

	return -1, 0
}

func indexEither(txt string, a, b byte) int {
	at := strings.IndexByte(txt, a)
	if a == b {
		return at
	}

	other := strings.IndexByte(txt, b)
	switch {
	case at < 0:
		return other
	case other < 0:
		return at
	}

	return min(at, other)
}

func runeBefore(path string, i int) rune {
	ch, _ := utf8.DecodeLastRuneInString(path[:i])

	return unicode.ToLower(ch)
}
