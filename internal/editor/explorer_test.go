package editor

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/ArditZubaku/tex/internal/editor/explorer"
	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/ArditZubaku/tex/internal/editor/view"
)

func inExplorer(t *testing.T, names ...string) string {
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

	inReadMode(t, "one\n", 0, 0)
	ed.SourceFile = filepath.Join(dir, "start.txt")
	singleWindow(20, 80)

	return dir
}

func entryNames() []string {
	names := make([]string, 0, len(ed.Exp.Entries()))
	for _, e := range ed.Exp.Entries() {
		names = append(names, explorer.Label(e))
	}

	return names
}

func wantEntries(t *testing.T, want ...string) {
	t.Helper()

	got := entryNames()
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
	dir := inExplorer(t, "start.txt")

	press(t, " e")

	if ed.Mode != state.ExplorerMode {
		t.Fatalf("mode = %v, want ExplorerMode", ed.Mode)
	}
	if ed.Exp.Dir() != dir {
		t.Errorf("explorerDir = %q, want %q", ed.Exp.Dir(), dir)
	}
}

func TestExplorerListsDirectoriesFirstWithTheParentOnTop(t *testing.T) {
	inExplorer(t, "b.txt", "zeta", "a.txt", "alpha")

	press(t, " e")

	wantEntries(t, "../", "alpha/", "zeta/", "a.txt", "b.txt")
}

func TestExplorerSelectsTheFileItWasOpenedOn(t *testing.T) {
	inExplorer(t, "a.txt", "start.txt")

	press(t, " e")

	if got := entryNames()[ed.Exp.Selection()]; got != "start.txt" {
		t.Errorf("selected %q, want start.txt", got)
	}
}

func TestExplorerHidesDotfilesUntilTheyAreToggled(t *testing.T) {
	inExplorer(t, ".hidden.txt", "shown.txt")

	press(t, " e")
	wantEntries(t, "../", "shown.txt")

	press(t, "H")
	wantEntries(t, "../", ".hidden.txt", "shown.txt")
}

func TestExplorerMovementStopsAtBothEnds(t *testing.T) {
	inExplorer(t, "a.txt", "b.txt")

	press(t, " e")
	press(t, "g")
	press(t, "kk")

	if ed.Exp.Selection() != 0 {
		t.Errorf("explorerSel = %d, want 0", ed.Exp.Selection())
	}

	press(t, "jjjjj")

	if want := len(ed.Exp.Entries()) - 1; ed.Exp.Selection() != want {
		t.Errorf("explorerSel = %d, want %d", ed.Exp.Selection(), want)
	}

	press(t, "g")

	if ed.Exp.Selection() != 0 {
		t.Errorf("explorerSel = %d, want 0", ed.Exp.Selection())
	}
}

func TestExplorerDescendsIntoADirectoryAndBackOut(t *testing.T) {
	dir := inExplorer(t, "sub")
	if err := os.WriteFile(filepath.Join(dir, "sub", "deep.txt"), []byte("deep\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	press(t, " e")
	press(t, "j\n")

	if ed.Exp.Dir() != filepath.Join(dir, "sub") {
		t.Fatalf("explorerDir = %q, want %q", ed.Exp.Dir(), filepath.Join(dir, "sub"))
	}
	wantEntries(t, "../", "deep.txt")

	press(t, "-")

	if ed.Exp.Dir() != dir {
		t.Errorf("explorerDir = %q, want %q", ed.Exp.Dir(), dir)
	}
	if got := entryNames()[ed.Exp.Selection()]; got != "sub/" {
		t.Errorf("selected %q, want sub/", got)
	}
}

func TestExplorerOpensAFileIntoTheBuffer(t *testing.T) {
	dir := inExplorer(t, "other.txt")

	press(t, " e")
	press(t, "j\n")

	if ed.Mode != state.ReadMode {
		t.Fatalf("mode = %v, want ReadMode", ed.Mode)
	}
	if ed.ExplorerOpen {
		t.Error("explorer still on screen after opening a file")
	}
	if want := filepath.Join(dir, "other.txt"); ed.SourceFile != want {
		t.Fatalf("sourceFile = %q, want %q", ed.SourceFile, want)
	}
	wantLines(t, ed.Buf, "in other.txt")
	if ed.Row != 0 || ed.Col != 0 {
		t.Errorf("cursor at %d,%d, want 0,0", ed.Row, ed.Col)
	}
}

func TestExplorerLeavesUnsavedChangesInTheirOwnBuffer(t *testing.T) {
	inExplorer(t, "other.txt")
	started := ed.SourceFile
	ed.Modified = true

	press(t, " e")
	press(t, "j\n")

	if ed.Mode != state.ReadMode {
		t.Errorf("mode = %v, want ReadMode", ed.Mode)
	}
	if len(view.Buffers()) != 2 {
		t.Fatalf("%d buffers open, want 2", len(view.Buffers()))
	}

	press(t, "\t")

	if ed.SourceFile != started {
		t.Errorf("sourceFile = %q, want %q", ed.SourceFile, started)
	}
	if !ed.Modified {
		t.Error("the unsaved changes were lost")
	}
}

func TestQuitLeavesTheExplorerForTheBuffer(t *testing.T) {
	inExplorer(t, "a.txt")

	press(t, " e")
	press(t, "q")

	if ed.Mode != state.ReadMode {
		t.Errorf("mode = %v, want ReadMode", ed.Mode)
	}
	if ed.Quitting {
		t.Error("leaving the explorer quit the editor")
	}
}

func TestExplorerScrollsTheSelectionIntoView(t *testing.T) {
	names := make([]string, 0, 40)
	for i := range 40 {
		names = append(names, string(rune('a'+i%26))+string(rune('a'+i/26))+".txt")
	}
	inExplorer(t, names...)

	press(t, " e")
	press(t, "G")
	displayExplorer()

	if row := explorerCursorRow(); row < ed.ScreenRow(explorer.HeaderRows) || row >= ed.StatusRow() {
		t.Errorf("cursor row = %d, want within [%d,%d)", row, ed.ScreenRow(explorer.HeaderRows), ed.StatusRow())
	}
	if want := ed.Exp.Selection() - (ed.Rows - explorer.HeaderRows) + 1; ed.Exp.Offset() != want {
		t.Errorf("explorerOffset = %d, want %d", ed.Exp.Offset(), want)
	}
}

func TestCtrlDAndCtrlUMoveTheSelectionHalfAScreen(t *testing.T) {
	names := make([]string, 0, 40)
	for i := range 40 {
		names = append(names, fmt.Sprintf("f%02d.txt", i))
	}
	inExplorer(t, names...)

	press(t, " e")
	press(t, "\x04")

	if want := explorerPage(); ed.Exp.Selection() != want {
		t.Errorf("explorerSel = %d, want %d", ed.Exp.Selection(), want)
	}

	press(t, "\x15")

	if ed.Exp.Selection() != 0 {
		t.Errorf("explorerSel = %d, want 0", ed.Exp.Selection())
	}
}

func TestCtrlDStopsAtTheLastEntry(t *testing.T) {
	inExplorer(t, "a.txt", "b.txt")

	press(t, " e")
	press(t, "\x04\x04\x04\x04\x04")

	if want := len(ed.Exp.Entries()) - 1; ed.Exp.Selection() != want {
		t.Errorf("explorerSel = %d, want %d", ed.Exp.Selection(), want)
	}
}

func TestLeaderEClosesTheExplorerItOpened(t *testing.T) {
	inExplorer(t, "a.txt")

	press(t, " e")
	press(t, " e")

	if ed.Mode != state.ReadMode {
		t.Errorf("mode = %v, want ReadMode", ed.Mode)
	}
	if ed.ExplorerOpen {
		t.Error("explorer still open after the second <leader>e")
	}
}

func TestExplorerSearchNarrowsTheListingAsItIsTyped(t *testing.T) {
	inExplorer(t, "main.go", "main_test.go", "README.md")

	press(t, " e")
	press(t, "/main")

	wantEntries(t, "main.go", "main_test.go")
	if ed.Mode != state.PromptMode {
		t.Errorf("mode = %v, want PromptMode", ed.Mode)
	}

	press(t, "_")

	wantEntries(t, "main_test.go")
}

func TestExplorerSearchIsCaseInsensitive(t *testing.T) {
	inExplorer(t, "README.md", "main.go")

	press(t, " e")
	press(t, "/readme\n")

	wantEntries(t, "README.md")
}

func TestEnterKeepsTheFilterAndReturnsToTheExplorer(t *testing.T) {
	inExplorer(t, "main.go", "README.md")

	press(t, " e")
	press(t, "/main\n")

	if ed.Mode != state.ExplorerMode {
		t.Fatalf("mode = %v, want ExplorerMode", ed.Mode)
	}
	if ed.Exp.Filtered() != "main" {
		t.Errorf("explorerFilter = %q, want main", ed.Exp.Filtered())
	}
	wantEntries(t, "main.go")
	if got := ed.Exp.SelectedName(); got != "main.go" {
		t.Errorf("selected %q, want main.go", got)
	}
}

func TestEscOnTheSearchPromptPutsTheWholeListingBack(t *testing.T) {
	inExplorer(t, "main.go", "README.md")

	press(t, " e")
	press(t, "/main")
	press(t, string(rune(27)))

	if ed.Mode != state.ExplorerMode {
		t.Fatalf("mode = %v, want ExplorerMode", ed.Mode)
	}
	wantEntries(t, "../", "README.md", "main.go")
}

func TestEscDropsTheFilterBeforeItClosesTheExplorer(t *testing.T) {
	inExplorer(t, "main.go", "README.md")

	press(t, " e")
	press(t, "/main\n")
	press(t, string(rune(27)))

	if ed.Mode != state.ExplorerMode {
		t.Fatalf("mode = %v, want ExplorerMode", ed.Mode)
	}
	wantEntries(t, "../", "README.md", "main.go")

	press(t, string(rune(27)))

	if ed.Mode != state.ReadMode {
		t.Errorf("mode = %v, want ReadMode", ed.Mode)
	}
}

func TestAFilteredFileStillOpens(t *testing.T) {
	dir := inExplorer(t, "other.txt", "README.md")

	press(t, " e")
	press(t, "/other\n")
	press(t, "\n")

	if ed.ExplorerOpen {
		t.Error("explorer still on screen after opening a filtered file")
	}
	if want := filepath.Join(dir, "other.txt"); ed.SourceFile != want {
		t.Fatalf("sourceFile = %q, want %q", ed.SourceFile, want)
	}
	wantLines(t, ed.Buf, "in other.txt")
}

func TestSteppingIntoADirectoryDropsTheFilter(t *testing.T) {
	dir := inExplorer(t, "sub")
	if err := os.WriteFile(filepath.Join(dir, "sub", "deep.txt"), []byte("deep\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	press(t, " e")
	press(t, "/sub\n")
	press(t, "\n")

	if ed.Exp.Filtered() != "" {
		t.Errorf("explorerFilter = %q, want empty", ed.Exp.Filtered())
	}
	wantEntries(t, "../", "deep.txt")
}

func TestTogglingDotfilesKeepsTheFilter(t *testing.T) {
	inExplorer(t, ".main.swp", "main.go", "README.md")

	press(t, " e")
	press(t, "/main\n")
	press(t, "H")

	if ed.Exp.Filtered() != "main" {
		t.Errorf("explorerFilter = %q, want main", ed.Exp.Filtered())
	}
	wantEntries(t, ".main.swp", "main.go")
}
