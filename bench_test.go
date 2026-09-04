package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// goFile is a stand-in for the source files the editor is actually used on:
// long enough to scroll through, and with the keywords, strings and comments
// that make the highlighter do work.
func goFile(tb testing.TB, lines int) string {
	tb.Helper()

	path := filepath.Join(tb.TempDir(), "big.go")
	f, err := os.Create(path)
	if err != nil {
		tb.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	w := bufio.NewWriter(f)
	_, _ = fmt.Fprintln(w, "package main")
	for i := range lines {
		switch i % 11 {
		case 0:
			_, _ = fmt.Fprintf(w, "func handler%d(ctx context.Context, req *Request) (*Response, error) {\n", i)
		case 1:
			_, _ = fmt.Fprintf(w, "\tname := fmt.Sprintf(\"row %%d of %%s\", %d, req.Table)\n", i)
		case 2:
			_, _ = fmt.Fprintf(w, "\t// looks up row %d and returns it unchanged\n", i)
		case 3:
			_, _ = fmt.Fprintln(w, "\tif err != nil { return nil, fmt.Errorf(\"lookup: %w\", err) }")
		case 4:
			_, _ = fmt.Fprintf(w, "\tfor i := range %d { total += weights[i] * float64(i) }\n", i%64)
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
		default:
			_, _ = fmt.Fprintln(w)
		}
	}
	if err := w.Flush(); err != nil {
		tb.Fatal(err)
	}

	return path
}

// openEditor puts the globals in the state runEditor leaves them in for a file
// filling one screen, which is what every render and motion benchmark needs.
func openEditor(tb testing.TB, path string) {
	tb.Helper()

	if buf != nil {
		buf.Close()
	}
	buf = openBuffer(path)
	sourceFile = path
	syntax = detectSyntax(path)
	ROWS, COLS = 48, 160
	screenRows, screenCols = ROWS, COLS
	offsetRow, offsetCol = 0, 0
	currentRow, currentCol = 0, 0
	mode = ReadMode
	hlSearch = false
	tb.Cleanup(func() {
		buf.Close()
		buf, syntax, root, current = nil, nil, nil, nil
	})
}

func BenchmarkOpenBuffer(b *testing.B) {
	path := goFile(b, 200000)

	for b.Loop() {
		bb := openBuffer(path)
		bb.Close()
	}
}

func BenchmarkRenderFrame(b *testing.B) {
	openEditor(b, goFile(b, 20000))

	for b.Loop() {
		displayTextBuffer()
	}
}

// BenchmarkScrollScreens is the closest thing to a session: hold <C-d> down
// and redraw every screenful on the way, so window refills, the line index and
// the highlighter all take their share.
func BenchmarkScrollScreens(b *testing.B) {
	openEditor(b, goFile(b, 20000))

	for b.Loop() {
		currentRow, offsetRow = 0, 0
		for currentRow+ROWS < buf.LineCount() {
			pageDown()
			scrollTextBuffer()
			displayTextBuffer()
		}
	}
}

func BenchmarkHighlightLines(b *testing.B) {
	openEditor(b, goFile(b, 20000))
	lines := make([][]rune, 0, 4096)
	for i := range 4096 {
		lines = append(lines, buf.Line(i))
	}

	for b.Loop() {
		inBlock := false
		for _, line := range lines {
			_, inBlock = lineColors(line, inBlock)
		}
	}
}

func BenchmarkSearchWholeBuffer(b *testing.B) {
	openEditor(b, goFile(b, 20000))
	pat := newPattern([]rune("Errorf"))

	for b.Loop() {
		var cols []int
		for row := range buf.LineCount() {
			cols = pat.matchesIn(row, cols[:0])
		}
	}
}

func BenchmarkFindMatch(b *testing.B) {
	openEditor(b, goFile(b, 20000))
	pat := newPattern([]rune("threshold"))

	for b.Loop() {
		row, col := 0, 0
		for range 500 {
			r, c, ok := findMatch(pat, row, col, false)
			if !ok {
				break
			}
			row, col = r, c
		}
	}
}

func BenchmarkWordMotions(b *testing.B) {
	openEditor(b, goFile(b, 20000))

	for b.Loop() {
		row, col := 0, 0
		for range 20000 {
			row, col = nextWordFrom(row, col)
		}
		for range 20000 {
			row, col = prevWordFrom(row, col)
		}
	}
}
