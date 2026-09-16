// Package ngram ranks a buffer's own words as completions for the word being
// typed, by how often they have followed the word before it. It is the
// fallback complete.Item source for files with no language server: nothing
// behind a candidate but the buffer having used it once already.
package ngram

import (
	"slices"
	"strings"

	"github.com/ArditZubaku/tex/internal/buffer"
	"github.com/ArditZubaku/tex/internal/chars"
)

// A Model is one buffer's word and word-pair counts, built fresh for every
// request rather than kept in step with edits: a single pass over one buffer
// is the same cost class as the syntax highlighter's own re-lex on every
// keystroke, and simpler than staying correct through every edit path that
// can change a line.
type Model struct {
	uni map[string]int
	bi  map[string]map[string]int
}

// Build tokenizes every line of b the way word motions do — chars.IsWord runs
// — and counts each word and each pair of words adjacent in reading order.
// The pair carries across a line break, so a sentence wrapped by the buffer
// rather than the author still counts as one.
func Build(b *buffer.Buffer) *Model {
	m := &Model{uni: map[string]int{}, bi: map[string]map[string]int{}}

	var scratch []rune
	prev := ""
	for i := range b.LineCount() {
		scratch = b.LineInto(i, scratch)
		prev = m.addLine(scratch, prev)
	}

	return m
}

// addLine counts the words of one line against prev, the last word of the
// line before, and answers with its own last word for the next line to carry.
func (m *Model) addLine(line []rune, prev string) string {
	for start := 0; start < len(line); {
		if !chars.IsWord(line[start]) {
			start++
			continue
		}

		end := start + 1
		for end < len(line) && chars.IsWord(line[end]) {
			end++
		}

		word := string(line[start:end])
		m.uni[word]++
		if prev != "" {
			bucket := m.bi[prev]
			if bucket == nil {
				bucket = map[string]int{}
				m.bi[prev] = bucket
			}
			bucket[word]++
		}

		prev, start = word, end
	}

	return prev
}

type candidate struct {
	word    string
	bi, uni int
}

// Rank is the buffer's words starting with prefix, most likely first: ones
// that have followed prev before, by how often, then everything else by how
// often it appears at all. Ties break alphabetically, so the same buffer
// always lists candidates in the same order.
func (m *Model) Rank(prev, prefix string, limit int) []string {
	if limit <= 0 {
		return nil
	}

	seen := make(map[string]bool, limit)
	out := make([]candidate, 0, limit)

	add := func(word string, bi int) {
		if seen[word] || !strings.HasPrefix(word, prefix) {
			return
		}

		seen[word] = true
		out = append(out, candidate{word, bi, m.uni[word]})
	}

	for word, count := range m.bi[prev] {
		add(word, count)
	}
	for word := range m.uni {
		add(word, 0)
	}

	slices.SortFunc(out, func(a, b candidate) int {
		if a.bi != b.bi {
			return b.bi - a.bi
		}

		if a.uni != b.uni {
			return b.uni - a.uni
		}

		return strings.Compare(a.word, b.word)
	})

	if len(out) > limit {
		out = out[:limit]
	}

	words := make([]string, len(out))
	for i, c := range out {
		words[i] = c.word
	}

	return words
}
