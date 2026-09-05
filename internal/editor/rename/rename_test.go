// The tests sit outside the package because the harness they share with the
// rest of the editor imports it.
package rename_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ArditZubaku/tex/internal/buffer"
	"github.com/ArditZubaku/tex/internal/editor/edtest"
	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/ArditZubaku/tex/internal/editor/view"
	"github.com/ArditZubaku/tex/internal/syntax"
)

const source = "package start\n\nfunc target() int {\n\treturn target_id\n}\n\ntarget()\n"

// inProject is the file being edited with files of its own kind beside it,
// which is the reach a rename has past its own buffer.
func inProject(t *testing.T, e *state.Editor, content string, row, col int, others map[string]string) (*buffer.Buffer, string) {
	t.Helper()

	dir := t.TempDir()
	for name, body := range others {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	b := edtest.InReadMode(t, e, content, row, col)
	e.SourceFile = filepath.Join(dir, "start.go")
	e.Lang = syntax.Detect(e.SourceFile)
	edtest.SingleWindow(e, 20, 80)

	return b, dir
}

func opened(t *testing.T, name string) *view.Entry {
	t.Helper()

	for _, entry := range view.Buffers() {
		if filepath.Base(entry.Path) == name {
			return entry
		}
	}
	t.Fatalf("%s is not open as a buffer", name)

	return nil
}

func lines(t *testing.T, entry *view.Entry) string {
	t.Helper()

	return strings.Join(edtest.Lines(t, entry.Buf), "\n")
}

func readFile(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	return string(content)
}

func wantStatus(t *testing.T, e *state.Editor, want string) {
	t.Helper()

	if e.StatusMsg != want {
		t.Errorf("status = %q, want %q", e.StatusMsg, want)
	}
}

func TestLeaderCrRenamesEveryMentionInTheBuffer(t *testing.T) {
	e := state.New()

	b := edtest.InReadMode(t, e, source, 2, 5)
	edtest.Press(t, e, " cr")
	edtest.Press(t, e, "\b\b\b\b\b\bhandle\n")

	edtest.WantLines(t, b, "package start", "", "func handle() int {", "\treturn target_id", "}", "", "handle()")
	if !e.Modified {
		t.Error("buffer not marked modified after a rename")
	}
}

func TestLeaderCrOpensThePromptOnTheNameUnderTheCursor(t *testing.T) {
	e := state.New()

	edtest.InReadMode(t, e, source, 2, 5)
	edtest.Press(t, e, " cr")

	if e.Mode != state.PromptMode {
		t.Errorf("mode = %d, want PromptMode", e.Mode)
	}
	if got, want := e.Prompt.Text(), ":rename target"; got != want {
		t.Errorf("prompt = %q, want %q", got, want)
	}
}

func TestRenameLeavesTheWordsMerelyHoldingTheName(t *testing.T) {
	e := state.New()

	b, _ := inProject(t, e, "package start\n\nfile = filename + myfile\n", 2, 0, nil)
	edtest.Press(t, e, ":rename path\n")

	edtest.WantLines(t, b, "package start", "", "path = filename + myfile")
}

func TestRenameLeavesTheCommentsAndStringsThatNameItAlone(t *testing.T) {
	e := state.New()

	b, _ := inProject(t, e,
		"package start\n\nvar file = 1\n\nfunc use() {\n\t// file is read here\n\tlog(\"file\", file) // not file\n}\n",
		2, 4, nil)

	edtest.Press(t, e, ":rename path\n")

	edtest.WantLines(t, b, "package start", "", "var path = 1", "", "func use() {",
		"\t// file is read here", "\tlog(\"file\", path) // not file", "}")
}

func TestRenameTakesTheDocCommentOverTheDeclarationWithIt(t *testing.T) {
	e := state.New()

	b, _ := inProject(t, e,
		"package start\n\n// target is what this file is about.\n// target again, still the doc comment.\nfunc target() {}\n\n// target elsewhere is prose.\nvar other = 1\n",
		4, 5, nil)

	edtest.Press(t, e, ":rename handle\n")

	edtest.WantLines(t, b, "package start", "", "// handle is what this file is about.",
		"// handle again, still the doc comment.", "func handle() {}", "",
		"// target elsewhere is prose.", "var other = 1")
}

func TestRenameTakesTheMentionTheCursorIsOnWhereverItIs(t *testing.T) {
	e := state.New()

	b, _ := inProject(t, e, "package start\n\n// target is named here\nfunc other() {}\n", 2, 3, nil)

	edtest.Press(t, e, ":rename handle\n")

	edtest.WantLines(t, b, "package start", "", "// handle is named here", "func other() {}")
}

func TestRenameIsOneUndoStep(t *testing.T) {
	e := state.New()

	b := edtest.InReadMode(t, e, source, 2, 5)
	edtest.Press(t, e, ":rename handle\n")
	edtest.Press(t, e, "u")

	edtest.WantLines(t, b, strings.Split(strings.TrimSuffix(source, "\n"), "\n")...)
}

func TestRenameKeepsTheCursorOnTheNameItWasOn(t *testing.T) {
	e := state.New()

	edtest.InReadMode(t, e, "id, id, id = one(id)\n", 0, 8)
	edtest.Press(t, e, ":rename index\n")

	edtest.WantCursor(t, e, 0, 14)
}

func TestRenameReportsWhatItChanged(t *testing.T) {
	e := state.New()

	edtest.InReadMode(t, e, source, 2, 5)
	edtest.Press(t, e, ":rename handle\n")

	wantStatus(t, e, "renamed target to handle: 2 changes on 2 lines")
}

func TestRenameRefusesANameThatIsNotAnIdentifier(t *testing.T) {
	e := state.New()

	b := edtest.InReadMode(t, e, source, 2, 5)
	edtest.Press(t, e, ":rename two words\n")

	edtest.WantLines(t, b, strings.Split(strings.TrimSuffix(source, "\n"), "\n")...)
	wantStatus(t, e, "E474: Invalid argument: two words")
	if e.Modified {
		t.Error("buffer marked modified by a refused rename")
	}
}

func TestRenameRefusesToStartOnNoIdentifier(t *testing.T) {
	e := state.New()

	edtest.InReadMode(t, e, "func target() int {\n", 0, 4)
	edtest.Press(t, e, " cr")

	wantStatus(t, e, "E349: No identifier under the cursor")
	if e.Mode != state.ReadMode {
		t.Errorf("mode = %d, want ReadMode", e.Mode)
	}
}

func TestRenameWithoutANameSaysSo(t *testing.T) {
	e := state.New()

	edtest.InReadMode(t, e, source, 2, 5)
	edtest.Press(t, e, ":rename\n")

	wantStatus(t, e, "E471: Argument required")
}

func TestRenameToTheNameItAlreadyHasChangesNothing(t *testing.T) {
	e := state.New()

	edtest.InReadMode(t, e, source, 2, 5)
	edtest.Press(t, e, ":rename target\n")

	wantStatus(t, e, "already named target")
	if e.Modified {
		t.Error("buffer marked modified by a rename to the same name")
	}
}

func TestRenameReachesTheFilesBesideTheOneBeingEdited(t *testing.T) {
	e := state.New()

	inProject(t, e, source, 2, 5, map[string]string{
		"caller.go": "// Package start is a doc comment above the header.\npackage start\n\nfunc caller() {\n\ttarget()\n}\n",
		"notes.txt": "target()\n",
	})

	edtest.Press(t, e, ":rename handle\n")

	if got, want := lines(t, opened(t, "caller.go")),
		"// Package start is a doc comment above the header.\npackage start\n\nfunc caller() {\n\thandle()\n}"; got != want {
		t.Errorf("caller.go = %q, want %q", got, want)
	}
	wantStatus(t, e, "renamed target to handle: 3 changes in 2 files, none written (:wa)")
}

func TestRenameLeavesTheFilesItReachedUnsavedUntilWriteAll(t *testing.T) {
	e := state.New()

	_, dir := inProject(t, e, source, 2, 5, map[string]string{"caller.go": "package start\n\ntarget()\n"})
	beside := filepath.Join(dir, "caller.go")

	edtest.Press(t, e, ":rename handle\n")

	if !opened(t, "caller.go").Modified {
		t.Error("the file the rename reached is not marked modified")
	}
	if got := readFile(t, beside); got != "package start\n\ntarget()\n" {
		t.Errorf("caller.go on disk = %q, want it left alone until it is written", got)
	}

	edtest.Press(t, e, ":wa\n")

	if got, want := readFile(t, beside), "package start\n\nhandle()\n"; got != want {
		t.Errorf("caller.go on disk = %q, want %q", got, want)
	}
	if opened(t, "caller.go").Modified {
		t.Error("the file is still marked modified after ':wa'")
	}
}

func TestRenameIsOneUndoStepInEachFileItReached(t *testing.T) {
	e := state.New()

	inProject(t, e, source, 2, 5, map[string]string{"caller.go": "package start\n\ntarget()\ntarget()\n"})

	edtest.Press(t, e, ":rename handle\n")
	edtest.Press(t, e, "L")
	edtest.Press(t, e, "u")

	edtest.WantCurrent(t, e, "caller.go")
	if got, want := lines(t, opened(t, "caller.go")), "package start\n\ntarget()\ntarget()"; got != want {
		t.Errorf("caller.go = %q, want %q", got, want)
	}
}

func TestRenameLeavesTheFilesThatNeverMentionTheNameClosed(t *testing.T) {
	e := state.New()

	_, dir := inProject(t, e, source, 2, 5, map[string]string{"other.go": "package start\n\nfunc other() {}\n"})

	edtest.Press(t, e, ":rename handle\n")

	if view.Buffer(filepath.Join(dir, "other.go")) != nil {
		t.Error("a file that never mentions the name was opened by the rename")
	}
}

func TestRenameFollowsThePackageTheNameBelongsToRatherThanTheSpelling(t *testing.T) {
	e := state.New()

	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o750); err != nil {
		t.Fatal(err)
	}
	write := func(rel, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Join(dir, filepath.Dir(rel)), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, rel), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("main.go", "package main\n\nfunc main() {\n\teditor.Run(os.Args)\n\tcommand.Run(e, line)\n}\n")
	write("internal/editor/editor.go", "package editor\n\nfunc Run(args []string) {}\n")
	write("internal/editor/command/command_test.go",
		"package command_test\n\nfunc TestRun(t *testing.T) {\n\tcommand.Run(e, \":w\")\n}\n")

	edtest.InReadMode(t, e, "package command\n\nfunc Run(e *state.Editor, line string) {\n\tRun(e, line)\n}\n", 2, 5)
	e.SourceFile = filepath.Join(dir, "internal", "editor", "command", "command.go")
	edtest.SingleWindow(e, 20, 80)

	edtest.Press(t, e, ":rename Execute\n")

	if got, want := lines(t, opened(t, "main.go")),
		"package main\n\nfunc main() {\n\teditor.Run(os.Args)\n\tcommand.Execute(e, line)\n}"; got != want {
		t.Errorf("main.go = %q, want %q", got, want)
	}
	if got, want := lines(t, opened(t, "command_test.go")),
		"package command_test\n\nfunc TestRun(t *testing.T) {\n\tcommand.Execute(e, \":w\")\n}"; got != want {
		t.Errorf("command_test.go = %q, want %q", got, want)
	}
	if view.Buffer(filepath.Join(dir, "internal", "editor", "editor.go")) != nil {
		t.Error("editor.go was opened: its own Run is not the one being renamed")
	}
}

func TestRenameOfALocalNeverLeavesTheBlockItIsIn(t *testing.T) {
	e := state.New()

	b, _ := inProject(t, e,
		"package start\n\nfunc one() {\n\tcount := 1\n\tuse(count)\n}\n\nfunc two() {\n\tcount := 2\n\tuse(count)\n}\n",
		3, 1, map[string]string{"other.go": "package start\n\ncount()\n"})

	edtest.Press(t, e, ":rename total\n")

	edtest.WantLines(t, b, "package start", "", "func one() {", "\ttotal := 1", "\tuse(total)", "}", "",
		"func two() {", "\tcount := 2", "\tuse(count)", "}")
	wantStatus(t, e, "renamed count to total: 2 changes, local to this block")
}

func TestRenameOfAParameterStaysWithItsFunction(t *testing.T) {
	e := state.New()

	b, _ := inProject(t, e, "package start\n\nfunc one(count int) {\n\tuse(count)\n}\n\nvar count = 2\n", 3, 6, nil)

	edtest.Press(t, e, ":rename total\n")

	edtest.WantLines(t, b, "package start", "", "func one(total int) {", "\tuse(total)", "}", "", "var count = 2")
}

func TestRenameOfAFieldTakesTheMentionsThroughAValueWithIt(t *testing.T) {
	e := state.New()

	b, _ := inProject(t, e,
		"package start\n\ntype Buffer struct {\n\tfile *os.File\n}\n\nfunc read(b *Buffer) {\n\tb.file.Close()\n}\n",
		7, 4, nil)

	edtest.Press(t, e, ":rename handle\n")

	edtest.WantLines(t, b, "package start", "", "type Buffer struct {", "\thandle *os.File", "}", "",
		"func read(b *Buffer) {", "\tb.handle.Close()", "}")
}
