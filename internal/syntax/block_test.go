package syntax

import (
	"bufio"
	"os"
	"testing"

	"github.com/ArditZubaku/tex/internal/fixture"
	"github.com/ArditZubaku/tex/internal/theme"
)

var trickyLines = []string{
	"",
	"package main",
	"a := b / c",
	"x *= 2",
	"p := *q",
	"// a line comment with /* inside it",
	"/* opens here",
	"still inside",
	"closes here */ and code after",
	"/* opens and closes */",
	`s := "*/"`,
	`s := "/*"`,
	"s := `a raw /* string`",
	"s := '*'",
	`escaped := "\"/*\""`,
	"/*/",
	"*/",
	"/",
	"*",
	"nested /* one /* two */",
	"ünïcödé /* and a comment */",
}

func lineSet(tb testing.TB) []string {
	tb.Helper()

	f, err := os.Open(fixture.GoSource(tb, 400))
	if err != nil {
		tb.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	lines := append([]string(nil), trickyLines...)
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	if err := sc.Err(); err != nil {
		tb.Fatal(err)
	}

	return lines
}

// The look-back skips a line CanChangeBlock rules out, so ruling one out has to
// mean the lexer would have left the state exactly as it found it.
func TestCanChangeBlockRulesOutOnlyLinesThatDoNot(t *testing.T) {
	palette := theme.Default()

	for _, line := range lineSet(t) {
		for _, inBlock := range []bool{false, true} {
			if goSyntax.CanChangeBlock([]byte(line), inBlock) {
				continue
			}
			if got := goSyntax.Highlight([]rune(line), inBlock, nil, &palette); got != inBlock {
				t.Errorf("CanChangeBlock(%q, %v) = false, but the line leaves it %v", line, inBlock, got)
			}
		}
	}
}

func TestCanChangeBlockNeedsBlockComments(t *testing.T) {
	if hashSyntax.CanChangeBlock([]byte("/* not a comment here */"), false) {
		t.Error("a language with no block comments reported it could open one")
	}

	var none *Syntax
	if none.CanChangeBlock([]byte("/*"), false) {
		t.Error("a file of no known language reported it could open a block")
	}
}
