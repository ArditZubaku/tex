package editor

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/ArditZubaku/tex/internal/editor/edtest"
	"github.com/ArditZubaku/tex/internal/editor/explorer"
	"github.com/ArditZubaku/tex/internal/editor/filetree"
	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/ArditZubaku/tex/internal/editor/view"
)

func inExplorer(t *testing.T, e *state.Editor, names ...string) string {
	t.Helper()

	dir := t.TempDir()
	for _, name := range names {
		path := filepath.Join(dir, name)
		if filepath.Ext(name) == "" {
			if err := os.Mkdir(path, 0o700); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.WriteFile(path, []byte("in "+name+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	edtest.InReadMode(t, e, "one\n", 0, 0)
	e.SourceFile = filepath.Join(dir, "start.txt")
	edtest.SingleWindow(e, 20, 80)

	return dir
}

func entryNames(e *state.Editor) []string {
	names := make([]string, 0, len(e.Exp.Entries()))
	for _, e := range e.Exp.Entries() {
		names = append(names, filetree.Label(e))
	}

	return names
}

func wantEntries(t *testing.T, e *state.Editor, want ...string) {
	t.Helper()

	got := entryNames(e)
	if len(got) != len(want) {
		t.Fatalf("entries = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("entries = %v, want %v", got, want)
		}
	}
}

func TestLeaderEOpensTheExplorerOnTheFilesOwnDirectory(t *testing.T) {
	e := state.New()

	dir := inExplorer(t, e, "start.txt")

	edtest.Press(t, e, " e")

	if e.Mode != state.ExplorerMode {
		t.Fatalf("mode = %v, want ExplorerMode", e.Mode)
	}
	if e.Exp.Dir() != dir {
		t.Errorf("explorerDir = %q, want %q", e.Exp.Dir(), dir)
	}
}

func TestExplorerListsDirectoriesFirstWithTheParentOnTop(t *testing.T) {
	e := state.New()

	inExplorer(t, e, "b.txt", "zeta", "a.txt", "alpha")

	edtest.Press(t, e, " e")

	wantEntries(t, e, "../", "alpha/", "zeta/", "a.txt", "b.txt")
}

func TestExplorerSelectsTheFileItWasOpenedOn(t *testing.T) {
	e := state.New()

	inExplorer(t, e, "a.txt", "start.txt")

	edtest.Press(t, e, " e")

	if got := entryNames(e)[e.Exp.Selection()]; got != "start.txt" {
		t.Errorf("selected %q, want start.txt", got)
	}
}

func TestExplorerHidesDotfilesUntilTheyAreToggled(t *testing.T) {
	e := state.New()

	inExplorer(t, e, ".hidden.txt", "shown.txt")

	edtest.Press(t, e, " e")
	wantEntries(t, e, "../", "shown.txt")

	edtest.Press(t, e, "H")
	wantEntries(t, e, "../", ".hidden.txt", "shown.txt")
}

func TestExplorerMovementStopsAtBothEnds(t *testing.T) {
	e := state.New()

	inExplorer(t, e, "a.txt", "b.txt")

	edtest.Press(t, e, " e")
	edtest.Press(t, e, "g")
	edtest.Press(t, e, "kk")

	if e.Exp.Selection() != 0 {
		t.Errorf("explorerSel = %d, want 0", e.Exp.Selection())
	}

	edtest.Press(t, e, "jjjjj")

	if want := len(e.Exp.Entries()) - 1; e.Exp.Selection() != want {
		t.Errorf("explorerSel = %d, want %d", e.Exp.Selection(), want)
	}

	edtest.Press(t, e, "g")

	if e.Exp.Selection() != 0 {
		t.Errorf("explorerSel = %d, want 0", e.Exp.Selection())
	}
}

func TestExplorerDescendsIntoADirectoryAndBackOut(t *testing.T) {
	e := state.New()

	dir := inExplorer(t, e, "sub")
	if err := os.WriteFile(filepath.Join(dir, "sub", "deep.txt"), []byte("deep\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	edtest.Press(t, e, " e")
	edtest.Press(t, e, "j\n")

	if e.Exp.Dir() != filepath.Join(dir, "sub") {
		t.Fatalf("explorerDir = %q, want %q", e.Exp.Dir(), filepath.Join(dir, "sub"))
	}
	wantEntries(t, e, "../", "deep.txt")

	edtest.Press(t, e, "-")

	if e.Exp.Dir() != dir {
		t.Errorf("explorerDir = %q, want %q", e.Exp.Dir(), dir)
	}
	if got := entryNames(e)[e.Exp.Selection()]; got != "sub/" {
		t.Errorf("selected %q, want sub/", got)
	}
}

func TestExplorerOpensAFileIntoTheBuffer(t *testing.T) {
	e := state.New()

	dir := inExplorer(t, e, "other.txt")

	edtest.Press(t, e, " e")
	edtest.Press(t, e, "j\n")

	if e.Mode != state.ReadMode {
		t.Fatalf("mode = %v, want ReadMode", e.Mode)
	}
	if e.ExplorerOpen {
		t.Error("explorer still on screen after opening a file")
	}
	if want := filepath.Join(dir, "other.txt"); e.SourceFile != want {
		t.Fatalf("sourceFile = %q, want %q", e.SourceFile, want)
	}
	edtest.WantLines(t, e.Buf, "in other.txt")
	if e.Row != 0 || e.Col != 0 {
		t.Errorf("cursor at %d,%d, want 0,0", e.Row, e.Col)
	}
}

func TestExplorerLeavesUnsavedChangesInTheirOwnBuffer(t *testing.T) {
	e := state.New()

	inExplorer(t, e, "other.txt")
	started := e.SourceFile
	e.Modified = true

	edtest.Press(t, e, " e")
	edtest.Press(t, e, "j\n")

	if e.Mode != state.ReadMode {
		t.Errorf("mode = %v, want ReadMode", e.Mode)
	}
	if len(view.Buffers()) != 2 {
		t.Fatalf("%d buffers open, want 2", len(view.Buffers()))
	}

	edtest.Press(t, e, "\t")

	if e.SourceFile != started {
		t.Errorf("sourceFile = %q, want %q", e.SourceFile, started)
	}
	if !e.Modified {
		t.Error("the unsaved changes were lost")
	}
}

func TestQuitLeavesTheExplorerForTheBuffer(t *testing.T) {
	e := state.New()

	inExplorer(t, e, "a.txt")

	edtest.Press(t, e, " e")
	edtest.Press(t, e, "q")

	if e.Mode != state.ReadMode {
		t.Errorf("mode = %v, want ReadMode", e.Mode)
	}
	if e.Quitting {
		t.Error("leaving the explorer quit the editor")
	}
}

func TestExplorerScrollsTheSelectionIntoView(t *testing.T) {
	e := state.New()

	names := make([]string, 0, 40)
	for i := range 40 {
		names = append(names, string(rune('a'+i%26))+string(rune('a'+i/26))+".txt")
	}
	inExplorer(t, e, names...)

	edtest.Press(t, e, " e")
	edtest.Press(t, e, "G")
	explorer.Draw(e)

	if row := explorer.CursorRow(e); row < e.ScreenRow(filetree.HeaderRows) || row >= e.StatusRow() {
		t.Errorf("cursor row = %d, want within [%d,%d)", row, e.ScreenRow(filetree.HeaderRows), e.StatusRow())
	}
	if want := e.Exp.Selection() - (e.Rows - filetree.HeaderRows) + 1; e.Exp.Offset() != want {
		t.Errorf("explorerOffset = %d, want %d", e.Exp.Offset(), want)
	}
}

func TestCtrlDAndCtrlUMoveTheSelectionHalfAScreen(t *testing.T) {
	e := state.New()

	names := make([]string, 0, 40)
	for i := range 40 {
		names = append(names, fmt.Sprintf("f%02d.txt", i))
	}
	inExplorer(t, e, names...)

	edtest.Press(t, e, " e")
	edtest.Press(t, e, "\x04")

	if want := explorer.Page(e); e.Exp.Selection() != want {
		t.Errorf("explorerSel = %d, want %d", e.Exp.Selection(), want)
	}

	edtest.Press(t, e, "\x15")

	if e.Exp.Selection() != 0 {
		t.Errorf("explorerSel = %d, want 0", e.Exp.Selection())
	}
}

func TestCtrlDStopsAtTheLastEntry(t *testing.T) {
	e := state.New()

	inExplorer(t, e, "a.txt", "b.txt")

	edtest.Press(t, e, " e")
	edtest.Press(t, e, "\x04\x04\x04\x04\x04")

	if want := len(e.Exp.Entries()) - 1; e.Exp.Selection() != want {
		t.Errorf("explorerSel = %d, want %d", e.Exp.Selection(), want)
	}
}

func TestLeaderEClosesTheExplorerItOpened(t *testing.T) {
	e := state.New()

	inExplorer(t, e, "a.txt")

	edtest.Press(t, e, " e")
	edtest.Press(t, e, " e")

	if e.Mode != state.ReadMode {
		t.Errorf("mode = %v, want ReadMode", e.Mode)
	}
	if e.ExplorerOpen {
		t.Error("explorer still open after the second <leader>e")
	}
}

func TestExplorerSearchNarrowsTheListingAsItIsTyped(t *testing.T) {
	e := state.New()

	inExplorer(t, e, "main.go", "main_test.go", "README.md")

	edtest.Press(t, e, " e")
	edtest.Press(t, e, "/main")

	wantEntries(t, e, "main.go", "main_test.go")
	if e.Mode != state.PromptMode {
		t.Errorf("mode = %v, want PromptMode", e.Mode)
	}

	edtest.Press(t, e, "_")

	wantEntries(t, e, "main_test.go")
}

func TestExplorerSearchIsCaseInsensitive(t *testing.T) {
	e := state.New()

	inExplorer(t, e, "README.md", "main.go")

	edtest.Press(t, e, " e")
	edtest.Press(t, e, "/readme\n")

	wantEntries(t, e, "README.md")
}

func TestEnterKeepsTheFilterAndReturnsToTheExplorer(t *testing.T) {
	e := state.New()

	inExplorer(t, e, "main.go", "README.md")

	edtest.Press(t, e, " e")
	edtest.Press(t, e, "/main\n")

	if e.Mode != state.ExplorerMode {
		t.Fatalf("mode = %v, want ExplorerMode", e.Mode)
	}
	if e.Exp.Filtered() != "main" {
		t.Errorf("explorerFilter = %q, want main", e.Exp.Filtered())
	}
	wantEntries(t, e, "main.go")
	if got := e.Exp.SelectedName(); got != "main.go" {
		t.Errorf("selected %q, want main.go", got)
	}
}

func TestEscOnTheSearchPromptPutsTheWholeListingBack(t *testing.T) {
	e := state.New()

	inExplorer(t, e, "main.go", "README.md")

	edtest.Press(t, e, " e")
	edtest.Press(t, e, "/main")
	edtest.Press(t, e, string(rune(27)))

	if e.Mode != state.ExplorerMode {
		t.Fatalf("mode = %v, want ExplorerMode", e.Mode)
	}
	wantEntries(t, e, "../", "README.md", "main.go")
}

func TestEscDropsTheFilterBeforeItClosesTheExplorer(t *testing.T) {
	e := state.New()

	inExplorer(t, e, "main.go", "README.md")

	edtest.Press(t, e, " e")
	edtest.Press(t, e, "/main\n")
	edtest.Press(t, e, string(rune(27)))

	if e.Mode != state.ExplorerMode {
		t.Fatalf("mode = %v, want ExplorerMode", e.Mode)
	}
	wantEntries(t, e, "../", "README.md", "main.go")

	edtest.Press(t, e, string(rune(27)))

	if e.Mode != state.ReadMode {
		t.Errorf("mode = %v, want ReadMode", e.Mode)
	}
}

func TestAFilteredFileStillOpens(t *testing.T) {
	e := state.New()

	dir := inExplorer(t, e, "other.txt", "README.md")

	edtest.Press(t, e, " e")
	edtest.Press(t, e, "/other\n")
	edtest.Press(t, e, "\n")

	if e.ExplorerOpen {
		t.Error("explorer still on screen after opening a filtered file")
	}
	if want := filepath.Join(dir, "other.txt"); e.SourceFile != want {
		t.Fatalf("sourceFile = %q, want %q", e.SourceFile, want)
	}
	edtest.WantLines(t, e.Buf, "in other.txt")
}

func TestSteppingIntoADirectoryDropsTheFilter(t *testing.T) {
	e := state.New()

	dir := inExplorer(t, e, "sub")
	if err := os.WriteFile(filepath.Join(dir, "sub", "deep.txt"), []byte("deep\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	edtest.Press(t, e, " e")
	edtest.Press(t, e, "/sub\n")
	edtest.Press(t, e, "\n")

	if e.Exp.Filtered() != "" {
		t.Errorf("explorerFilter = %q, want empty", e.Exp.Filtered())
	}
	wantEntries(t, e, "../", "deep.txt")
}

func TestTogglingDotfilesKeepsTheFilter(t *testing.T) {
	e := state.New()

	inExplorer(t, e, ".main.swp", "main.go", "README.md")

	edtest.Press(t, e, " e")
	edtest.Press(t, e, "/main\n")
	edtest.Press(t, e, "H")

	if e.Exp.Filtered() != "main" {
		t.Errorf("explorerFilter = %q, want main", e.Exp.Filtered())
	}
	wantEntries(t, e, ".main.swp", "main.go")
}
