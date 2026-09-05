package editor

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
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
	names := make([]string, 0, len(explorerEntries))
	for _, e := range explorerEntries {
		names = append(names, entryLabel(e))
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
	if explorerDir != dir {
		t.Errorf("explorerDir = %q, want %q", explorerDir, dir)
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

	if got := entryNames()[explorerSel]; got != "start.txt" {
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

	if explorerSel != 0 {
		t.Errorf("explorerSel = %d, want 0", explorerSel)
	}

	press(t, "jjjjj")

	if want := len(explorerEntries) - 1; explorerSel != want {
		t.Errorf("explorerSel = %d, want %d", explorerSel, want)
	}

	press(t, "g")

	if explorerSel != 0 {
		t.Errorf("explorerSel = %d, want 0", explorerSel)
	}
}

func TestExplorerDescendsIntoADirectoryAndBackOut(t *testing.T) {
	dir := inExplorer(t, "sub")
	if err := os.WriteFile(filepath.Join(dir, "sub", "deep.txt"), []byte("deep\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	press(t, " e")
	press(t, "j\n")

	if explorerDir != filepath.Join(dir, "sub") {
		t.Fatalf("explorerDir = %q, want %q", explorerDir, filepath.Join(dir, "sub"))
	}
	wantEntries(t, "../", "deep.txt")

	press(t, "-")

	if explorerDir != dir {
		t.Errorf("explorerDir = %q, want %q", explorerDir, dir)
	}
	if got := entryNames()[explorerSel]; got != "sub/" {
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
	scrollExplorer()

	if row := explorerCursorRow(); row < screenRow(explorerHeaderRows) || row >= statusRow() {
		t.Errorf("cursor row = %d, want within [%d,%d)", row, screenRow(explorerHeaderRows), statusRow())
	}
	if want := explorerSel - (ROWS - explorerHeaderRows) + 1; explorerOffset != want {
		t.Errorf("explorerOffset = %d, want %d", explorerOffset, want)
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

	if want := explorerPage(); explorerSel != want {
		t.Errorf("explorerSel = %d, want %d", explorerSel, want)
	}

	press(t, "\x15")

	if explorerSel != 0 {
		t.Errorf("explorerSel = %d, want 0", explorerSel)
	}
}

func TestCtrlDStopsAtTheLastEntry(t *testing.T) {
	inExplorer(t, "a.txt", "b.txt")

	press(t, " e")
	press(t, "\x04\x04\x04\x04\x04")

	if want := len(explorerEntries) - 1; explorerSel != want {
		t.Errorf("explorerSel = %d, want %d", explorerSel, want)
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
	if explorerFilter != "main" {
		t.Errorf("explorerFilter = %q, want main", explorerFilter)
	}
	wantEntries(t, "main.go")
	if got := selectedName(); got != "main.go" {
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

	if explorerFilter != "" {
		t.Errorf("explorerFilter = %q, want empty", explorerFilter)
	}
	wantEntries(t, "../", "deep.txt")
}

func TestTogglingDotfilesKeepsTheFilter(t *testing.T) {
	inExplorer(t, ".main.swp", "main.go", "README.md")

	press(t, " e")
	press(t, "/main\n")
	press(t, "H")

	if explorerFilter != "main" {
		t.Errorf("explorerFilter = %q, want main", explorerFilter)
	}
	wantEntries(t, ".main.swp", "main.go")
}
