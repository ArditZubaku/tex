package editor

import (
	"path/filepath"
	"testing"

	"github.com/ArditZubaku/tex/internal/editor/edtest"
	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/nsf/termbox-go"
)

func TestLeaderSsListsTheDeclarationsOfTheFile(t *testing.T) {
	e := state.New()

	inDefinition(e, t, "package main\n\ntype Buffer struct{}\n\nfunc open() {}\n\nfunc (b *Buffer) Line() {}\n", 0, 0, nil)

	edtest.Press(t, e, " ss")

	if e.Mode != state.PickerMode {
		t.Fatalf("mode = %v, want PickerMode", e.Mode)
	}
	wantMatches(e, t,
		"Struct    Buffer",
		"Function  open",
		"Method    Buffer.Line",
	)
}

func TestLeaderSsListsEveryNameOfAVarBlock(t *testing.T) {
	e := state.New()

	inDefinition(e, t, "var (\n\tROWS, COLS int\n\tbuf *Buffer\n)\n\nconst (\n\tReadMode Mode = iota\n\tEditMode\n)\n", 0, 0, nil)

	edtest.Press(t, e, " ss")

	wantMatches(e, t,
		"Variable  ROWS",
		"Variable  COLS",
		"Variable  buf",
		"Constant  ReadMode",
		"Constant  EditMode",
	)
}

func TestLeaderSsLeavesLocalsOut(t *testing.T) {
	e := state.New()

	inDefinition(e, t, "func open() {\n\ttotal := 0\n\tfor i := range 3 {\n\t\ttotal += i\n\t}\n}\n", 0, 0, nil)

	edtest.Press(t, e, " ss")

	wantMatches(e, t, "Function  open")
}

func TestLeaderSsReadsALanguageWithoutBraces(t *testing.T) {
	e := state.New()

	inDefinition(e, t, "class Parser:\n    def parse(self):\n        pass\n", 0, 0, nil)

	edtest.Press(t, e, " ss")

	wantMatches(e, t,
		"Class     Parser",
		"Function  parse",
	)
}

func TestEnterOnASymbolGoesToItsName(t *testing.T) {
	e := state.New()

	inDefinition(e, t, "package main\n\nfunc open() {}\n", 0, 0, nil)

	edtest.Press(t, e, " ss")
	edtest.PressKey(t, e, termbox.KeyEnter)

	wantAt(e, t, 2, 5)
	if e.Pick.Open() {
		t.Error("the popup stayed open")
	}
}

func TestTypingNarrowsTheSymbols(t *testing.T) {
	e := state.New()

	inDefinition(e, t, "func open() {}\nfunc close() {}\ntype Reader interface{}\n", 0, 0, nil)

	edtest.Press(t, e, " ss")
	edtest.Press(t, e, "close")

	wantMatches(e, t, "Function  close")
}

func TestLeaderSsSaysWhenTheFileDeclaresNothing(t *testing.T) {
	e := state.New()

	inDefinition(e, t, "just some prose\nand another line\n", 0, 0, nil)

	edtest.Press(t, e, " ss")

	if e.Mode == state.PickerMode {
		t.Fatalf("the popup opened on %v", matchedLabels(e))
	}
	if e.StatusMsg != "no symbols in start.go" {
		t.Errorf("statusMsg = %q", e.StatusMsg)
	}
}

func TestLeaderShiftSListsTheSymbolsOfTheFilesBeside(t *testing.T) {
	e := state.New()

	inDefinition(e, t, "func open() {}\n", 0, 0, map[string]string{
		"other.go":  "type Reader interface{}\n",
		"notes.txt": "func ignored() {}\n",
	})

	edtest.Press(t, e, " sS")

	if e.Mode != state.PickerMode {
		t.Fatalf("mode = %v, want PickerMode", e.Mode)
	}
	wantMatches(e, t,
		"Function  open  start.go:1",
		"Interface Reader  other.go:1",
	)
}

func TestEnterOnAWorkspaceSymbolOpensItsFile(t *testing.T) {
	e := state.New()

	inDefinition(e, t, "func open() {}\n", 0, 0, map[string]string{"other.go": "package main\n\nfunc target() {}\n"})

	edtest.Press(t, e, " sS")
	edtest.PressKey(t, e, termbox.KeyCtrlN)
	edtest.PressKey(t, e, termbox.KeyEnter)

	if got := filepath.Base(e.SourceFile); got != "other.go" {
		t.Fatalf("editing %q, want other.go", got)
	}
	wantAt(e, t, 2, 5)
}
