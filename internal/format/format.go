// Package format is the formatter a saved file is run back through: which
// program formats the language it is written in, whether that program is
// installed, and rewriting the file with it.
package format

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// A tool is one formatter: the program to run and the arguments that make it
// rewrite a file in place, the path going on the end.
type tool struct {
	name string
	args []string
	node bool // found in the project's node_modules before the PATH
}

// tools lists each language's formatters in the order they are preferred, so
// that a project with gofumpt is formatted by it and one with only the
// toolchain's own gofmt still is. The first one installed wins; a language
// with none installed is left alone.
var tools = map[string][]tool{
	".go": goTools,

	".rs": {{name: "rustfmt", args: []string{"--edition", "2021"}}},

	".js": webTools, ".jsx": webTools, ".mjs": webTools, ".cjs": webTools,
	".ts": webTools, ".tsx": webTools, ".mts": webTools, ".cts": webTools,
}

var goTools = []tool{
	{name: "gofumpt", args: []string{"-w"}},
	{name: "goimports", args: []string{"-w"}},
	{name: "gofmt", args: []string{"-w"}},
}

var webTools = []tool{
	{name: "prettier", args: []string{"--write", "--log-level", "silent"}, node: true},
	{name: "biome", args: []string{"format", "--write"}, node: true},
}

// A formatter reads the whole file, which is the one thing the editor itself
// never does. Past this the wait would be felt, so a file that large is left
// as it was written.
const maxSize = 4 << 20

// The editor's loop blocks on the keyboard, so a formatter that hangs would
// hang the editor with it.
const timeout = 5 * time.Second

// Result is what a formatter did: which one ran, and whether it rewrote the
// file. A file whose language has no formatter installed comes back zero,
// which is what tells a caller that nothing ran.
type Result struct {
	Name    string
	Changed bool
}

// Run formats the file in place. The file is expected to hold what the buffer
// just wrote, and a caller that gets Changed back has to read it again.
func Run(path string) (Result, error) {
	before, err := os.Stat(path)
	if err != nil || before.Size() > maxSize {
		return Result{}, nil
	}

	run, bin := installed(path)
	if bin == "" {
		return Result{}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Every one of these reads the project's own configuration out of the
	// directories above the file, which it only finds from inside the tree.
	// Naming the file from in there also keeps the absolute path out of the
	// complaint a formatter makes, which has a status line to fit on.
	dir, name := filepath.Split(path)
	cmd := exec.CommandContext(ctx, bin, append(slices.Clone(run.args), name)...)
	cmd.Dir = dir

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return Result{Name: run.name}, &Error{Name: run.name, Reason: reason(stderr.String(), err)}
	}

	// A file that cannot be stat'd back is reported unchanged rather than
	// guessed at: rereading one that is no longer there would put an empty
	// buffer over what was just written.
	after, err := os.Stat(path)

	return Result{Name: run.name, Changed: err == nil && !sameFile(before, after)}, nil
}

// Error is a formatter refusing the file, which while it is being typed into
// is most often a syntax error rather than anything wrong with the setup.
type Error struct {
	Name   string
	Reason string
}

func (e *Error) Error() string { return e.Name + ": " + e.Reason }

// reason is the line of the formatter's complaint worth showing: the first,
// since that is the one naming where it stopped, short enough to sit on the
// status line beside the rest of the message.
func reason(stderr string, err error) string {
	const maxReason = 90

	first, _, _ := strings.Cut(strings.TrimSpace(stderr), "\n")
	if first == "" {
		first = err.Error()
	}
	if len(first) > maxReason {
		first = first[:maxReason] + "…"
	}

	return first
}

// sameFile is how a formatter that left the file alone is told from one that
// rewrote it, without reading either version back: none of them rewrites a
// file it had nothing to change, and one that does moves the modification time.
func sameFile(before, after os.FileInfo) bool {
	return before.Size() == after.Size() && before.ModTime().Equal(after.ModTime())
}

// installed is the first of the file's formatters that is actually there.
func installed(path string) (tool, string) {
	for _, run := range tools[strings.ToLower(filepath.Ext(path))] {
		if run.node {
			if bin := nodeBin(run.name, path); bin != "" {
				return run, bin
			}
		}
		if bin := lookPath(run.name); bin != "" {
			return run, bin
		}
	}

	return tool{}, ""
}

// found is what the PATH was last seen to hold. A formatter is looked for on
// every save and a miss costs a stat per directory of the PATH, so the answer
// is kept — which is why one installed mid-session is picked up on the next
// run of the editor rather than the next save.
var found = map[string]string{}

func lookPath(name string) string {
	if bin, ok := found[name]; ok {
		return bin
	}

	bin, err := exec.LookPath(name)
	if err != nil {
		bin = ""
	}
	found[name] = bin

	return bin
}

// nodeBin is where a JavaScript project keeps its formatter: installed into the
// tree rather than onto the PATH, which is where prettier almost always is.
func nodeBin(name, path string) string {
	dir, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return ""
	}
	if bin, ok := found[dir+"\x00"+name]; ok {
		return bin
	}

	bin := ""
	for at := dir; ; {
		candidate := filepath.Join(at, "node_modules", ".bin", name)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			bin = candidate
			break
		}
		parent := filepath.Dir(at)
		if parent == at {
			break
		}
		at = parent
	}
	found[dir+"\x00"+name] = bin

	return bin
}

// Reset drops what the lookups found, so that a test is not answered from
// another one's PATH.
func Reset() {
	clear(found)
}
