package keys_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/ArditZubaku/tex/internal/editor/edtest"
	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/ArditZubaku/tex/internal/editor/view"
)

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func initCommittedFile(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	gitRun(t, dir, "init", "-q")
	gitRun(t, dir, "config", "user.email", "a@example.com")
	gitRun(t, dir, "config", "user.name", "A Author")

	path := filepath.Join(dir, "f.go")
	if err := os.WriteFile(path, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "add", "f.go")
	gitRun(t, dir, "commit", "-q", "-m", "initial scaffold")

	return path
}

func TestLeaderGBShowsBlameForTheCurrentLine(t *testing.T) {
	path := initCommittedFile(t)

	e := state.New()
	edtest.InReadMode(t, e, "package main\n", 0, 0)
	e.SourceFile = path
	edtest.SingleWindow(e, 24, 80)

	edtest.Press(t, e, " gb")

	if !e.Hov.Showing() {
		t.Fatal(" gb did not open the hover box")
	}
}

func TestLeaderGSOpensTheDiffForTheCurrentLine(t *testing.T) {
	path := initCommittedFile(t)

	e := state.New()
	edtest.InReadMode(t, e, "package main\n", 0, 0)
	e.SourceFile = path
	edtest.SingleWindow(e, 24, 80)

	edtest.Press(t, e, " gs")
	t.Cleanup(func() {
		if w := view.Focused(); w.Entry.Terminal != nil {
			w.Entry.Terminal.Kill()
		}
	})

	w := view.Focused()
	if w.Entry.Terminal == nil {
		t.Fatal(" gs did not open a terminal-backed pane")
	}
	if e.Mode != state.TerminalMode {
		t.Errorf("mode = %v, want TerminalMode", e.Mode)
	}
}
