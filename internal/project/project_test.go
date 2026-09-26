package project_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"github.com/ArditZubaku/tex/internal/project"
)

func initRepo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	if err := exec.Command("git", "-C", dir, "init", "-q").Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}

	return dir
}

func write(t *testing.T, dir, rel, content string) {
	t.Helper()

	path := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func track(t *testing.T, dir, rel string) {
	t.Helper()

	if err := exec.Command("git", "-C", dir, "add", rel).Run(); err != nil {
		t.Fatalf("git add %s: %v", rel, err)
	}
}

func TestListLeavesOutWhatGitignoreExcludes(t *testing.T) {
	dir := initRepo(t)
	write(t, dir, ".gitignore", "ignored.txt\nbuild/\n")
	write(t, dir, "tracked.go", "package main")
	track(t, dir, "tracked.go")
	write(t, dir, "untracked.go", "package main")
	write(t, dir, "ignored.txt", "scratch")
	write(t, dir, "build/output.bin", "binary")

	files, err := project.List(dir)
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"tracked.go", "untracked.go"}
	if !slices.Equal(files, want) {
		t.Errorf("List() = %v, want %v", files, want)
	}
}

func TestListStillLeavesOutDotfilesEvenWhenTracked(t *testing.T) {
	dir := initRepo(t)
	write(t, dir, ".golangci.yaml", "run: {}")
	track(t, dir, ".golangci.yaml")
	write(t, dir, "main.go", "package main")
	track(t, dir, "main.go")

	files, err := project.List(dir)
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"main.go"}
	if !slices.Equal(files, want) {
		t.Errorf("List() = %v, want %v", files, want)
	}
}

func TestListFallsBackToWalkingOutsideAnyRepository(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "solo.go", "package main")

	files, err := project.List(dir)
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"solo.go"}
	if !slices.Equal(files, want) {
		t.Errorf("List() = %v, want %v", files, want)
	}
}
