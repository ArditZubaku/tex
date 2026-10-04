package git_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/ArditZubaku/tex/internal/git"
)

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

func commitFile(t *testing.T, dir, content, summary string, when time.Time) string {
	t.Helper()

	path := filepath.Join(dir, "f.go")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, dir, "add", "f.go")

	stamp := when.Format(time.RFC3339)
	cmd := exec.Command("git", "-C", dir, "commit", "-q", "-m", summary)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_DATE="+stamp, "GIT_COMMITTER_DATE="+stamp)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}

	return path
}

func TestBlameReadsTheCommitThatOwnsTheLine(t *testing.T) {
	dir := initRepo(t)
	when := time.Date(2024, 3, 14, 9, 0, 0, 0, time.UTC)
	path := commitFile(t, dir, "package main\n", "initial scaffold", when)

	line, err := git.Blame(dir, path, 1)
	if err != nil {
		t.Fatal(err)
	}

	if line.Author != "A Author" {
		t.Errorf("Author = %q, want %q", line.Author, "A Author")
	}
	if line.Summary != "initial scaffold" {
		t.Errorf("Summary = %q, want %q", line.Summary, "initial scaffold")
	}
	if !line.When.Equal(when) {
		t.Errorf("When = %v, want %v", line.When, when)
	}
	if len(line.Hash) != 40 {
		t.Errorf("Hash = %q, want a 40-char sha", line.Hash)
	}
}

func TestBlameNamesTheLaterCommitOnceALineIsChanged(t *testing.T) {
	dir := initRepo(t)
	first := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	second := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	commitFile(t, dir, "package main\n\nfunc a() {}\n", "add a", first)
	path := commitFile(t, dir, "package main\n\nfunc b() {}\n", "swap a for b", second)

	line, err := git.Blame(dir, path, 3)
	if err != nil {
		t.Fatal(err)
	}

	if line.Summary != "swap a for b" {
		t.Errorf("Summary = %q, want %q", line.Summary, "swap a for b")
	}
	if !line.When.Equal(second) {
		t.Errorf("When = %v, want %v", line.When, second)
	}
}

func TestBlameReportsNotCommittedYetRatherThanAnError(t *testing.T) {
	dir := initRepo(t)
	path := commitFile(t, dir, "package main\n", "initial", time.Now())
	if err := os.WriteFile(path, []byte("package main\n\nvar x int\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	line, err := git.Blame(dir, path, 3)
	if err != nil {
		t.Fatal(err)
	}

	if line.Author != "Not Committed Yet" {
		t.Errorf("Author = %q, want %q", line.Author, "Not Committed Yet")
	}
}

func TestBlameFailsOutsideAnyRepository(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.go")
	if err := os.WriteFile(path, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := git.Blame(dir, path, 1); err == nil {
		t.Fatal("Blame() outside a repository: want an error, got nil")
	}
}
