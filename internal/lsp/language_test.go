package lsp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// inProject is a file with the mark of a project of its kind above it, which is
// what a server needs before it will be started at all.
func inProject(t *testing.T, mark, name, content string) string {
	t.Helper()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, mark), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	return path
}

func TestEachLanguageStartsTheServerItNames(t *testing.T) {
	for _, one := range []struct {
		mark, name, program string
	}{
		{"go.mod", "main.go", "gopls"},
		{"Cargo.toml", "main.rs", "rust-analyzer"},
		{"tsconfig.json", "main.ts", "typescript-language-server"},
		{"package.json", "main.js", "typescript-language-server"},
	} {
		t.Run(one.name, func(t *testing.T) {
			h := reconciling(t)
			path := inProject(t, one.mark, one.name, "\n")

			Sync([]File{{Path: path, Buf: opening(t, path)}})

			if _, started := h.argv[one.program]; !started {
				t.Fatalf("%s started %v, want %s", one.name, h.argv, one.program)
			}
		})
	}
}

// It speaks over its standard input only when told to, so without the argument
// the handshake is never answered and nothing else here happens either.
func TestTheTypeScriptServerIsToldToUseItsStandardInput(t *testing.T) {
	h := reconciling(t)
	path := inProject(t, "tsconfig.json", "main.ts", "\n")

	Sync([]File{{Path: path, Buf: opening(t, path)}})

	if got := strings.Join(h.argv["typescript-language-server"], " "); got != "typescript-language-server --stdio" {
		t.Errorf("the server was started as %q", got)
	}
}

// One server answers for both, and the languageId is what tells it which — and
// whether the JSX in a .tsx file is parsed rather than read as a comparison.
func TestTheLanguageAFileIsOpenedAsIsItsOwn(t *testing.T) {
	for name, want := range map[string]string{
		"main.ts":  "typescript",
		"main.tsx": "typescriptreact",
		"main.js":  "javascript",
		"main.jsx": "javascriptreact",
		"main.rs":  "rust",
	} {
		t.Run(name, func(t *testing.T) {
			mark := "package.json"
			if filepath.Ext(name) == ".rs" {
				mark = "Cargo.toml"
			}

			h := reconciling(t)
			path := inProject(t, mark, name, "\n")
			b := opening(t, path)
			h.up(File{Path: path, Buf: b})

			if got := openedIn(t, h.sent()).LanguageID; got != want {
				t.Errorf("%s was opened as %q, want %q", name, got, want)
			}
		})
	}
}

// A Go backend and the TypeScript frontend beside it are two servers, and
// neither is asked about the other's files.
func TestTwoLanguagesOpenAtOnceAreTwoServers(t *testing.T) {
	h := reconciling(t)

	dir := t.TempDir()
	for _, mark := range []string{"go.mod", "package.json"} {
		if err := os.WriteFile(filepath.Join(dir, mark), []byte("module x\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	goFile, tsFile := filepath.Join(dir, "main.go"), filepath.Join(dir, "main.ts")
	for _, path := range []string{goFile, tsFile} {
		if err := os.WriteFile(path, []byte("\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	files := []File{
		{Path: goFile, Buf: opening(t, goFile)},
		{Path: tsFile, Buf: opening(t, tsFile)},
	}
	Sync(files)

	if len(servers) != 2 {
		t.Fatalf("%d servers are running, want one per language", len(servers))
	}

	for _, name := range []string{"gopls", "typescript-language-server"} {
		h.shakeWith(h.end(name))
	}
	Sync(files)

	if got := openedIn(t, h.end("gopls").next()).URI; got != FileURI(goFile) {
		t.Errorf("gopls was handed %q, want the Go file", got)
	}
	if got := openedIn(t, h.end("typescript-language-server").next()).URI; got != FileURI(tsFile) {
		t.Errorf("the TypeScript server was handed %q, want the TypeScript file", got)
	}
}

// A second server for one language is another few hundred megabytes of the same
// type information, which is what 'gd' into the standard library would cost.
func TestASecondRootForOneLanguageIsNotASecondServer(t *testing.T) {
	h := reconciling(t)
	first := project(t, "main.go", "package main\n")
	second := project(t, "other.go", "package other\n")

	h.up(File{Path: first, Buf: opening(t, first)})
	openedIn(t, h.sent())

	Sync([]File{
		{Path: first, Buf: opening(t, first)},
		{Path: second, Buf: opening(t, second)},
	})

	if len(servers) != 1 {
		t.Fatalf("%d servers are running, want the one", len(servers))
	}
	if held(second) != nil {
		t.Error("a file outside the root the server was started in was sent to it anyway")
	}
}

// The nearest of them is the project: a package inside a monorepo is its own
// root, which is where its own tsconfig applies.
func TestTheNearestMarkIsTheProjectRoot(t *testing.T) {
	h := reconciling(t)

	top := t.TempDir()
	if err := os.WriteFile(filepath.Join(top, "package.json"), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	inner := filepath.Join(top, "apps", "web")
	if err := os.MkdirAll(inner, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inner, "tsconfig.json"), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(inner, "main.ts")
	if err := os.WriteFile(path, []byte("\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	Sync([]File{{Path: path, Buf: opening(t, path)}})

	var asked initializeParams
	if err := json.Unmarshal(h.sent().Params, &asked); err != nil {
		t.Fatal(err)
	}
	if asked.RootURI != FileURI(inner) {
		t.Errorf("the server was started in %q, want the nearest project %q", asked.RootURI, FileURI(inner))
	}
}

func TestAFileOfNoLanguageStartsNothing(t *testing.T) {
	reconciling(t)
	path := inProject(t, "package.json", "notes.md", "nothing to see\n")

	Sync([]File{{Path: path, Buf: opening(t, path)}})

	if len(servers) != 0 {
		t.Fatalf("%d servers are running for a file no server answers for", len(servers))
	}
}
