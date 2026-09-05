package main

import (
	"path/filepath"
	"testing"

	"github.com/nsf/termbox-go"
)

func TestLeaderSsListsTheDeclarationsOfTheFile(t *testing.T) {
	inDefinition(t, "package main\n\ntype Buffer struct{}\n\nfunc open() {}\n\nfunc (b *Buffer) Line() {}\n", 0, 0, nil)

	press(t, " ss")

	if mode != PickerMode {
		t.Fatalf("mode = %v, want PickerMode", mode)
	}
	wantMatches(t,
		"Struct    Buffer",
		"Function  open",
		"Method    Buffer.Line",
	)
}

func TestLeaderSsListsEveryNameOfAVarBlock(t *testing.T) {
	inDefinition(t, "var (\n\tROWS, COLS int\n\tbuf *Buffer\n)\n\nconst (\n\tReadMode Mode = iota\n\tEditMode\n)\n", 0, 0, nil)

	press(t, " ss")

	wantMatches(t,
		"Variable  ROWS",
		"Variable  COLS",
		"Variable  buf",
		"Constant  ReadMode",
		"Constant  EditMode",
	)
}

func TestLeaderSsLeavesLocalsOut(t *testing.T) {
	inDefinition(t, "func open() {\n\ttotal := 0\n\tfor i := range 3 {\n\t\ttotal += i\n\t}\n}\n", 0, 0, nil)

	press(t, " ss")

	wantMatches(t, "Function  open")
}

func TestLeaderSsReadsALanguageWithoutBraces(t *testing.T) {
	inDefinition(t, "class Parser:\n    def parse(self):\n        pass\n", 0, 0, nil)

	press(t, " ss")

	wantMatches(t,
		"Class     Parser",
		"Function  parse",
	)
}

func TestEnterOnASymbolGoesToItsName(t *testing.T) {
	inDefinition(t, "package main\n\nfunc open() {}\n", 0, 0, nil)

	press(t, " ss")
	pressKey(t, termbox.KeyEnter)

	wantAt(t, 2, 5)
	if pickerOpen {
		t.Error("the popup stayed open")
	}
}

func TestTypingNarrowsTheSymbols(t *testing.T) {
	inDefinition(t, "func open() {}\nfunc close() {}\ntype Reader interface{}\n", 0, 0, nil)

	press(t, " ss")
	press(t, "close")

	wantMatches(t, "Function  close")
}

func TestLeaderSsSaysWhenTheFileDeclaresNothing(t *testing.T) {
	inDefinition(t, "just some prose\nand another line\n", 0, 0, nil)

	press(t, " ss")

	if mode == PickerMode {
		t.Fatalf("the popup opened on %v", matchedLabels())
	}
	if statusMsg != "no symbols in start.go" {
		t.Errorf("statusMsg = %q", statusMsg)
	}
}

func TestLeaderShiftSListsTheSymbolsOfTheFilesBeside(t *testing.T) {
	inDefinition(t, "func open() {}\n", 0, 0, map[string]string{
		"other.go":  "type Reader interface{}\n",
		"notes.txt": "func ignored() {}\n",
	})

	press(t, " sS")

	if mode != PickerMode {
		t.Fatalf("mode = %v, want PickerMode", mode)
	}
	wantMatches(t,
		"Function  open  start.go:1",
		"Interface Reader  other.go:1",
	)
}

func TestEnterOnAWorkspaceSymbolOpensItsFile(t *testing.T) {
	inDefinition(t, "func open() {}\n", 0, 0, map[string]string{"other.go": "package main\n\nfunc target() {}\n"})

	press(t, " sS")
	pressKey(t, termbox.KeyCtrlN)
	pressKey(t, termbox.KeyEnter)

	if got := filepath.Base(sourceFile); got != "other.go" {
		t.Fatalf("editing %q, want other.go", got)
	}
	wantAt(t, 2, 5)
}
