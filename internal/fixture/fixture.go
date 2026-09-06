// Package fixture is the synthetic input the benchmarks measure against: a
// source file with enough of every lexical shape to make the highlighter work,
// a plain-text file with the long uniform lines a log has, and a file listing
// for the picker. It exists so every package measures the same corpus.
package fixture

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// GoSource writes a Go file of about lines lines: keywords, calls, numbers,
// strings with escapes, line comments and a two-line block comment, so the
// lexer takes every branch it has.
func GoSource(tb testing.TB, lines int) string {
	tb.Helper()

	return write(tb, "big.go", lines, goLine)
}

func goLine(w *bufio.Writer, i int) {
	switch i % 12 {
	case 0:
		_, _ = fmt.Fprintf(w, "func handler%d(ctx context.Context, req *Request) (*Response, error) {\n", i)
	case 1:
		_, _ = fmt.Fprintf(w, "\tname := fmt.Sprintf(\"row %%d of \\\"%%s\\\"\", %d, req.Table)\n", i)
	case 2:
		_, _ = fmt.Fprintf(w, "\t// looks up row %d and returns it unchanged\n", i)
	case 3:
		_, _ = fmt.Fprintln(w, "\tif err != nil { return nil, fmt.Errorf(\"lookup: %w\", err) }")
	case 4:
		_, _ = fmt.Fprintf(w, "\tfor i := range %d { total += weights[i] * float64(i) * 3.5 }\n", i%64)
	case 5:
		_, _ = fmt.Fprintln(w, "/* a block comment that spans")
	case 6:
		_, _ = fmt.Fprintln(w, "   two lines of prose */")
	case 7:
		_, _ = fmt.Fprintf(w, "\tconst threshold = %d // tuned by hand\n", i)
	case 8:
		_, _ = fmt.Fprintln(w, "\tswitch v := any(req).(type) { case *Request: return v.Response, nil }")
	case 9:
		_, _ = fmt.Fprintln(w, "}")
	case 10:
		_, _ = fmt.Fprintf(w, "\tvar buf [%d]byte // scratch for the ünïcödé path\n", 1+i%32)
	default:
		_, _ = fmt.Fprintln(w)
	}
}

// PlainText is the shape test.txt has: uniform lines of about 120 ASCII
// characters, which is what a log or a data dump looks like to the buffer.
func PlainText(tb testing.TB, lines int) string {
	tb.Helper()

	return write(tb, "big.txt", lines, func(w *bufio.Writer, i int) {
		_, _ = fmt.Fprintf(w, "%07d %s\n", i, strings.Repeat("abcdefghij ", 10))
	})
}

func write(tb testing.TB, name string, lines int, line func(*bufio.Writer, int)) string {
	tb.Helper()

	path := filepath.Join(tb.TempDir(), name)
	f, err := os.Create(path)
	if err != nil {
		tb.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	w := bufio.NewWriterSize(f, 1<<20)
	for i := range lines {
		line(w, i)
	}
	if err := w.Flush(); err != nil {
		tb.Fatal(err)
	}

	return path
}

// Paths is the listing the picker fuzzy-matches over: nested directories with
// repeating name stems, so a query has many near-misses to score.
func Paths(n int) []string {
	stems := []string{"buffer", "render", "syntax", "search", "history", "explorer", "picker", "state"}
	dirs := []string{"internal", "internal/editor", "internal/editor/find", "cmd/tex", "pkg/util", "vendor/x/y"}

	out := make([]string, 0, n)
	for i := range n {
		out = append(out, fmt.Sprintf("%s/%s_%d.go", dirs[i%len(dirs)], stems[i%len(stems)], i))
	}

	return out
}
