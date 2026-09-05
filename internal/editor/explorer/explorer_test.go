// The tests sit outside the package because the harness they share with the
// rest of the editor imports it.
package explorer_test

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

func TestACreatesAFileInTheDirectoryBeingListed(t *testing.T) {
	e := state.New()

	dir := inExplorer(t, e, "start.txt")

	edtest.Press(t, e, " e")
	edtest.Press(t, e, "anotes.md\n")

	if e.Mode != state.ExplorerMode {
		t.Fatalf("mode = %v, want ExplorerMode", e.Mode)
	}
	info, err := os.Stat(filepath.Join(dir, "notes.md"))
	if err != nil {
		t.Fatal(err)
	}
	if info.IsDir() {
		t.Error("notes.md is a directory")
	}
	wantEntries(t, e, "../", "notes.md", "start.txt")
	if got := e.Exp.SelectedName(); got != "notes.md" {
		t.Errorf("selected %q, want notes.md", got)
	}
}

func TestATrailingSlashCreatesADirectory(t *testing.T) {
	e := state.New()

	dir := inExplorer(t, e, "start.txt")

	edtest.Press(t, e, " e")
	edtest.Press(t, e, "apkg/\n")

	info, err := os.Stat(filepath.Join(dir, "pkg"))
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() {
		t.Error("pkg is not a directory")
	}
	wantEntries(t, e, "../", "pkg/", "start.txt")
}

func TestAPathCreatesTheDirectoriesOnTheWayToTheFile(t *testing.T) {
	e := state.New()

	dir := inExplorer(t, e, "start.txt")

	edtest.Press(t, e, " e")
	edtest.Press(t, e, "ainternal/editor/keys.go\n")

	if _, err := os.Stat(filepath.Join(dir, "internal", "editor", "keys.go")); err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "internal", "editor"); e.Exp.Dir() != want {
		t.Errorf("explorerDir = %q, want %q", e.Exp.Dir(), want)
	}
	if got := e.Exp.SelectedName(); got != "keys.go" {
		t.Errorf("selected %q, want keys.go", got)
	}
}

func TestACreatingOverAnExistingNameLeavesItAlone(t *testing.T) {
	e := state.New()

	dir := inExplorer(t, e, "start.txt")

	edtest.Press(t, e, " e")
	edtest.Press(t, e, "astart.txt\n")

	content, err := os.ReadFile(filepath.Join(dir, "start.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "in start.txt\n" {
		t.Errorf("start.txt = %q, want it untouched", content)
	}
	if e.StatusMsg == "" {
		t.Error("nothing reported for a name that already exists")
	}
}

func TestEscOnTheCreatePromptMakesNothing(t *testing.T) {
	e := state.New()

	dir := inExplorer(t, e, "start.txt")

	edtest.Press(t, e, " e")
	edtest.Press(t, e, "anotes.md")
	edtest.Press(t, e, string(rune(27)))

	if e.Mode != state.ExplorerMode {
		t.Fatalf("mode = %v, want ExplorerMode", e.Mode)
	}
	if _, err := os.Stat(filepath.Join(dir, "notes.md")); !os.IsNotExist(err) {
		t.Error("notes.md was created by a cancelled prompt")
	}
	wantEntries(t, e, "../", "start.txt")
}

func TestTheCreatePromptDoesNotNarrowTheListing(t *testing.T) {
	e := state.New()

	inExplorer(t, e, "main.go", "README.md")

	edtest.Press(t, e, " e")
	edtest.Press(t, e, "azzz")

	if txt, ok := e.PromptStatus(); !ok || txt != "new: zzz" {
		t.Errorf("prompt = %q, want %q", txt, "new: zzz")
	}
	wantEntries(t, e, "../", "README.md", "main.go")
}

func TestYyThenPCopiesTheFileUnderAnotherName(t *testing.T) {
	e := state.New()

	dir := inExplorer(t, e, "notes.md")

	edtest.Press(t, e, " e")
	edtest.Press(t, e, "jyy")
	edtest.Press(t, e, "p")

	if txt, ok := e.PromptStatus(); !ok || txt != "paste as: notes.md" {
		t.Fatalf("prompt = %q, want %q", txt, "paste as: notes.md")
	}

	edtest.Press(t, e, "\x15copy.md\n")

	content, err := os.ReadFile(filepath.Join(dir, "copy.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "in notes.md\n" {
		t.Errorf("copy.md = %q, want the yanked file's content", content)
	}
	wantEntries(t, e, "../", "copy.md", "notes.md")
	if got := e.Exp.SelectedName(); got != "copy.md" {
		t.Errorf("selected %q, want copy.md", got)
	}
}

func TestAYankedFilePastesIntoAnotherDirectory(t *testing.T) {
	e := state.New()

	dir := inExplorer(t, e, "notes.md", "pkg")

	edtest.Press(t, e, " e")
	edtest.Press(t, e, "jjyy")
	edtest.Press(t, e, "kl")

	if want := filepath.Join(dir, "pkg"); e.Exp.Dir() != want {
		t.Fatalf("explorerDir = %q, want %q", e.Exp.Dir(), want)
	}

	edtest.Press(t, e, "p\n")

	if _, err := os.Stat(filepath.Join(dir, "pkg", "notes.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "notes.md")); err != nil {
		t.Error("the yanked file went with the copy")
	}
	wantEntries(t, e, "../", "notes.md")
}

func TestPastingOverAnExistingNameLeavesItAlone(t *testing.T) {
	e := state.New()

	dir := inExplorer(t, e, "notes.md", "other.md")

	edtest.Press(t, e, " e")
	edtest.Press(t, e, "jyy")
	edtest.Press(t, e, "p\x15other.md\n")

	content, err := os.ReadFile(filepath.Join(dir, "other.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "in other.md\n" {
		t.Errorf("other.md = %q, want it untouched", content)
	}
}

func TestPWithNothingYankedOpensNoPrompt(t *testing.T) {
	e := state.New()

	inExplorer(t, e, "notes.md")

	edtest.Press(t, e, " e")
	edtest.Press(t, e, "p")

	if e.Mode != state.ExplorerMode {
		t.Errorf("mode = %v, want ExplorerMode", e.Mode)
	}
	if e.StatusMsg == "" {
		t.Error("nothing reported for a paste with an empty register")
	}
}

func TestYyRefusesADirectory(t *testing.T) {
	e := state.New()

	inExplorer(t, e, "pkg")

	edtest.Press(t, e, " e")
	edtest.Press(t, e, "jyy")

	if got := e.Exp.Yanked(); got != "" {
		t.Errorf("yanked %q, want a directory to be refused", got)
	}
}

func TestDDeletesTheFileOnceTheConfirmationIsAnswered(t *testing.T) {
	e := state.New()

	dir := inExplorer(t, e, "notes.md", "other.md")

	edtest.Press(t, e, " e")
	edtest.Press(t, e, "jd")

	if txt, ok := e.PromptStatus(); !ok || txt != "delete notes.md? [y/N] " {
		t.Fatalf("prompt = %q, want %q", txt, "delete notes.md? [y/N] ")
	}

	edtest.Press(t, e, "y\n")

	if _, err := os.Stat(filepath.Join(dir, "notes.md")); !os.IsNotExist(err) {
		t.Error("notes.md is still there")
	}
	wantEntries(t, e, "../", "other.md")
	if got := e.Exp.SelectedName(); got != "other.md" {
		t.Errorf("selected %q, want other.md", got)
	}
}

func TestDKeepsTheFileWhenTheConfirmationIsNot(t *testing.T) {
	e := state.New()

	dir := inExplorer(t, e, "notes.md")

	edtest.Press(t, e, " e")
	edtest.Press(t, e, "jd\n")

	if _, err := os.Stat(filepath.Join(dir, "notes.md")); err != nil {
		t.Error("notes.md was deleted without a y")
	}

	edtest.Press(t, e, "d")
	edtest.Press(t, e, string(rune(27)))

	if _, err := os.Stat(filepath.Join(dir, "notes.md")); err != nil {
		t.Error("notes.md was deleted by a cancelled prompt")
	}
	if e.Mode != state.ExplorerMode {
		t.Errorf("mode = %v, want ExplorerMode", e.Mode)
	}
}

func TestDRefusesADirectoryWithAnythingInIt(t *testing.T) {
	e := state.New()

	dir := inExplorer(t, e, "pkg")
	if err := os.WriteFile(filepath.Join(dir, "pkg", "in.txt"), []byte("x\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	edtest.Press(t, e, " e")
	edtest.Press(t, e, "jdy\n")

	if _, err := os.Stat(filepath.Join(dir, "pkg")); err != nil {
		t.Error("a directory with a file in it was deleted")
	}
	wantEntries(t, e, "../", "pkg/")
}

func TestDDeletesAnEmptyDirectory(t *testing.T) {
	e := state.New()

	dir := inExplorer(t, e, "pkg")

	edtest.Press(t, e, " e")
	edtest.Press(t, e, "jdy\n")

	if _, err := os.Stat(filepath.Join(dir, "pkg")); !os.IsNotExist(err) {
		t.Error("pkg is still there")
	}
	wantEntries(t, e, "../")
}

func TestTheParentEntryIsNeitherYankedNorDeleted(t *testing.T) {
	e := state.New()

	dir := inExplorer(t, e, "notes.md")

	edtest.Press(t, e, " e")
	edtest.Press(t, e, "yy")
	edtest.Press(t, e, "d")

	if got := e.Exp.Yanked(); got != "" {
		t.Errorf("yanked %q, want the parent entry to be refused", got)
	}
	if e.Mode != state.ExplorerMode {
		t.Errorf("mode = %v, want ExplorerMode", e.Mode)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Error("the directory being listed was deleted")
	}
}

func TestDeletingAFileClosesTheBufferItWasOpenedIn(t *testing.T) {
	e := state.New()

	inExplorer(t, e, "start.txt", "notes.md")

	edtest.Press(t, e, " e")
	edtest.Press(t, e, "gj\n")

	edtest.WantBuffers(t, "start.txt", "notes.md")
	edtest.WantCurrent(t, e, "notes.md")

	edtest.Press(t, e, " e")
	edtest.Press(t, e, "dy\n")

	edtest.WantBuffers(t, "start.txt")
	edtest.WantCurrent(t, e, "start.txt")
}

func TestDeletingAFileClosesItsBufferFromBehindAnother(t *testing.T) {
	e := state.New()

	inExplorer(t, e, "start.txt", "notes.md", "other.md")

	edtest.Press(t, e, " e")
	edtest.Press(t, e, "gj\n")
	edtest.Press(t, e, " e")
	edtest.Press(t, e, "j\n")

	edtest.WantBuffers(t, "start.txt", "notes.md", "other.md")
	edtest.WantCurrent(t, e, "other.md")

	edtest.Press(t, e, " e")
	edtest.Press(t, e, "kdy\n")

	edtest.WantBuffers(t, "start.txt", "other.md")
	edtest.WantCurrent(t, e, "other.md")
}

func TestDeletingAFileWithUnsavedChangesStillClosesItsBuffer(t *testing.T) {
	e := state.New()

	inExplorer(t, e, "start.txt", "notes.md")

	edtest.Press(t, e, " e")
	edtest.Press(t, e, "gj\n")
	edtest.Press(t, e, "ix")
	edtest.Press(t, e, string(rune(27)))

	if !e.Modified {
		t.Fatal("notes.md is not modified")
	}

	edtest.Press(t, e, " e")
	edtest.Press(t, e, "dy\n")

	edtest.WantBuffers(t, "start.txt")
	if e.Modified {
		t.Error("the editor still reports unsaved changes to the deleted file")
	}
}

func TestDeletingAFileNothingHasOpenedLeavesTheBufferListAlone(t *testing.T) {
	e := state.New()

	inExplorer(t, e, "start.txt", "notes.md", "other.md")

	edtest.Press(t, e, " e")
	edtest.Press(t, e, "gj\n")
	edtest.Press(t, e, " e")
	edtest.Press(t, e, "jdy\n")

	edtest.WantBuffers(t, "start.txt", "notes.md")
	edtest.WantCurrent(t, e, "notes.md")
}
