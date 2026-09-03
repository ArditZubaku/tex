package main

import (
	"strings"
	"testing"

	"github.com/nsf/termbox-go"
)

// A mask spells out one character per rune of the line: k keyword, l literal
// constant, t type, e escape, f function, b builtin, s string, n number,
// c comment, . uncoloured.
func mask(t *testing.T, s *Syntax, line string, inBlock bool) (string, bool) {
	t.Helper()

	active = defaultTheme

	runes := []rune(line)
	out := make([]termbox.Attribute, len(runes))
	for i := range out {
		out[i] = active.plain
	}

	open := s.highlight(runes, inBlock, out)

	var b strings.Builder
	for _, color := range out {
		switch color {
		case active.keyword:
			b.WriteByte('k')
		case active.constant:
			b.WriteByte('l')
		case active.typeName:
			b.WriteByte('t')
		case active.escape:
			b.WriteByte('e')
		case active.function:
			b.WriteByte('f')
		case active.builtin:
			b.WriteByte('b')
		case active.stringLit:
			b.WriteByte('s')
		case active.number:
			b.WriteByte('n')
		case active.comment:
			b.WriteByte('c')
		default:
			b.WriteByte('.')
		}
	}

	return b.String(), open
}

func TestHighlightGo(t *testing.T) {
	cases := []struct {
		name string
		line string
		want string
	}{
		{"keywords and types", "var x int = 42", "kkk...ttt...nn"},
		{"a word is not a keyword by prefix", "myvar varx", ".........."},
		{"literal constants", "return nil, true", "kkkkkk.lll..llll"},
		{"a called name", "greet(x)", "fffff..."},
		{"a name that is not called", "greet x", "......."},
		{"a builtin outranks the call colour", "len(xs)", "bbb...."},
		{"a method call, not its receiver", "os.Exit(1)", "...ffff.n."},
		{"strings", `p("hi")`, "f.ssss."},
		{"a comment marker inside a string", `p("// no")`, "f.sssssss."},
		{"a quote inside a comment", "// it's fine", "cccccccccccc"},
		{"a comment after code", "x++ // why", "....cccccc"},
		{"an escaped quote does not end the string", `"a\"b" 1`, "sseess.n"},
		{"an escape sequence inside a string", `"a\nb"`, "sseess"},
		{"a trailing backslash", `"a\`, "sse"},
		{"an unterminated string stops at the line end", `s := "abc`, ".....ssss"},
		{"a raw string keeps its backslashes", "`a\\` 1", "ssss.n"},
		{"a one-line block comment", "a /* b */ c", "..ccccccc.."},
		{"floats and hex", "1.5 0xff", "nnn.nnnn"},
		{"a number glued to a word is left alone", "x1 = 2", ".....n"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, open := mask(t, goSyntax, tc.line, false)
			if got != tc.want {
				t.Errorf("highlight(%q)\ngot  %s\nwant %s", tc.line, got, tc.want)
			}
			if open {
				t.Errorf("highlight(%q) left a block comment open", tc.line)
			}
		})
	}
}

func TestHighlightBlockComments(t *testing.T) {
	cases := []struct {
		name     string
		line     string
		inBlock  bool
		want     string
		wantOpen bool
	}{
		{"opening one runs to the end of the line", "x /* start", false, "..cccccccc", true},
		{"a middle line is all comment", "still going", true, "ccccccccccc", true},
		{"closing one hands the rest back", "done */ var", true, "ccccccc.kkk", false},
		{"a keyword inside is not highlighted", "var x", true, "ccccc", true},
		{"a quote inside does not open a string", `it's fine`, true, "ccccccccc", true},
		{"a marker inside a string does not open one", `s := "/*"`, false, ".....ssss", false},
		{"an empty line keeps the state", "", true, "", true},
		{"/*/ does not close itself", "/*/ x", false, "ccccc", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, open := mask(t, goSyntax, tc.line, tc.inBlock)
			if got != tc.want {
				t.Errorf("highlight(%q, inBlock=%v)\ngot  %s\nwant %s", tc.line, tc.inBlock, got, tc.want)
			}
			if open != tc.wantOpen {
				t.Errorf("highlight(%q, inBlock=%v) left open = %v, want %v", tc.line, tc.inBlock, open, tc.wantOpen)
			}
		})
	}
}

func TestHighlightHashLanguages(t *testing.T) {
	cases := []struct {
		name string
		line string
		want string
	}{
		{"a hash comment", "x = 1  # why", "....n..ccccc"},
		{"a hash inside a string", `p("#1")`, "f.ssss."},
		{"python keywords and builtins", "def f(): return len(x)", "kkk.f....kkkkkk.bbb..."},
		{"python constants", "self.x = None", "llll.....llll"},
		{"a slash pair is not a comment here", "a // b", "......"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, _ := mask(t, hashSyntax, tc.line, false)
			if got != tc.want {
				t.Errorf("highlight(%q)\ngot  %s\nwant %s", tc.line, got, tc.want)
			}
		})
	}
}

func TestDetectSyntax(t *testing.T) {
	cases := []struct {
		name string
		file string
		want *Syntax
	}{
		{"go by extension", "main.go", goSyntax},
		{"c family by extension", "src/App.tsx", cSyntax},
		{"an uppercase extension still matches", "SCRIPT.PY", hashSyntax},
		{"a bare name with no extension", "/etc/Makefile", hashSyntax},
		{"a dotfile", "/home/me/.zshrc", hashSyntax},
		{"an unknown extension has no syntax", "notes.txt", nil},
		{"a name with no extension at all", "LICENSE", nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := detectSyntax(tc.file); got != tc.want {
				t.Errorf("detectSyntax(%q) = %v, want %v", tc.file, got, tc.want)
			}
		})
	}
}

func TestLineColorsWithoutSyntax(t *testing.T) {
	syntax = nil
	defer func() { syntax = nil }()

	got, open := lineColors([]rune("var x int"), true)
	if got != nil || open {
		t.Errorf("lineColors with no syntax = %v, %v, want nil, false", got, open)
	}
}

func TestBlockStateBefore(t *testing.T) {
	syntax = goSyntax
	defer func() { syntax = nil }()

	buf = openBuffer(writeTemp(t, "a\n/* open\nstill\n*/ b\nc\n"))
	defer buf.Close()

	cases := []struct {
		name string
		row  int
		want bool
	}{
		{"the first line starts outside", 0, false},
		{"the line after the opener is inside", 2, true},
		{"the closing line is still inside", 3, true},
		{"the line after the closer is outside", 4, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := blockStateBefore(tc.row); got != tc.want {
				t.Errorf("blockStateBefore(%d) = %v, want %v", tc.row, got, tc.want)
			}
		})
	}
}

// Past the look-back bound the window is assumed to start outside a comment,
// which is what keeps a redraw from lexing the whole file.
func TestBlockStateBeforeIsBounded(t *testing.T) {
	syntax = goSyntax
	defer func() { syntax = nil }()

	lines := "/* open\n" + strings.Repeat("still\n", blockLookback+10)
	buf = openBuffer(writeTemp(t, lines))
	defer buf.Close()

	if !blockStateBefore(blockLookback) {
		t.Error("blockStateBefore within the bound = false, want true")
	}
	if blockStateBefore(blockLookback + 2) {
		t.Error("blockStateBefore past the bound = true, want false")
	}
}
