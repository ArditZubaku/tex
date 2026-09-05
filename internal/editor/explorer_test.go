package editor

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/ArditZubaku/tex/internal/editor/explorer"
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
	sourceFile = filepath.Join(dir, "start.txt")
	singleWindow(20, 80)

	return dir
}

func entryNames() []string {
	names := make([]string, 0, len(exp.Entries()))
	for _, e := range exp.Entries() {
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

	if mode != ExplorerMode {
		t.Fatalf("mode = %v, want ExplorerMode", mode)
	}
	if exp.Dir() != dir {
		t.Errorf("explorerDir = %q, want %q", exp.Dir(), dir)
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

	if got := entryNames()[exp.Selection()]; got != "start.txt" {
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

	if exp.Selection() != 0 {
		t.Errorf("explorerSel = %d, want 0", exp.Selection())
	}

	press(t, "jjjjj")

	if want := len(exp.Entries()) - 1; exp.Selection() != want {
		t.Errorf("explorerSel = %d, want %d", exp.Selection(), want)
	}

	press(t, "g")

	if exp.Selection() != 0 {
		t.Errorf("explorerSel = %d, want 0", exp.Selection())
	}
}

func TestExplorerDescendsIntoADirectoryAndBackOut(t *testing.T) {
	dir := inExplorer(t, "sub")
	if err := os.WriteFile(filepath.Join(dir, "sub", "deep.txt"), []byte("deep\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	press(t, " e")
	press(t, "j\n")

	if exp.Dir() != filepath.Join(dir, "sub") {
		t.Fatalf("explorerDir = %q, want %q", exp.Dir(), filepath.Join(dir, "sub"))
	}
	wantEntries(t, "../", "deep.txt")

	press(t, "-")

	if exp.Dir() != dir {
		t.Errorf("explorerDir = %q, want %q", exp.Dir(), dir)
	}
	if got := entryNames()[exp.Selection()]; got != "sub/" {
		t.Errorf("selected %q, want sub/", got)
	}
}

func TestExplorerOpensAFileIntoTheBuffer(t *testing.T) {
	dir := inExplorer(t, "other.txt")

	press(t, " e")
	press(t, "j\n")

	if mode != ReadMode {
		t.Fatalf("mode = %v, want ReadMode", mode)
	}
	if explorerOpen {
		t.Error("explorer still on screen after opening a file")
	}
	if want := filepath.Join(dir, "other.txt"); sourceFile != want {
		t.Fatalf("sourceFile = %q, want %q", sourceFile, want)
	}
	wantLines(t, buf, "in other.txt")
	if currentRow != 0 || currentCol != 0 {
		t.Errorf("cursor at %d,%d, want 0,0", currentRow, currentCol)
	}
}

func TestExplorerLeavesUnsavedChangesInTheirOwnBuffer(t *testing.T) {
	inExplorer(t, "other.txt")
	started := sourceFile
	modified = true

	press(t, " e")
	press(t, "j\n")

	if mode != ReadMode {
		t.Errorf("mode = %v, want ReadMode", mode)
	}
	if len(buffers) != 2 {
		t.Fatalf("%d buffers open, want 2", len(buffers))
	}

	press(t, "\t")

	if sourceFile != started {
		t.Errorf("sourceFile = %q, want %q", sourceFile, started)
	}
	if !modified {
		t.Error("the unsaved changes were lost")
	}
}

func TestQuitLeavesTheExplorerForTheBuffer(t *testing.T) {
	inExplorer(t, "a.txt")

	press(t, " e")
	press(t, "q")

	if mode != ReadMode {
		t.Errorf("mode = %v, want ReadMode", mode)
	}
	if quitting {
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

	if row := explorerCursorRow(); row < screenRow(explorer.HeaderRows) || row >= statusRow() {
		t.Errorf("cursor row = %d, want within [%d,%d)", row, screenRow(explorer.HeaderRows), statusRow())
	}
	if want := exp.Selection() - (ROWS - explorer.HeaderRows) + 1; exp.Offset() != want {
		t.Errorf("explorerOffset = %d, want %d", exp.Offset(), want)
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

	if want := explorerPage(); exp.Selection() != want {
		t.Errorf("explorerSel = %d, want %d", exp.Selection(), want)
	}

	press(t, "\x15")

	if exp.Selection() != 0 {
		t.Errorf("explorerSel = %d, want 0", exp.Selection())
	}
}

func TestCtrlDStopsAtTheLastEntry(t *testing.T) {
	inExplorer(t, "a.txt", "b.txt")

	press(t, " e")
	press(t, "\x04\x04\x04\x04\x04")

	if want := len(exp.Entries()) - 1; exp.Selection() != want {
		t.Errorf("explorerSel = %d, want %d", exp.Selection(), want)
	}
}

func TestLeaderEClosesTheExplorerItOpened(t *testing.T) {
	inExplorer(t, "a.txt")

	press(t, " e")
	press(t, " e")

	if mode != ReadMode {
		t.Errorf("mode = %v, want ReadMode", mode)
	}
	if explorerOpen {
		t.Error("explorer still open after the second <leader>e")
	}
}

func TestExplorerSearchNarrowsTheListingAsItIsTyped(t *testing.T) {
	inExplorer(t, "main.go", "main_test.go", "README.md")

	press(t, " e")
	press(t, "/main")

	wantEntries(t, "main.go", "main_test.go")
	if mode != PromptMode {
		t.Errorf("mode = %v, want PromptMode", mode)
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

	if mode != ExplorerMode {
		t.Fatalf("mode = %v, want ExplorerMode", mode)
	}
	if exp.Filtered() != "main" {
		t.Errorf("explorerFilter = %q, want main", exp.Filtered())
	}
	wantEntries(t, "main.go")
	if got := exp.SelectedName(); got != "main.go" {
		t.Errorf("selected %q, want main.go", got)
	}
}

func TestEscOnTheSearchPromptPutsTheWholeListingBack(t *testing.T) {
	inExplorer(t, "main.go", "README.md")

	press(t, " e")
	press(t, "/main")
	press(t, string(rune(27)))

	if mode != ExplorerMode {
		t.Fatalf("mode = %v, want ExplorerMode", mode)
	}
	wantEntries(t, "../", "README.md", "main.go")
}

func TestEscDropsTheFilterBeforeItClosesTheExplorer(t *testing.T) {
	inExplorer(t, "main.go", "README.md")

	press(t, " e")
	press(t, "/main\n")
	press(t, string(rune(27)))

	if mode != ExplorerMode {
		t.Fatalf("mode = %v, want ExplorerMode", mode)
	}
	wantEntries(t, "../", "README.md", "main.go")

	press(t, string(rune(27)))

	if mode != ReadMode {
		t.Errorf("mode = %v, want ReadMode", mode)
	}
}

func TestAFilteredFileStillOpens(t *testing.T) {
	dir := inExplorer(t, "other.txt", "README.md")

	press(t, " e")
	press(t, "/other\n")
	press(t, "\n")

	if explorerOpen {
		t.Error("explorer still on screen after opening a filtered file")
	}
	if want := filepath.Join(dir, "other.txt"); sourceFile != want {
		t.Fatalf("sourceFile = %q, want %q", sourceFile, want)
	}
	wantLines(t, buf, "in other.txt")
}

func TestSteppingIntoADirectoryDropsTheFilter(t *testing.T) {
	dir := inExplorer(t, "sub")
	if err := os.WriteFile(filepath.Join(dir, "sub", "deep.txt"), []byte("deep\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	press(t, " e")
	press(t, "/sub\n")
	press(t, "\n")

	if exp.Filtered() != "" {
		t.Errorf("explorerFilter = %q, want empty", exp.Filtered())
	}
	wantEntries(t, "../", "deep.txt")
}

func TestTogglingDotfilesKeepsTheFilter(t *testing.T) {
	inExplorer(t, ".main.swp", "main.go", "README.md")

	press(t, " e")
	press(t, "/main\n")
	press(t, "H")

	if exp.Filtered() != "main" {
		t.Errorf("explorerFilter = %q, want main", exp.Filtered())
	}
	wantEntries(t, ".main.swp", "main.go")
}
