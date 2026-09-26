// Package project is the tree the file being edited sits in: where its root is
// and what files are under it.
package project

import (
	"context"
	"io/fs"
	"iter"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// A walk of a large tree costs more than a listing is worth, so it stops once it
// has more files than anybody scrolls through and lets the filter do the rest.
const maxFiles = 20000

// listTimeout bounds git the way a save or a grep does: the loop blocks on the
// picker opening, and a hung process must not hang it forever.
const listTimeout = 5 * time.Second

// Root is the directory the files are listed from: the repository the file
// being edited sits in, or its own directory when it is in none.
func Root(file string) string {
	dir, err := filepath.Abs(filepath.Dir(file))
	if err != nil {
		return "."
	}

	for at := dir; ; {
		if info, err := os.Stat(filepath.Join(at, ".git")); err == nil && info.IsDir() {
			return at
		}
		parent := filepath.Dir(at)
		if parent == at {
			return dir
		}
		at = parent
	}
}

// List is every file below root a picker is for: git's own idea of what is
// tracked or untracked-but-not-ignored, so that '.gitignore' is honoured the
// same way it already is for a grep. A root git cannot answer for - no binary
// on PATH, or no repository there at all - falls back to walking the tree
// whole, hidden entries aside.
func List(root string) ([]string, error) {
	files, err := gitFiles(root)
	if err != nil {
		if files, err = walkFiles(root); err != nil {
			return nil, err
		}
	}

	return filterHidden(files), nil
}

func gitFiles(root string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), listTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "-C", root, "ls-files", "--cached", "--others", "--exclude-standard", "-z")

	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	trimmed := strings.TrimSuffix(string(out), "\x00")
	if trimmed == "" {
		return nil, nil
	}

	return strings.Split(trimmed, "\x00"), nil
}

// walkFiles is what answers when git cannot: every file below root, save what
// a hidden directory holds - '.git' alone outweighs the rest of most trees -
// which is worth skipping before it is walked rather than after.
func walkFiles(root string) ([]string, error) {
	files := make([]string, 0, 256)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if name := entry.Name(); entry.IsDir() {
			if strings.HasPrefix(name, ".") && path != root {
				return fs.SkipDir
			}
			return nil
		}
		if len(files) >= maxFiles {
			return fs.SkipAll
		}

		if rel, err := filepath.Rel(root, path); err == nil {
			files = append(files, rel)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return files, nil
}

// filterHidden drops anything with a dot-prefixed path component regardless of
// where the list came from, then caps and orders the result the same way
// every time, git and the walk otherwise disagreeing on both.
func filterHidden(files []string) []string {
	kept := make([]string, 0, len(files))
	for _, rel := range files {
		if !hidden(rel) {
			kept = append(kept, rel)
		}
	}
	if len(kept) > maxFiles {
		kept = kept[:maxFiles]
	}
	sort.Strings(kept)

	return kept
}

func hidden(rel string) bool {
	for _, part := range strings.Split(rel, "/") {
		if strings.HasPrefix(part, ".") {
			return true
		}
	}

	return false
}

// Same says whether two paths name the same file, which is what keeps the file
// being edited out of a walk over the ones beside it.
func Same(a, b string) bool {
	left, err := filepath.Abs(a)
	if err != nil {
		return false
	}
	right, err := filepath.Abs(b)
	if err != nil {
		return false
	}

	return left == right
}

// HasPackage says whether the tree holds a directory of that name, which is
// what tells the package a qualifier names ('command.Run') from the value one a
// field is reached through ('b.file'): a package is a directory, and a value is
// not. Only the walk is paid for, since no file has to be read to answer it.
func HasPackage(of, name string) bool {
	files, err := List(Root(of))
	if err != nil {
		return false
	}

	for _, rel := range files {
		for dir := filepath.Dir(rel); dir != "."; dir = filepath.Dir(dir) {
			if filepath.Base(dir) == name {
				return true
			}
		}
	}

	return false
}

// A file is worth reading whole to look through, but not at the cost of
// pulling a tree of them into memory: anything larger than this is left to a
// real index.
const maxFileSize = 1 << 20

// Siblings is every file beside the one given, of the same kind, read whole:
// the reach 'gd', 'gr' and the workspace symbols all settle for. A file that
// cannot be read is skipped rather than reported, since a listing of the rest
// is still worth having.
func Siblings(of string) iter.Seq2[string, []byte] {
	return func(yield func(string, []byte) bool) {
		root := Root(of)
		files, err := List(root)
		if err != nil {
			return
		}

		ext := filepath.Ext(of)
		for _, rel := range files {
			path := filepath.Join(root, rel)
			if filepath.Ext(path) != ext || Same(path, of) {
				continue
			}

			info, err := os.Stat(path)
			if err != nil || info.Size() > maxFileSize {
				continue
			}
			content, err := os.ReadFile(path)
			if err != nil {
				continue
			}

			if !yield(path, content) {
				return
			}
		}
	}
}
