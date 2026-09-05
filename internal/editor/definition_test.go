package editor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ArditZubaku/tex/internal/editor/edtest"
	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/nsf/termbox-go"
)

func inDefinition(t *testing.T, e *state.Editor, content string, row, col int, others map[string]string) {
	t.Helper()

	dir := t.TempDir()
	for name, body := range others {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	edtest.InReadMode(t, e, content, row, col)
	e.SourceFile = filepath.Join(dir, "start.go")
	edtest.SingleWindow(e, 20, 80)
}

func wantAt(t *testing.T, e *state.Editor, row, col int) {
	t.Helper()

	if e.Row != row || e.Col != col {
		t.Errorf("cursor at %d,%d, want %d,%d", e.Row, e.Col, row, col)
	}
}

func TestGdJumpsToTheFunctionUnderTheCursor(t *testing.T) {
	e := state.New()

	inDefinition(t, e, "func target() {}\n\nfunc caller() {\n\ttarget()\n}\n", 3, 1, nil)

	edtest.Press(t, e, "gd")

	wantAt(t, e, 0, 5)
}

func TestGdPrefersADeclarationToAnEarlierMention(t *testing.T) {
	e := state.New()

	inDefinition(t, e, "// target is called below\nvar target = 1\ntarget++\n", 2, 0, nil)

	edtest.Press(t, e, "gd")

	wantAt(t, e, 1, 4)
}

func TestGdTakesTheTypeADeclarationDeclares(t *testing.T) {
	e := state.New()

	inDefinition(t, e, "type window struct{}\n\nvar w window\n", 2, 6, nil)

	edtest.Press(t, e, "gd")

	wantAt(t, e, 0, 5)
}

func TestGdFallsBackToTheFirstMentionOfTheWord(t *testing.T) {
	e := state.New()

	inDefinition(t, e, "one two\n\nsomething two more\n", 2, 10, nil)

	edtest.Press(t, e, "gd")

	wantAt(t, e, 0, 4)
}

func TestGdFindsTheDefinitionInAnotherFileOfTheSameKind(t *testing.T) {
	e := state.New()

	inDefinition(t, e, "helper()\n", 0, 0, map[string]string{
		"other.go":  "package main\n\nfunc helper() {}\n",
		"notes.txt": "func helper() {}\n",
	})

	edtest.Press(t, e, "gd")

	if got := filepath.Base(e.SourceFile); got != "other.go" {
		t.Fatalf("editing %q, want other.go", got)
	}
	wantAt(t, e, 2, 5)
}

func TestGdSaysWhenThereIsNoDefinitionToFind(t *testing.T) {
	e := state.New()

	inDefinition(t, e, "missing()\n", 0, 0, nil)

	edtest.Press(t, e, "gd")

	if e.StatusMsg != "E388: Couldn't find definition of missing" {
		t.Errorf("statusMsg = %q", e.StatusMsg)
	}
	wantAt(t, e, 0, 0)
}

func TestGdSaysWhenThereIsNoWordUnderTheCursor(t *testing.T) {
	e := state.New()

	inDefinition(t, e, "   \n", 0, 0, nil)

	edtest.Press(t, e, "gd")

	if e.StatusMsg != "E349: No identifier under the cursor" {
		t.Errorf("statusMsg = %q", e.StatusMsg)
	}
}

func TestCtrlOGoesBackToWhereTheJumpLeftFrom(t *testing.T) {
	e := state.New()

	inDefinition(t, e, "func target() {}\n\nfunc caller() {\n\ttarget()\n}\n", 3, 1, nil)

	edtest.Press(t, e, "gd")
	edtest.PressKey(t, e, termbox.KeyCtrlO)

	wantAt(t, e, 3, 1)
}

func TestCtrlOComesBackAcrossFiles(t *testing.T) {
	e := state.New()

	inDefinition(t, e, "helper()\n", 0, 0, map[string]string{"other.go": "package main\n\nfunc helper() {}\n"})

	edtest.Press(t, e, "gd")
	edtest.PressKey(t, e, termbox.KeyCtrlO)

	if got := filepath.Base(e.SourceFile); got != "start.go" {
		t.Errorf("editing %q, want start.go", got)
	}
	wantAt(t, e, 0, 0)
}

func TestGdOnAnArgumentTakesTheParameterItWasPassedAs(t *testing.T) {
	e := state.New()

	inDefinition(t, e, "func closeFile(file *os.File) {\n\tfile.Close()\n}\n\nfunc (b *B) Close() {\n\tb.file = nil\n}\n", 1, 1, nil)

	edtest.Press(t, e, "gd")

	wantAt(t, e, 0, 15)
}

func TestGdSkipsAFieldOfSomethingElseOfTheSameName(t *testing.T) {
	e := state.New()

	inDefinition(t, e, "var file = 1\n\nfunc f(b *B) {\n\tb.file = nil\n\tuse(file)\n}\n", 4, 6, nil)

	edtest.Press(t, e, "gd")

	wantAt(t, e, 0, 4)
}

func TestGdTakesTheLocalNearestAboveTheCursor(t *testing.T) {
	e := state.New()

	inDefinition(t, e, "var out = 1\n\nfunc f() {\n\tout := 2\n\tuse(out)\n}\n", 4, 6, nil)

	edtest.Press(t, e, "gd")

	wantAt(t, e, 3, 1)
}
