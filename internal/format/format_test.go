package format

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeTool puts a program of that name on the PATH, running the shell body it
// is given with the file to format left in "$f".
func fakeTool(t *testing.T, name, body string) {
	t.Helper()

	dir := t.TempDir()
	script := "#!/bin/sh\nfor f; do :; done\n" + body + "\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", dir)
	Reset()
}

func sourceFile(t *testing.T, name, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	return path
}

func readBack(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	return string(content)
}

func TestRunFormatsTheFileInPlace(t *testing.T) {
	fakeTool(t, "gofumpt", `printf 'package main\n' > "$f"`)
	path := sourceFile(t, "a.go", "package   main\n")

	result, err := Run(path)
	if err != nil {
		t.Fatal(err)
	}
	if result.Name != "gofumpt" || !result.Changed {
		t.Fatalf("result = %+v, want gofumpt having changed the file", result)
	}
	if got := readBack(t, path); got != "package main\n" {
		t.Errorf("file = %q", got)
	}
}

func TestRunReportsAFileTheFormatterLeftAlone(t *testing.T) {
	fakeTool(t, "gofumpt", ":")
	path := sourceFile(t, "a.go", "package main\n")

	result, err := Run(path)
	if err != nil {
		t.Fatal(err)
	}
	if result.Name != "gofumpt" || result.Changed {
		t.Fatalf("result = %+v, want gofumpt having changed nothing", result)
	}
}

func TestRunReportsWhatTheFormatterRefused(t *testing.T) {
	fakeTool(t, "gofumpt", `echo "a.go:2:1: expected declaration" >&2; exit 2`)
	path := sourceFile(t, "a.go", "package main\nfunc(\n")

	_, err := Run(path)

	var refused *Error
	if !errors.As(err, &refused) {
		t.Fatalf("err = %v, want a format.Error", err)
	}
	if refused.Name != "gofumpt" || refused.Reason != "a.go:2:1: expected declaration" {
		t.Errorf("error = %+v, want the formatter's own first line", refused)
	}
	if got := readBack(t, path); got != "package main\nfunc(\n" {
		t.Errorf("file = %q, want the refused file left as it was", got)
	}
}

func TestRunTruncatesALongComplaint(t *testing.T) {
	fakeTool(t, "gofumpt", `printf '%0.sx' $(seq 200) >&2; exit 2`)
	path := sourceFile(t, "a.go", "package main\n")

	_, err := Run(path)
	if err == nil {
		t.Fatal("want an error")
	}
	if len(err.Error()) > 120 {
		t.Errorf("message is %d long, want it short enough for the status line", len(err.Error()))
	}
}

func TestRunLeavesALanguageWithNoFormatterAlone(t *testing.T) {
	fakeTool(t, "gofumpt", `printf 'formatted\n' > "$f"`)
	path := sourceFile(t, "notes.md", "# hi\n")

	result, err := Run(path)
	if err != nil {
		t.Fatal(err)
	}
	if result != (Result{}) {
		t.Errorf("result = %+v, want nothing having run", result)
	}
	if got := readBack(t, path); got != "# hi\n" {
		t.Errorf("file = %q", got)
	}
}

func TestRunLeavesALanguageWhoseFormatterIsNotInstalledAlone(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	Reset()
	path := sourceFile(t, "a.rs", "fn  main() {}\n")

	result, err := Run(path)
	if err != nil {
		t.Fatal(err)
	}
	if result != (Result{}) {
		t.Errorf("result = %+v, want nothing having run", result)
	}
}

func TestRunTakesTheFirstFormatterInstalled(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"goimports", "gofmt"} {
		script := "#!/bin/sh\nfor f; do :; done\nprintf '" + name + "\\n' > \"$f\"\n"
		if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir)
	Reset()

	path := sourceFile(t, "a.go", "package main\n")
	result, err := Run(path)
	if err != nil {
		t.Fatal(err)
	}
	if result.Name != "goimports" {
		t.Errorf("ran %q, want the preferred goimports", result.Name)
	}
}

func TestRunSkipsAFileTooBigToFormat(t *testing.T) {
	fakeTool(t, "gofumpt", `printf 'formatted\n' > "$f"`)
	content := strings.Repeat("// padding\n", (maxSize/11)+1)
	path := sourceFile(t, "a.go", content)

	result, err := Run(path)
	if err != nil {
		t.Fatal(err)
	}
	if result != (Result{}) {
		t.Errorf("result = %+v, want nothing having run", result)
	}
	if got := readBack(t, path); got != content {
		t.Error("the oversized file was rewritten")
	}
}

func TestRunPrefersTheFormatterInstalledInTheProject(t *testing.T) {
	fakeTool(t, "prettier", `printf 'from the path\n' > "$f"`)

	root := t.TempDir()
	local := filepath.Join(root, "node_modules", ".bin")
	if err := os.MkdirAll(local, 0o700); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\nfor f; do :; done\nprintf 'from the project\\n' > \"$f\"\n"
	if err := os.WriteFile(filepath.Join(local, "prettier"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(root, "src", "a.ts")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("let x=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := Run(path); err != nil {
		t.Fatal(err)
	}
	if got := readBack(t, path); got != "from the project\n" {
		t.Errorf("file = %q, want the project's own prettier to have run", got)
	}
}

func TestTheSameFormatterIsOnlyLookedForOnce(t *testing.T) {
	fakeTool(t, "gofumpt", ":")
	path := sourceFile(t, "a.go", "package main\n")

	if _, err := Run(path); err != nil {
		t.Fatal(err)
	}

	// with the PATH emptied the answer can only come from what the first run left
	t.Setenv("PATH", t.TempDir())
	result, err := Run(path)
	if err != nil {
		t.Fatal(err)
	}
	if result.Name != "gofumpt" {
		t.Errorf("ran %q, want the looked-up gofumpt remembered", result.Name)
	}
}
