package editor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nsf/termbox-go"
)

func inDefinition(t *testing.T, content string, row, col int, others map[string]string) {
	t.Helper()

	dir := t.TempDir()
	for name, body := range others {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	inReadMode(t, content, row, col)
	ed.SourceFile = filepath.Join(dir, "start.go")
	singleWindow(20, 80)
}

func wantAt(t *testing.T, row, col int) {
	t.Helper()

	if ed.Row != row || ed.Col != col {
		t.Errorf("cursor at %d,%d, want %d,%d", ed.Row, ed.Col, row, col)
	}
}

func TestGdJumpsToTheFunctionUnderTheCursor(t *testing.T) {
	inDefinition(t, "func target() {}\n\nfunc caller() {\n\ttarget()\n}\n", 3, 1, nil)

	press(t, "gd")

	wantAt(t, 0, 5)
}

func TestGdPrefersADeclarationToAnEarlierMention(t *testing.T) {
	inDefinition(t, "// target is called below\nvar target = 1\ntarget++\n", 2, 0, nil)

	press(t, "gd")

	wantAt(t, 1, 4)
}

func TestGdTakesTheTypeADeclarationDeclares(t *testing.T) {
	inDefinition(t, "type window struct{}\n\nvar w window\n", 2, 6, nil)

	press(t, "gd")

	wantAt(t, 0, 5)
}

func TestGdFallsBackToTheFirstMentionOfTheWord(t *testing.T) {
	inDefinition(t, "one two\n\nsomething two more\n", 2, 10, nil)

	press(t, "gd")

	wantAt(t, 0, 4)
}

func TestGdFindsTheDefinitionInAnotherFileOfTheSameKind(t *testing.T) {
	inDefinition(t, "helper()\n", 0, 0, map[string]string{
		"other.go":  "package main\n\nfunc helper() {}\n",
		"notes.txt": "func helper() {}\n",
	})

	press(t, "gd")

	if got := filepath.Base(ed.SourceFile); got != "other.go" {
		t.Fatalf("editing %q, want other.go", got)
	}
	wantAt(t, 2, 5)
}

func TestGdSaysWhenThereIsNoDefinitionToFind(t *testing.T) {
	inDefinition(t, "missing()\n", 0, 0, nil)

	press(t, "gd")

	if ed.StatusMsg != "E388: Couldn't find definition of missing" {
		t.Errorf("statusMsg = %q", ed.StatusMsg)
	}
	wantAt(t, 0, 0)
}

func TestGdSaysWhenThereIsNoWordUnderTheCursor(t *testing.T) {
	inDefinition(t, "   \n", 0, 0, nil)

	press(t, "gd")

	if ed.StatusMsg != "E349: No identifier under the cursor" {
		t.Errorf("statusMsg = %q", ed.StatusMsg)
	}
}

func TestCtrlOGoesBackToWhereTheJumpLeftFrom(t *testing.T) {
	inDefinition(t, "func target() {}\n\nfunc caller() {\n\ttarget()\n}\n", 3, 1, nil)

	press(t, "gd")
	pressKey(t, termbox.KeyCtrlO)

	wantAt(t, 3, 1)
}

func TestCtrlOComesBackAcrossFiles(t *testing.T) {
	inDefinition(t, "helper()\n", 0, 0, map[string]string{"other.go": "package main\n\nfunc helper() {}\n"})

	press(t, "gd")
	pressKey(t, termbox.KeyCtrlO)

	if got := filepath.Base(ed.SourceFile); got != "start.go" {
		t.Errorf("editing %q, want start.go", got)
	}
	wantAt(t, 0, 0)
}

func TestGdOnAnArgumentTakesTheParameterItWasPassedAs(t *testing.T) {
	inDefinition(t, "func closeFile(file *os.File) {\n\tfile.Close()\n}\n\nfunc (b *B) Close() {\n\tb.file = nil\n}\n", 1, 1, nil)

	press(t, "gd")

	wantAt(t, 0, 15)
}

func TestGdSkipsAFieldOfSomethingElseOfTheSameName(t *testing.T) {
	inDefinition(t, "var file = 1\n\nfunc f(b *B) {\n\tb.file = nil\n\tuse(file)\n}\n", 4, 6, nil)

	press(t, "gd")

	wantAt(t, 0, 4)
}

func TestGdTakesTheLocalNearestAboveTheCursor(t *testing.T) {
	inDefinition(t, "var out = 1\n\nfunc f() {\n\tout := 2\n\tuse(out)\n}\n", 4, 6, nil)

	press(t, "gd")

	wantAt(t, 3, 1)
}
