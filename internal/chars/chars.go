// Package chars is VIM's word/punct/space split, which the word motions and
// the picker's fuzzy match both read characters through.
package chars

// Class mirrors VIM's word/punct/space split: a run of same-class characters
// is one "word" for w/b purposes, so e.g. `"foo` is two words (the quote, then
// foo) rather than one.
type Class int

const (
	Space Class = iota
	Word
	Punct
)

func IsSpace(ch rune) bool {
	return ch == ' ' || ch == '\t'
}

func IsWord(ch rune) bool {
	return ch == '_' ||
		(ch >= 'a' && ch <= 'z') ||
		(ch >= 'A' && ch <= 'Z') ||
		(ch >= '0' && ch <= '9')
}

func ClassOf(ch rune) Class {
	switch {
	case IsSpace(ch):
		return Space
	case IsWord(ch):
		return Word
	default:
		return Punct
	}
}
