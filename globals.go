package main

import "time"

var (
	ROWS, COLS             int
	offsetRow, offsetCol   int
	currentRow, currentCol int
	buf                    *Buffer
	sourceFile             string
	syntax                 *Syntax
	mode                   Mode
	modified               bool
	quitting               bool
)

// pendingCount is the count typed so far, cmdCount the one the running command
// was given. maxCount keeps a fat-fingered "99999999p" from hanging the editor.
var (
	pendingCount int
	cmdCount     int
	hadCount     bool // a count of 1 and no count at all mean different things to zz
)

const maxCount = 9999

// count is how much of itself the running command should do, defaulting to
// once when no count was typed.
func count() int {
	return max(cmdCount, 1)
}

// chordTimeout bounds how long a leading key of a two-key chord (e.g. "gg")
// stays pending before it's treated as a fresh, unrelated keypress.
const chordTimeout = 500 * time.Millisecond

var (
	lastCh     rune
	lastChTime time.Time
)

// CharClass mirrors VIM's word/punct/space split: a run of same-class
// characters is one "word" for w/b purposes, so e.g. `"foo` is two words
// (the quote, then foo) rather than one.
type CharClass int

const (
	ClassSpace CharClass = iota
	ClassWord
	ClassPunct
)

type Mode int

const (
	ReadMode Mode = iota
	EditMode
	PromptMode
	VisualMode
)
