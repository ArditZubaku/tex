package preview

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/nsf/termbox-go"
)

// fakeGlow puts a program named "glow" on the PATH, running the shell body it
// is given with its arguments left in "$@".
func fakeGlow(t *testing.T, body string) {
	t.Helper()

	dir := t.TempDir()
	script := "#!/bin/sh\n" + body + "\n"
	if err := os.WriteFile(filepath.Join(dir, "glow"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", dir)
	Reset()
}

func sourceFile(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "a.md")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	return path
}

func TestIsMarkdownRecognisesTheCommonExtensions(t *testing.T) {
	for _, path := range []string{"a.md", "a.MD", "a.markdown", "a.mdown", "a.mkd"} {
		if !IsMarkdown(path) {
			t.Errorf("IsMarkdown(%q) = false, want true", path)
		}
	}
	for _, path := range []string{"a.txt", "a.go", "a"} {
		if IsMarkdown(path) {
			t.Errorf("IsMarkdown(%q) = true, want false", path)
		}
	}
}

func TestAvailableIsFalseWithoutGlowOnThePath(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	Reset()

	if Available() {
		t.Error("Available() = true, want false with an empty PATH")
	}
}

func TestAvailableIsTrueOnceGlowIsFound(t *testing.T) {
	fakeGlow(t, ":")

	if !Available() {
		t.Error("Available() = false, want true with glow on the PATH")
	}
}

func TestTheLookupIsOnlyDoneOnce(t *testing.T) {
	fakeGlow(t, ":")
	Available()

	// with the PATH emptied the answer can only come from what the first look left
	t.Setenv("PATH", t.TempDir())
	if !Available() {
		t.Error("Available() = false, want the looked-up glow remembered")
	}
}

func TestRenderRunsGlowWithTheStyleAndWidth(t *testing.T) {
	fakeGlow(t, `printf '%s\n' "$*"`)
	path := sourceFile(t, "# hi\n")

	rows, err := Render(path, 40)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want the one line glow printed", len(rows))
	}
	got := textOf(rows[0])
	want := "-s dark -w 40 " + path
	if got != want {
		t.Errorf("glow ran with %q, want %q", got, want)
	}
}

func TestRenderNeverInheritsTheEditorsStdin(t *testing.T) {
	// a glow that reads stdin would hang here forever if exec.Cmd inherited
	// the caller's own, since nothing is ever written to it
	fakeGlow(t, `read line; printf 'read: %s\n' "$line"`)
	path := sourceFile(t, "# hi\n")

	rows, err := Render(path, 40)
	if err != nil {
		t.Fatal(err)
	}
	if got := textOf(rows[0]); got != "read: " {
		t.Errorf("glow read %q from a closed stdin, want an empty line", got)
	}
}

func TestRenderReportsGlowNotBeingInstalled(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	Reset()

	_, err := Render(sourceFile(t, "# hi\n"), 40)

	var refused *Error
	if !errors.As(err, &refused) {
		t.Fatalf("err = %v, want a preview.Error", err)
	}
}

func TestRenderReportsWhatGlowRefused(t *testing.T) {
	fakeGlow(t, `echo "no such file" >&2; exit 1`)

	_, err := Render(sourceFile(t, "# hi\n"), 40)

	var refused *Error
	if !errors.As(err, &refused) || refused.Reason != "no such file" {
		t.Fatalf("err = %v, want glow's own first line", err)
	}
}

func TestParseReadsAPlainRowWithNoEscapesAtAll(t *testing.T) {
	fakeGlow(t, `printf '# hi\n'`)
	path := sourceFile(t, "# hi\n")

	rows, err := Render(path, 40)
	if err != nil {
		t.Fatal(err)
	}
	if got := textOf(rows[0]); got != "# hi" {
		t.Errorf("row = %q, want %q", got, "# hi")
	}
	for _, c := range rows[0] {
		if c.Fg != 0 || c.Bg != 0 {
			t.Fatalf("cell %+v, want the default colours with no SGR at all", c)
		}
	}
}

func TestParseAppliesA256ColourPerGlyph(t *testing.T) {
	fakeGlow(t, `printf '\033[38;5;142mA\033[0m\033[38;5;208mB\033[0m\n'`)
	path := sourceFile(t, "AB\n")

	rows, err := Render(path, 40)
	if err != nil {
		t.Fatal(err)
	}
	row := rows[0]
	if len(row) != 2 {
		t.Fatalf("row = %+v, want two cells", row)
	}
	if row[0].Ch != 'A' || row[0].Fg != termbox.Attribute(143) {
		t.Errorf("cell 0 = %+v, want 'A' at 256-colour index 142", row[0])
	}
	if row[1].Ch != 'B' || row[1].Fg != termbox.Attribute(209) {
		t.Errorf("cell 1 = %+v, want 'B' at 256-colour index 208", row[1])
	}
}

func TestParseKeepsBoldAcrossADefaultColourReset(t *testing.T) {
	// glow's own default-foreground code, 39, rather than a bare reset: it
	// should drop the colour without also dropping the bold that came with it
	fakeGlow(t, `printf '\033[1;38;5;214mA\033[39mB\033[0m\n'`)
	path := sourceFile(t, "AB\n")

	rows, err := Render(path, 40)
	if err != nil {
		t.Fatal(err)
	}
	row := rows[0]
	if row[1].Fg&termbox.AttrBold == 0 {
		t.Errorf("cell 1 = %+v, want bold to survive resetting only the colour", row[1])
	}
	if row[1].Fg&colorMask != 0 {
		t.Errorf("cell 1 = %+v, want no colour left after code 39", row[1])
	}
}

func TestParseReadsABackgroundPill(t *testing.T) {
	fakeGlow(t, `printf '\033[38;5;235;48;5;214mOK\033[0m\n'`)
	path := sourceFile(t, "OK\n")

	rows, err := Render(path, 40)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range rows[0] {
		if c.Bg != termbox.Attribute(215) {
			t.Errorf("cell %+v, want background at 256-colour index 214", c)
		}
	}
}

func TestParseDecodesMultiByteUTF8(t *testing.T) {
	fakeGlow(t, `printf '\033[38;5;214m•\033[0m bullet\n'`)
	path := sourceFile(t, "- bullet\n")

	rows, err := Render(path, 40)
	if err != nil {
		t.Fatal(err)
	}
	if got := textOf(rows[0]); got != "• bullet" {
		t.Errorf("row = %q, want the bullet decoded as one rune", got)
	}
}

func TestParseSplitsOnEveryLine(t *testing.T) {
	fakeGlow(t, `printf 'one\ntwo\nthree\n'`)
	path := sourceFile(t, "one\ntwo\nthree\n")

	rows, err := Render(path, 40)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(rows))
	}
	for i, want := range []string{"one", "two", "three"} {
		if got := textOf(rows[i]); got != want {
			t.Errorf("row %d = %q, want %q", i, got, want)
		}
	}
}

func textOf(row []Cell) string {
	runes := make([]rune, len(row))
	for i, c := range row {
		runes[i] = c.Ch
	}

	return string(runes)
}
