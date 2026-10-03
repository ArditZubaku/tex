package watch

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func waitUntil(t *testing.T, cond func() bool) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out waiting for condition")
}

func changedContains(path string) bool {
	for _, p := range Changed() {
		if p == path {
			return true
		}
	}

	return false
}

func write(t *testing.T, path, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAddNoticesAWriteToTheWatchedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	write(t, path, "one")

	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}

	Add(path)
	t.Cleanup(func() { Remove(path) })

	write(t, path, "two")
	waitUntil(t, func() bool { return changedContains(abs) })
}

func TestAddIgnoresAWriteToAnUnwatchedFileInTheSameDirectory(t *testing.T) {
	dir := t.TempDir()
	watched, other := filepath.Join(dir, "watched.txt"), filepath.Join(dir, "other.txt")
	write(t, watched, "x")
	write(t, other, "x")

	Add(watched)
	t.Cleanup(func() { Remove(watched) })

	absOther, err := filepath.Abs(other)
	if err != nil {
		t.Fatal(err)
	}

	write(t, other, "y")
	time.Sleep(300 * time.Millisecond)

	if changedContains(absOther) {
		t.Fatal("watch surfaced a change to a file nothing asked for")
	}
}

func refsFor(t *testing.T, dir string) (int, bool) {
	t.Helper()

	abs, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	defer mu.Unlock()
	n, ok := refs[abs]

	return n, ok
}

func TestRemoveStopsWatchingTheDirectoryOnceNothingElseNeedsIt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "solo.txt")
	write(t, path, "x")

	Add(path)
	if n, ok := refsFor(t, dir); !ok || n != 1 {
		t.Fatalf("refs = %d, %v after Add, want 1, true", n, ok)
	}

	Remove(path)
	if _, ok := refsFor(t, dir); ok {
		t.Fatal("directory still refcounted after its only path was removed")
	}
}

func TestTwoWatchedFilesInOneDirectoryShareOneRef(t *testing.T) {
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a.txt"), filepath.Join(dir, "b.txt")
	write(t, a, "x")
	write(t, b, "x")

	Add(a)
	Add(b)
	if n, _ := refsFor(t, dir); n != 2 {
		t.Fatalf("refs = %d after two Adds, want 2", n)
	}

	Remove(a)
	if n, ok := refsFor(t, dir); !ok || n != 1 {
		t.Fatalf("refs = %d, %v after one Remove, want 1, true", n, ok)
	}

	Remove(b)
	if _, ok := refsFor(t, dir); ok {
		t.Fatal("directory still refcounted after both paths were removed")
	}
}
