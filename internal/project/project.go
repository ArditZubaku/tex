// Package project is the tree the file being edited sits in: where its root is
// and what files are under it.
package project

import (
	"io/fs"
	"iter"
	"os"
	"path/filepath"
	"strings"
)

// A walk of a large tree costs more than a listing is worth, so it stops once it
// has more files than anybody scrolls through and lets the filter do the rest.
const maxFiles = 20000

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

// List walks the tree below root, leaving out what is hidden: a picker is
// for the files being worked on, and '.git' alone holds more than all of them.
func List(root string) ([]string, error) {
	files := make([]string, 0, 256)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if name := entry.Name(); strings.HasPrefix(name, ".") && path != root {
			if entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
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
