// Package watch is the editor noticing a file changed from outside it: another
// program's write landing while tex still has the file open. It knows nothing
// of buffers or entries, only of paths and which of them moved.
package watch

import (
	"log/slog"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"
)

var (
	wake func()

	mu      sync.Mutex
	watcher *fsnotify.Watcher
	refs    map[string]int    // watched directory -> paths under it still wanted
	dirOf   map[string]string // watched path -> its directory, for Remove and for run's filter
	changed map[string]struct{}
)

// Wake is how watch tells the editor's loop a frame is owed, exactly as
// internal/editor/terminal's own Wake does.
func Wake(ask func()) { wake = ask }

// ensure starts the watcher and its goroutine on the first path anybody asks
// to watch, rather than unconditionally at startup when nothing may ever open
// a real file at all.
func ensure() *fsnotify.Watcher {
	if watcher != nil {
		return watcher
	}

	w, err := fsnotify.NewWatcher()
	if err != nil {
		slog.Error("Failed to start the file watcher", "error", err)
		return nil
	}

	watcher = w
	refs = map[string]int{}
	dirOf = map[string]string{}
	changed = map[string]struct{}{}
	go run(w)

	return w
}

// Add watches path's directory rather than path itself, which is what
// survives the save-through-rename tex's own Buffer.Save, and most other
// editors, write through: the inode fsnotify opened is the one that gets
// unlinked, not the name a buffer has open. Watching the same directory for a
// second path is a ref-counted no-op rather than a second fsnotify.Add.
func Add(path string) {
	mu.Lock()
	defer mu.Unlock()

	w := ensure()
	if w == nil {
		return
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return
	}
	if _, already := dirOf[abs]; already {
		return
	}

	dir := filepath.Dir(abs)
	if refs[dir] == 0 {
		if err := w.Add(dir); err != nil {
			slog.Error("Failed to watch directory", "dir", dir, "error", err)
			return
		}
	}
	refs[dir]++
	dirOf[abs] = dir
}

// Remove drops a path a caller no longer has open. A path never added, or
// removed twice, is left alone rather than treated as an error: a buffer list
// closing an entry it never got to watch is routine, not a bug.
func Remove(path string) {
	mu.Lock()
	defer mu.Unlock()

	if watcher == nil {
		return
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return
	}
	dir, ok := dirOf[abs]
	if !ok {
		return
	}

	delete(dirOf, abs)
	delete(changed, abs)
	refs[dir]--
	if refs[dir] <= 0 {
		delete(refs, dir)
		_ = watcher.Remove(dir)
	}
}

// run is the one goroutine that ever reads the watcher's channels, exactly as
// terminal's readLoop is the one that ever reads the pty.
func run(w *fsnotify.Watcher) {
	for {
		select {
		case event, ok := <-w.Events:
			if !ok {
				return
			}
			onEvent(event)

		case err, ok := <-w.Errors:
			if !ok {
				return
			}
			slog.Error("File watcher error", "error", err)
		}
	}
}

func onEvent(event fsnotify.Event) {
	if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) == 0 {
		return
	}

	abs, err := filepath.Abs(event.Name)
	if err != nil {
		return
	}

	mu.Lock()
	_, watched := dirOf[abs]
	if watched {
		changed[abs] = struct{}{}
	}
	mu.Unlock()

	// A directory can hold files nothing has open; only a wake for one that
	// does is worth a frame nobody asked for.
	if watched && wake != nil {
		wake()
	}
}

// Reset drops every watch and closes the watcher behind them, so a test
// starts with nothing watched rather than carrying over whatever the test
// before it opened. Closing it is what ends run's goroutine: Close makes
// fsnotify close its own channels, which is what run is waiting on.
func Reset() {
	mu.Lock()
	defer mu.Unlock()

	if watcher != nil {
		_ = watcher.Close()
	}
	watcher = nil
	refs, dirOf, changed = nil, nil, nil
}

// Changed drains every watched path with an event since the last call, which
// is Poll's own cue to look again at what is on disk.
func Changed() []string {
	mu.Lock()
	defer mu.Unlock()

	if len(changed) == 0 {
		return nil
	}

	out := make([]string, 0, len(changed))
	for path := range changed {
		out = append(out, path)
	}
	clear(changed)

	return out
}
