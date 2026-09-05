// Package syntax is every language's lexical surface the editor can colour:
// enough to tell comments, strings, numbers and reserved words apart, and
// nothing more.
package syntax

import (
	"path/filepath"
	"strings"

	"github.com/ArditZubaku/tex/internal/chars"
	"github.com/ArditZubaku/tex/internal/theme"
	"github.com/nsf/termbox-go"
)

// Syntax is one language's lexical surface: enough to tell comments, strings,
// numbers and reserved words apart, and nothing more. Every delimiter is
// ASCII, so a rune of a line matches a byte of a delimiter one for one.
type Syntax struct {
	lineComment string
	blockStart  string
	blockEnd    string
	quotes      string
	keywords    map[string]bool
	types       map[string]bool
	builtins    map[string]bool
	constants   map[string]bool
}

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

func hasPrefixAt(line []rune, i int, prefix string) bool {
	if prefix == "" || i+len(prefix) > len(line) {
		return false
	}
	for _, ch := range prefix {
		if line[i] != ch {
			return false
		}
		i++
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

func (s *Syntax) Highlight(line []rune, inBlock bool, out []termbox.Attribute, palette *theme.Palette) bool {
	for i := 0; i < len(line); {
		switch {
		case inBlock:
			start := i
			i, inBlock = s.scanBlock(line, i)
			paint(out, start, i, palette.Comment)

		case hasPrefixAt(line, i, s.lineComment):
			paint(out, i, len(line), palette.Comment)

			return false

		case hasPrefixAt(line, i, s.blockStart):
			start := i
			i, inBlock = s.scanBlock(line, i+len(s.blockStart))
			paint(out, start, i, palette.Comment)

		case strings.ContainsRune(s.quotes, line[i]):
			i = s.scanString(line, i, out, palette)

		case line[i] >= '0' && line[i] <= '9':
			start := i
			for i < len(line) && (chars.IsWord(line[i]) || line[i] == '.') {
				i++
			}
			paint(out, start, i, palette.Number)

		case chars.IsWord(line[i]):
			start := i
			for i < len(line) && chars.IsWord(line[i]) {
				i++
			}
			paint(out, start, i, s.wordColor(string(line[start:i]), line, i, palette))

		default:
			i++
		}
	}

	return inBlock
}

// A name the language reserves nothing for is still worth colouring when it is
// being called: an open bracket right after it is what tells a call from a
// variable, which is as far as one line of context reaches.
func (s *Syntax) wordColor(word string, line []rune, after int, palette *theme.Palette) termbox.Attribute {
	switch {
	case s.keywords[word]:
		return palette.Keyword
	case s.constants[word]:
		return palette.Constant
	case s.types[word]:
		return palette.TypeName
	case s.builtins[word]:
		return palette.Builtin
	case after < len(line) && line[after] == '(':
		return palette.Function
	default:
		return palette.Plain
	}
}

func (s *Syntax) scanBlock(line []rune, i int) (int, bool) {
	for ; i < len(line); i++ {
		if hasPrefixAt(line, i, s.blockEnd) {
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
