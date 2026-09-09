// Package syntax is every language's lexical surface the editor can colour:
// enough to tell comments, strings, numbers and reserved words apart, and
// nothing more.
package syntax

import (
	"bytes"
	"path/filepath"
	"strings"

	"github.com/nsf/termbox-go"

	"github.com/ArditZubaku/tex/internal/chars"
	"github.com/ArditZubaku/tex/internal/theme"
)

// Syntax is one language's lexical surface: enough to tell comments, strings,
// numbers and reserved words apart, and nothing more. Every delimiter is
// ASCII, so a rune of a line matches a byte of a delimiter one for one.
type Syntax struct {
	lineComment string
	blockStart  string
	blockEnd    string
	quotes      string
	quoteSet    [asciiMax]bool

	// the first rune of each delimiter, so that the common case of a character
	// that starts none of them costs a compare
	commentAt, blockAt, blockEndAt rune

	keywords  map[string]bool
	types     map[string]bool
	builtins  map[string]bool
	constants map[string]bool

	// kinds is the four tables above merged, so that a word costs one lookup
	// rather than four misses; longest is the length past which none can match,
	// and firstOf which ASCII byte a name of each length can start with, which
	// rules most identifiers out before the table is reached at all.
	kinds   map[string]wordKind
	longest int
	firstOf [wordBytes + 1][2]uint64
}

// A wordKind is which of a language's tables a word is in, which is as much as
// the colour of it depends on.
type wordKind uint8

const (
	plainWord wordKind = iota
	keywordWord
	constantWord
	typeWord
	builtinWord
)

// wordBytes bounds the key a lookup is built in. A word longer than the
// longest name any table holds cannot be in one, so it never reaches the key.
const wordBytes = 32

// asciiMax bounds the lookup tables below. Every delimiter a language of the
// tables here uses is ASCII, so a byte-wide set covers all of them.
const asciiMax = 128

func words(list string) map[string]bool {
	set := make(map[string]bool)
	for w := range strings.FieldsSeq(list) {
		set[w] = true
	}

	return set
}

var goSyntax = &Syntax{
	lineComment: "//",
	blockStart:  "/*",
	blockEnd:    "*/",
	quotes:      "\"'`",
	keywords: words(`break case chan const continue default defer else fallthrough for
		func go goto if import interface map package range return select struct switch type var`),
	types: words(`any bool byte comparable complex64 complex128 error float32 float64
		int int8 int16 int32 int64 rune string uint uint8 uint16 uint32 uint64 uintptr`),
	builtins: words(`append cap clear close complex copy delete imag len make max min new
		panic print println real recover`),
	constants: words(`true false nil iota`),
}

// One table covers the whole C-descended family: a keyword of a language the
// file isn't written in simply never appears in it.
var cSyntax = &Syntax{
	lineComment: "//",
	blockStart:  "/*",
	blockEnd:    "*/",
	quotes:      "\"'`",
	keywords: words(`abstract alignas alignof and as asm async await break case catch class
		const constexpr continue crate debugger default defer delete do dyn else enum explicit
		export extern final finally fn for friend function goto if impl implements import in
		instanceof interface let loop match mod move mut namespace new operator override package
		private protected pub public readonly ref register return sizeof static struct super
		switch synchronized template throw throws trait try typedef typeof union unsafe
		use using var virtual volatile where while with yield`),
	types: words(`auto bool boolean byte char double f32 f64 float i8 i16 i32 i64 i128
		int isize long never number object short signed size_t str string symbol tuple u8
		u16 u32 u64 u128 uint unknown unsigned usize void wchar_t
		Self String Vec Option Result Promise Array Map Set Object String Number Boolean`),
	builtins: words(`console printf sprintf fprintf scanf malloc calloc realloc free memcpy
		memset strlen strcmp assert require println print panic drop clone unwrap
		parseInt parseFloat isNaN JSON Math`),
	constants: words(`true false null nil undefined NULL nullptr this self`),
}

// Anything whose comments start with '#'. Python and the shells share enough
// of their vocabulary that one table reads well for both.
var hashSyntax = &Syntax{
	lineComment: "#",
	quotes:      "\"'",
	keywords: words(`alias and as assert async await break case class continue declare def del
		do done elif else esac except exit export fi finally for from function global if import
		in is lambda local nonlocal not or pass raise readonly return select set shift source
		then trap try unset until while with yield`),
	types: words(`bool bytes complex dict float frozenset int list object set str tuple type`),
	builtins: words(`abs all any bin callable chr dir divmod enumerate eval exec filter format
		getattr hasattr hex id input isinstance issubclass iter len map max min next open ord
		print range repr reversed round setattr sorted sum super zip
		cd echo printf pwd read shift source test unset`),
	constants: words(`True False None NotImplemented Ellipsis self cls true false null`),
}

// The lookups the lexer reads on its hot path are derived rather than written
// out, so that a language's literal above stays the one place it is described.
func init() {
	for _, s := range []*Syntax{goSyntax, cSyntax, hashSyntax} {
		for _, quote := range s.quotes {
			s.quoteSet[quote] = true
		}

		s.commentAt, s.blockAt = firstRune(s.lineComment), firstRune(s.blockStart)
		s.blockEndAt = firstRune(s.blockEnd)

		s.kinds = make(map[string]wordKind)
		// least specific first, so that a word in two tables keeps the colour
		// the switch this replaced would have reached first
		s.merge(s.builtins, builtinWord)
		s.merge(s.types, typeWord)
		s.merge(s.constants, constantWord)
		s.merge(s.keywords, keywordWord)
	}
}

func (s *Syntax) merge(words map[string]bool, kind wordKind) {
	for word := range words {
		if len(word) > wordBytes {
			continue
		}
		s.kinds[word] = kind
		s.longest = max(s.longest, len(word))
		if first := word[0]; first < asciiMax {
			s.firstOf[len(word)][first>>6] |= 1 << (first & 63)
		}
	}
}

// noRune is what a language without that delimiter gets: nothing in a line
// equals it, so the check below simply never fires.
const noRune = rune(-1)

func firstRune(delimiter string) rune {
	if delimiter == "" {
		return noRune
	}

	return rune(delimiter[0])
}

var extensionSyntax = map[string]*Syntax{
	".go": goSyntax,

	".c": cSyntax, ".h": cSyntax, ".cc": cSyntax, ".cpp": cSyntax, ".hpp": cSyntax,
	".cs": cSyntax, ".java": cSyntax, ".js": cSyntax, ".jsx": cSyntax, ".mjs": cSyntax,
	".cjs": cSyntax, ".ts": cSyntax, ".tsx": cSyntax, ".rs": cSyntax, ".kt": cSyntax,
	".swift": cSyntax, ".scala": cSyntax, ".dart": cSyntax, ".php": cSyntax, ".zig": cSyntax,

	".py": hashSyntax, ".sh": hashSyntax, ".bash": hashSyntax, ".zsh": hashSyntax,
	".rb": hashSyntax, ".pl": hashSyntax, ".yaml": hashSyntax, ".yml": hashSyntax,
	".toml": hashSyntax, ".tf": hashSyntax, ".conf": hashSyntax,
}

var baseNameSyntax = map[string]*Syntax{
	"makefile": hashSyntax, "dockerfile": hashSyntax, "gemfile": hashSyntax,
	".bashrc": hashSyntax, ".zshrc": hashSyntax, ".profile": hashSyntax,
	".gitignore": hashSyntax, ".gitconfig": hashSyntax, ".env": hashSyntax,
}

// detectSyntax returns nil for a file no rule matches, which leaves it drawn
// in the terminal's own colours.
func Detect(name string) *Syntax {
	base := strings.ToLower(filepath.Base(name))
	if s, ok := extensionSyntax[filepath.Ext(base)]; ok {
		return s
	}

	return baseNameSyntax[base]
}

// colors is reused by every row of every redraw: highlighting a screenful
// allocates only when a line is longer than the longest one drawn so far.
var colors []termbox.Attribute

// LineColors returns one colour per rune of line, or nil when the file has no
// syntax, plus the block-comment state the line below starts in.
func (s *Syntax) LineColors(line []rune, inBlock bool, palette *theme.Palette) ([]termbox.Attribute, bool) {
	if s == nil {
		return nil, false
	}

	if cap(colors) < len(line) {
		colors = make([]termbox.Attribute, len(line))
	}

	out := colors[:len(line)]
	for i := range out {
		out[i] = palette.Plain
	}

	return out, s.Highlight(line, inBlock, out, palette)
}

// Code reports which columns of a line the lexer reads as code rather than as a
// comment or a string, and the block-comment state the line below starts in. It
// is the same pass the screen is coloured by, run against a palette that tells
// those two apart, so what a command reads as code cannot drift from what the
// colours show. A file of no known language answers nil, which is every column.
func (s *Syntax) Code(line []rune, inBlock bool, out []bool) ([]bool, bool) {
	if s == nil {
		return nil, false
	}

	if cap(marks) < len(line) {
		marks = make([]termbox.Attribute, len(line))
	}
	painted := marks[:len(line)]
	for i := range painted {
		painted[i] = codeMark
	}
	inBlock = s.Highlight(line, inBlock, painted, &proseOnly)

	for _, at := range painted {
		out = append(out, at == codeMark)
	}

	return out, inBlock
}

// proseOnly is the whole palette Code needs: one shade for what is written for
// people to read, another for everything else.
const (
	codeMark  = termbox.Attribute(0)
	proseMark = termbox.Attribute(1)
)

var (
	marks     []termbox.Attribute
	proseOnly = theme.Palette{Comment: proseMark, StringLit: proseMark, Escape: proseMark}
)

// hasPrefixAt takes the delimiter's first rune separately so that the common
// case — a character that starts no delimiter at all — costs one compare. Past
// it, a delimiter is ASCII, so its bytes are compared against the line's runes
// one for one, without decoding it. A language without the delimiter passes
// noRune, which nothing in a line is.
func hasPrefixAt(line []rune, i int, first rune, prefix string) bool {
	if line[i] != first || i+len(prefix) > len(line) {
		return false
	}
	for j := 1; j < len(prefix); j++ {
		if line[i+j] != rune(prefix[j]) {
			return false
		}
	}

	return true
}

func paint(out []termbox.Attribute, from, to int, color termbox.Attribute) {
	for i := from; i < to && i < len(out); i++ {
		out[i] = color
	}
}

// highlight colours one line into out (which may be nil to run the lexer for
// its state alone) and reports whether the line leaves a block comment open,
// which is the only state the next line needs.
// HasBlockComments says whether the language has a comment that can span
// lines, which is the one construct a redraw cannot lex a line at a time.
func (s *Syntax) HasBlockComments() bool {
	return s != nil && s.blockStart != ""
}

// LineComment is the delimiter 'gcc' toggles at the front of a line, or "" for
// a file with no known syntax.
func (s *Syntax) LineComment() string {
	if s == nil {
		return ""
	}

	return s.lineComment
}

// CanChangeBlock says whether a line could leave the block-comment state
// different from how it found it. Only the opening delimiter opens one and only
// the closing delimiter closes it, so a line whose raw bytes hold neither the
// first byte of the one that applies leaves the state exactly as it was —
// whatever quotes, words or line comments it holds. It is what lets a redraw
// look back over a screenful of lines without decoding most of them.
func (s *Syntax) CanChangeBlock(raw []byte, inBlock bool) bool {
	if !s.HasBlockComments() {
		return false
	}
	if inBlock {
		return bytes.IndexByte(raw, s.blockEnd[0]) >= 0
	}

	return bytes.IndexByte(raw, s.blockStart[0]) >= 0
}

func (s *Syntax) Highlight(line []rune, inBlock bool, out []termbox.Attribute, palette *theme.Palette) bool {
	for i := 0; i < len(line); {
		switch {
		case inBlock:
			start := i
			i, inBlock = s.scanBlock(line, i)
			paint(out, start, i, palette.Comment)

		case hasPrefixAt(line, i, s.commentAt, s.lineComment):
			paint(out, i, len(line), palette.Comment)

			return false

		case hasPrefixAt(line, i, s.blockAt, s.blockStart):
			start := i
			i, inBlock = s.scanBlock(line, i+len(s.blockStart))
			paint(out, start, i, palette.Comment)

		case line[i] < asciiMax && s.quoteSet[line[i]]:
			i = s.scanString(line, i, out, palette)

		case chars.IsWord(line[i]):
			i = s.scanWord(line, i, out, palette)

		default:
			i++
		}
	}

	return inBlock
}

// A name the language reserves nothing for is still worth colouring when it is
// being called: an open bracket right after it is what tells a call from a
// variable, which is as far as one line of context reaches.
func (s *Syntax) wordColor(word []rune, line []rune, after int, palette *theme.Palette) termbox.Attribute {
	switch s.kindOf(word) {
	case keywordWord:
		return palette.Keyword
	case constantWord:
		return palette.Constant
	case typeWord:
		return palette.TypeName
	case builtinWord:
		return palette.Builtin
	}
	if after < len(line) && line[after] == '(' {
		return palette.Function
	}

	return palette.Plain
}

// A word is ASCII by construction, so its key is copied a byte at a time into
// an array on the stack: indexing a map by a string over one of those is the
// shape the compiler turns into a lookup with nothing allocated for it.
func (s *Syntax) kindOf(word []rune) wordKind {
	if len(word) > s.longest {
		return plainWord
	}
	if first := word[0]; first >= asciiMax || s.firstOf[len(word)][first>>6]&(1<<(first&63)) == 0 {
		return plainWord
	}

	var key [wordBytes]byte
	for i, ch := range word {
		key[i] = byte(ch)
	}

	return s.kinds[string(key[:len(word)])]
}

// scanWord takes the run of word characters at i — a number if it starts with
// a digit, which is the one run a '.' carries on through — and colours it.
func (s *Syntax) scanWord(line []rune, i int, out []termbox.Attribute, palette *theme.Palette) int {
	start := i

	if line[i] >= '0' && line[i] <= '9' {
		for i < len(line) && (chars.IsWord(line[i]) || line[i] == '.') {
			i++
		}
		paint(out, start, i, palette.Number)

		return i
	}

	for i < len(line) && chars.IsWord(line[i]) {
		i++
	}
	// A run of a lexer asked only for its block-comment state paints nothing,
	// and a word's colour is the most expensive thing here.
	if out != nil {
		paint(out, start, i, s.wordColor(line[start:i], line, i, palette))
	}

	return i
}

func (s *Syntax) scanBlock(line []rune, i int) (int, bool) {
	for ; i < len(line); i++ {
		if hasPrefixAt(line, i, s.blockEndAt, s.blockEnd) {
			return i + len(s.blockEnd), false
		}
	}

	return len(line), true
}

// An unterminated string runs to the end of the line rather than into the next
// one: half-typed quotes are normal in Insert mode, and colouring the rest of
// the file over one of them would be worse than getting the line wrong.
func (s *Syntax) scanString(line []rune, i int, out []termbox.Attribute, palette *theme.Palette) int {
	quote := line[i]
	start := i

	for i++; i < len(line); i++ {
		if line[i] == '\\' && quote != '`' {
			paint(out, start, i, palette.StringLit)
			start = min(i+2, len(line))
			paint(out, i, start, palette.Escape)
			i = start - 1

			continue
		}
		if line[i] == quote {
			paint(out, start, i+1, palette.StringLit)

			return i + 1
		}
	}
	paint(out, start, len(line), palette.StringLit)

	return len(line)
}
