// Package explorer is the file tree as the editor drives it: what '<leader>e'
// opens, the keys it reads while it is up, and the file it hands to ':e'.
package explorer

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nsf/termbox-go"

	"github.com/ArditZubaku/tex/internal/editor/command"
	"github.com/ArditZubaku/tex/internal/editor/filetree"
	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/ArditZubaku/tex/internal/editor/view"
)

// Open is '<leader>e', which toggles: the second press is what closes
// the explorer again.
func Open(e *state.Editor) {
	if e.ExplorerOpen {
		Close(e)
		return
	}

	dir, err := filepath.Abs(filepath.Dir(e.SourceFile))
	if err != nil {
		dir = "."
	}

	if goTo(e, dir, filepath.Base(e.SourceFile)) {
		e.ExplorerOpen, e.Mode = true, state.ExplorerMode
	}
}

func Close(e *state.Editor) {
	e.ExplorerOpen, e.Mode = false, state.ReadMode
	e.ClampCol()
}

func goTo(e *state.Editor, dir, on string) bool {
	if err := e.Exp.Go(dir, on); err != nil {
		e.StatusMsg = "E484: Can't open file " + dir
		return false
	}

	return true
}

func leaveDir(e *state.Editor) {
	if err := e.Exp.Leave(); err != nil {
		e.StatusMsg = "E484: Can't open file " + filepath.Dir(e.Exp.Dir())
	}
}

// The prompt the status line is handed is the same one every time; the
// delimiter is what says which of them Enter is settling.
const (
	searchPrompt = '/'
	createPrompt = 'a'
	pastePrompt  = 'p'
)

// startSearch is '/', which hands the status line to the same prompt
// the buffer's search uses; what is typed there narrows the listing as it goes.
func startSearch(e *state.Editor) { e.StartPrompt(searchPrompt) }

// startCreate is 'a', which asks for a name to make in the directory being
// listed.
func startCreate(e *state.Editor) { e.StartLabelledPrompt(createPrompt, "new: ", "") }

// Typing is each key of a prompt opened over the explorer: the search narrows
// the listing as the pattern is typed, so that what is on screen is always what
// Enter would settle on. The others have nothing to show until Enter.
func Typing(e *state.Editor, delimiter rune, input string) {
	if delimiter == searchPrompt {
		Filter(e, input)
	}
}

func Submit(e *state.Editor, delimiter rune, input string) {
	switch delimiter {
	case createPrompt:
		create(e, input)
	case pastePrompt:
		paste(e, input)
	default:
		Filter(e, input)
	}
}

// Cancel is Esc: the pattern typed so far goes, along with the filter it was
// narrowing, while a name left unfinished leaves the listing as it found it.
func Cancel(e *state.Editor, delimiter rune) {
	if delimiter == searchPrompt {
		Filter(e, "")
	}
}

func Filter(e *state.Editor, filter string) { e.Exp.Filter(filter) }

// create is what 'a' does with the name it was given, taken relative to the
// directory being listed: a trailing '/' makes a directory and anything else a
// file, with the directories named on the way to either made as well. The
// listing moves to whichever directory holds what was made, with it selected.
func create(e *state.Editor, input string) {
	name := strings.TrimSpace(input)
	if name == "" {
		return
	}

	path := e.Exp.Path(name)
	if _, err := os.Lstat(path); err == nil {
		e.StatusMsg = "E13: File exists: " + path
		return
	}

	if !makePath(e, path, strings.HasSuffix(name, "/")) {
		return
	}
	if goTo(e, filepath.Dir(path), filepath.Base(path)) {
		e.StatusMsg = fmt.Sprintf("%q created", path)
	}
}

func makePath(e *state.Editor, path string, dir bool) bool {
	if dir {
		return makeDir(e, path)
	}
	if !makeDir(e, filepath.Dir(path)) {
		return false
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		e.StatusMsg = "E212: Can't open file for writing: " + path
		return false
	}

	return file.Close() == nil
}

func makeDir(e *state.Editor, dir string) bool {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		e.StatusMsg = "E739: Cannot create directory: " + dir
		return false
	}

	return true
}

// selected is the entry 'yy', 'p' and 'd' work on. The parent entry is a move
// rather than a file, so none of them touch it.
func selected(e *state.Editor) (path, name string, ok bool) {
	entry, ok := e.Exp.Selected()
	if !ok || entry.Name == filetree.ParentDir {
		return "", "", false
	}

	return e.Exp.Path(entry.Name), entry.Name, true
}

// yank is 'yy': the file under the selection picked up for 'p' to write out
// again. A directory is a tree to walk rather than a file to copy, so it is
// refused rather than half done.
func yank(e *state.Editor) {
	path, name, ok := selected(e)
	if !ok {
		return
	}

	info, err := os.Stat(path)
	switch {
	case err != nil:
		e.StatusMsg = "E484: Can't open file " + path
	case info.IsDir():
		e.StatusMsg = "E17: " + path + " is a directory"
	default:
		e.Exp.Yank(path)
		e.StatusMsg = fmt.Sprintf("%q yanked", name)
	}
}

// startPaste is 'p', which asks what to call the copy before it writes one.
// The name it opens on is the yanked file's own, which is the answer everywhere
// but the directory it came from.
func startPaste(e *state.Editor) {
	yanked := e.Exp.Yanked()
	if yanked == "" {
		e.StatusMsg = "E353: Nothing in register"
		return
	}

	e.StartLabelledPrompt(pastePrompt, "paste as: ", filepath.Base(yanked))
}

// paste copies the yanked file under the name typed, which is taken relative to
// the directory being listed the way the one 'a' asks for is.
func paste(e *state.Editor, input string) {
	from, name := e.Exp.Yanked(), strings.TrimSpace(input)
	if from == "" || name == "" {
		return
	}

	path := e.Exp.Path(name)
	if _, err := os.Lstat(path); err == nil {
		e.StatusMsg = "E13: File exists: " + path
		return
	}
	if _, err := os.Stat(from); err != nil {
		e.StatusMsg = "E484: Can't open file " + from
		return
	}

	if !makeDir(e, filepath.Dir(path)) {
		return
	}
	if err := copyFile(from, path); err != nil {
		e.StatusMsg = "E212: Can't open file for writing: " + path
		return
	}
	if goTo(e, filepath.Dir(path), filepath.Base(path)) {
		e.StatusMsg = fmt.Sprintf("%q copied to %q", filepath.Base(from), path)
	}
}

// copyFile keeps the mode it read the file with, so that a copied script is
// still one.
func copyFile(from, to string) error {
	src, err := os.Open(from)
	if err != nil {
		return err
	}
	defer func() { _ = src.Close() }()

	info, err := src.Stat()
	if err != nil {
		return err
	}

	dst, err := os.OpenFile(to, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(dst, src); err != nil {
		_ = dst.Close()

		return err
	}

	return dst.Close()
}

// clearFilter is Esc: it drops the filter it finds, and closes the explorer
// when there is none left to drop.
func clearFilter(e *state.Editor) {
	if e.Exp.Filtered() == "" {
		Close(e)
		return
	}

	Filter(e, "")
}

func openSelected(e *state.Editor) {
	entry, ok := e.Exp.Selected()
	if !ok {
		return
	}
	if entry.Name == filetree.ParentDir {
		leaveDir(e)
		return
	}

	// the entry's own flag is false for a symlink to a directory, so what to do
	// with the one being opened is worth the single stat
	path := e.Exp.Path(entry.Name)
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		goTo(e, path, "")
		return
	}

	if command.Edit(e, path, false) {
		Close(e)
	}
}

func toggleHidden(e *state.Editor) {
	if err := e.Exp.ToggleHidden(); err != nil {
		e.StatusMsg = "E484: Can't open file " + e.Exp.Dir()
	}
}

func down(e *state.Editor)   { e.Exp.Move(1) }
func up(e *state.Editor)     { e.Exp.Move(-1) }
func top(e *state.Editor)    { e.Exp.Top() }
func bottom(e *state.Editor) { e.Exp.Bottom() }

// A half screen of the listing, which is what Ctrl-D and Ctrl-U move by, the
// same fraction of the window they move the buffer by.
func pageDown(e *state.Editor) { e.Exp.Move(Page(e)) }
func pageUp(e *state.Editor)   { e.Exp.Move(-Page(e)) }

func Page(e *state.Editor) int {
	return max((e.Rows-filetree.HeaderRows)/2, 1)
}

var actions = map[rune]func(*state.Editor){
	'j': down,
	'k': up,
	'l': openSelected,
	'h': leaveDir,
	'-': leaveDir,
	'g': top,
	'G': bottom,
	'H': toggleHidden,
	'a': startCreate,
	'y': startYank,
	'p': startPaste,
	'/': startSearch,
	'q': Close,
}

var specialActions = map[termbox.Key]func(*state.Editor){
	termbox.KeyEnter:      openSelected,
	termbox.KeyEsc:        clearFilter,
	termbox.KeyArrowDown:  down,
	termbox.KeyArrowUp:    up,
	termbox.KeyArrowRight: openSelected,
	termbox.KeyArrowLeft:  leaveDir,
	termbox.KeyCtrlD:      pageDown,
	termbox.KeyCtrlU:      pageUp,
	termbox.KeyPgdn:       pageDown,
	termbox.KeyPgup:       pageUp,
}

// chords are the keys that mean something only once a second one has arrived:
// '<leader>e' closes the explorer again, and 'yy' picks up the file under the
// selection.
var chords = map[[2]rune]func(*state.Editor){
	{' ', 'e'}: Open,
	{'y', 'y'}: yank,
}

func Key(e *state.Editor, event termbox.Event) {
	if event.Key == termbox.KeySpace {
		startChord(e, ' ')
		return
	}

	if event.Ch != 0 {
		runeKey(e, event.Ch)
		return
	}

	e.PendingKeys = e.PendingKeys[:0]
	if action, ok := view.MoveKeys[event.Key]; ok {
		leaveFor(e, func() { action(e) })
		return
	}
	if action, ok := specialActions[event.Key]; ok {
		action(e)
	}
}

// runeKey is a key that is a rune: the second half of a chord that is waiting
// for one, and a command of its own otherwise. A chord no second key completes
// swallows that key rather than running it.
func runeKey(e *state.Editor, ch rune) {
	pending, waiting := pendingChord(e)
	e.PendingKeys = e.PendingKeys[:0]

	if waiting {
		if action, ok := chords[[2]rune{pending, ch}]; ok {
			action(e)
		}
		return
	}
	if action, ok := actions[ch]; ok {
		action(e)
	}
}

func startChord(e *state.Editor, key rune) {
	e.PendingKeys, e.PendingTime = append(e.PendingKeys[:0], key), time.Now()
}

// startYank is the first 'y' of 'yy', a chord the way '<leader>e' is.
func startYank(e *state.Editor) { startChord(e, 'y') }

func pendingChord(e *state.Editor) (rune, bool) {
	if len(e.PendingKeys) == 0 || time.Since(e.PendingTime) >= state.ChordTimeout {
		return 0, false
	}

	return e.PendingKeys[0], true
}

// leaveFor is Ctrl-hjkl out of the tree: the window it lands in is
// showing a buffer, so the explorer is left behind the way opening a file from
// it leaves it. A move with no window that way changes nothing.
func leaveFor(e *state.Editor, move func()) {
	was := view.CurrentWindow(e)
	move()
	if view.Focused() != was {
		Close(e)
	}
}

func Draw(e *state.Editor)          { e.Exp.Draw(e.WindowArea(), &e.Palette) }
func CursorRow(e *state.Editor) int { return e.Exp.CursorRow(e.WindowArea()) }
func Status(e *state.Editor) string { return e.Exp.Status() }
