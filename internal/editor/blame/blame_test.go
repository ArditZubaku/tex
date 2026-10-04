package blame_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ArditZubaku/tex/internal/editor/blame"
	"github.com/ArditZubaku/tex/internal/editor/edtest"
	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/ArditZubaku/tex/internal/editor/view"
)

// killAnyGitShowPane is cleanup for a test that called ShowDiff successfully:
// a real 'git show' process was started, and nothing but killing it ends it.
func killAnyGitShowPane() {
	if w := view.Focused(); w != nil && w.Entry.Terminal != nil {
		w.Entry.Terminal.Kill()
	}
}

func initRepo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	run(t, dir, "init", "-q")
	run(t, dir, "config", "user.email", "a@example.com")
	run(t, dir, "config", "user.name", "A Author")

	return dir
}

func run(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func commitFile(t *testing.T, dir, content, summary string) string {
	t.Helper()

	path := filepath.Join(dir, "f.go")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, dir, "add", "f.go")
	run(t, dir, "commit", "-q", "-m", summary)

	return path
}

func TestShowLinePutsTheCommitInTheHoverBox(t *testing.T) {
	dir := initRepo(t)
	path := commitFile(t, dir, "package main\n", "initial scaffold")

	e := state.New()
	edtest.InReadMode(t, e, "package main\n", 0, 0)
	e.SourceFile = path
	edtest.SingleWindow(e, 24, 80)

	blame.ShowLine(e)

	if !e.Hov.Showing() {
		t.Fatal("ShowLine() left the hover box empty")
	}
	text := e.Hov.Text()
	if !strings.Contains(text, "A Author") {
		t.Errorf("box = %q, want it to name the author", text)
	}
	if !strings.Contains(text, "initial scaffold") {
		t.Errorf("box = %q, want the commit summary", text)
	}
}

func TestShowLineOutsideARepositorySetsAStatusMessageInstead(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.go")
	if err := os.WriteFile(path, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	e := state.New()
	edtest.InReadMode(t, e, "package main\n", 0, 0)
	e.SourceFile = path
	edtest.SingleWindow(e, 24, 80)

	blame.ShowLine(e)

	if e.Hov.Showing() {
		t.Error("ShowLine() outside a repository opened the hover box")
	}
	if e.StatusMsg == "" {
		t.Error("ShowLine() outside a repository left StatusMsg empty")
	}
}

func TestShowLineUsesTheCursorsOwnRowNotAlwaysTheFirstLine(t *testing.T) {
	dir := initRepo(t)
	path := commitFile(t, dir, "package main\n", "first")
	commitFile(t, dir, "package main\n\nfunc a() {}\n", "second")

	e := state.New()
	edtest.InReadMode(t, e, "package main\n\nfunc a() {}\n", 2, 0)
	e.SourceFile = path
	edtest.SingleWindow(e, 24, 80)

	blame.ShowLine(e)

	if text := e.Hov.Text(); !strings.Contains(text, "second") {
		t.Errorf("box = %q, want the commit that added row 2", text)
	}
}

func TestShowDiffOpensAGitShowPaneForTheCommit(t *testing.T) {
	dir := initRepo(t)
	path := commitFile(t, dir, "package main\n", "initial scaffold")

	e := state.New()
	edtest.InReadMode(t, e, "package main\n", 0, 0)
	e.SourceFile = path
	edtest.SingleWindow(e, 24, 80)

	blame.ShowDiff(e)
	t.Cleanup(func() { killAnyGitShowPane() })

	if len(view.List(e)) != 2 {
		t.Fatalf("windows = %d, want 2", len(view.List(e)))
	}
	w := view.Focused()
	if w.Entry.Terminal == nil {
		t.Fatal("ShowDiff() did not open a terminal-backed pane")
	}
	if w.Entry.Terminal.Exited() {
		t.Error("the git show pane exited immediately")
	}
	if e.Mode != state.TerminalMode {
		t.Errorf("mode = %v, want TerminalMode", e.Mode)
	}
}

func TestShowDiffOnAnUncommittedLineSetsAStatusMessage(t *testing.T) {
	dir := initRepo(t)
	path := commitFile(t, dir, "package main\n", "initial")
	if err := os.WriteFile(path, []byte("package main\n\nvar x int\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	e := state.New()
	edtest.InReadMode(t, e, "package main\n\nvar x int\n", 2, 0)
	e.SourceFile = path
	edtest.SingleWindow(e, 24, 80)

	blame.ShowDiff(e)
	t.Cleanup(func() { killAnyGitShowPane() })

	if e.StatusMsg == "" {
		t.Error("ShowDiff() on an uncommitted line left StatusMsg empty")
	}
	if len(view.List(e)) != 1 {
		t.Errorf("windows = %d, want 1 (no pane opened)", len(view.List(e)))
	}
}

func TestShowDiffOutsideARepositorySetsAStatusMessage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.go")
	if err := os.WriteFile(path, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	e := state.New()
	edtest.InReadMode(t, e, "package main\n", 0, 0)
	e.SourceFile = path
	edtest.SingleWindow(e, 24, 80)

	blame.ShowDiff(e)

	if e.StatusMsg == "" {
		t.Error("ShowDiff() outside a repository left StatusMsg empty")
	}
}
