package find_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nsf/termbox-go"

	"github.com/ArditZubaku/tex/internal/editor/edtest"
	"github.com/ArditZubaku/tex/internal/editor/state"
)

// fakeRg stands in for ripgrep so a test says exactly what was "found" rather
// than depending on a real binary and a real search turning up the same thing.
func fakeRg(t *testing.T, body string) {
	t.Helper()

	dir := t.TempDir()
	script := "#!/bin/sh\n" + body + "\n"
	if err := os.WriteFile(filepath.Join(dir, "rg"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", dir)
}

func TestLeaderSlashOpensAPromptForAPattern(t *testing.T) {
	e := state.New()
	edtest.InReadMode(t, e, "package main\n", 0, 0)

	edtest.Press(t, e, " /")

	if e.Mode != state.PromptMode || e.Prompt.Text() != "Search> " {
		t.Fatalf("mode = %v, prompt = %q, want PromptMode under \"Search> \"", e.Mode, e.Prompt.Text())
	}
}

func TestSearchListsWhatRipgrepFound(t *testing.T) {
	e := state.New()
	inDefinition(t, e, "package main\n", 0, 0, nil)
	fakeRg(t, `
		echo 'start.go:1:1:package main'
		echo 'other.go:2:1:map[string]int{"a": 1}'
	`)

	edtest.Press(t, e, " /main\n")

	if e.Mode != state.PickerMode {
		t.Fatalf("mode = %v, want PickerMode", e.Mode)
	}
	wantMatches(t, e, "start.go:1: package main", `other.go:2: map[string]int{"a": 1}`)
}

// A rune wider than a byte in front of the match is what tells ripgrep's own
// byte column apart from the rune column the cursor actually moves on.
func TestEnterOnAMatchGoesToTheRuneColumnRatherThanTheByteOne(t *testing.T) {
	e := state.New()
	inDefinition(t, e, "pkg π target\n", 0, 0, nil)
	fakeRg(t, `echo 'start.go:1:8:pkg π target'`) // byte 8 is 't', rune 6 is

	edtest.Press(t, e, " /target\n")
	edtest.PressKey(t, e, termbox.KeyEnter)

	wantAt(t, e, 0, 6)
	if e.Pick.Open() {
		t.Error("the popup stayed open")
	}
}

func TestSearchReportsNoMatches(t *testing.T) {
	e := state.New()
	inDefinition(t, e, "package main\n", 0, 0, nil)
	fakeRg(t, `exit 1`)

	edtest.Press(t, e, " /nowhere\n")

	if e.Mode == state.PickerMode {
		t.Fatal("the popup opened with nothing in it")
	}
	if e.StatusMsg != "no matches for nowhere" {
		t.Errorf("statusMsg = %q", e.StatusMsg)
	}
}

func TestSearchReportsRipgrepNotOnThePath(t *testing.T) {
	e := state.New()
	inDefinition(t, e, "package main\n", 0, 0, nil)
	t.Setenv("PATH", t.TempDir()) // empty: nothing named rg is on it

	edtest.Press(t, e, " /main\n")

	if e.StatusMsg != "ripgrep: not found on PATH" {
		t.Errorf("statusMsg = %q", e.StatusMsg)
	}
}

func TestSearchReportsWhatRipgrepComplainedAbout(t *testing.T) {
	e := state.New()
	inDefinition(t, e, "package main\n", 0, 0, nil)
	fakeRg(t, `echo "regex parse error" >&2; exit 2`)

	edtest.Press(t, e, " /(unclosed\n")

	if e.StatusMsg != "ripgrep: regex parse error" {
		t.Errorf("statusMsg = %q", e.StatusMsg)
	}
}

func TestSearchIgnoresAnEmptyPattern(t *testing.T) {
	e := state.New()
	inDefinition(t, e, "package main\n", 0, 0, nil)
	fakeRg(t, `echo 'start.go:1:1:should not run'`)

	edtest.Press(t, e, " /\n")

	if e.Mode != state.ReadMode {
		t.Errorf("mode = %v, want an empty pattern left alone", e.Mode)
	}
}
