package lsp

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/ArditZubaku/tex/internal/buffer"
)

// A File is what the editor says about one buffer it holds open, and all the
// reconciling needs to know about it. The buffer is read on the loop's
// goroutine and nowhere else: reading a line moves its window and its cache.
type File struct {
	Path     string
	Buf      *buffer.Buffer
	Modified bool
}

// The servers the editor knows how to talk to, what marks the root of a project
// each wants to be started in, and what each calls the language. Without a root
// above the file every scratch .go file is answered with "no packages found for
// open file", drawn as an error across its first line.
var (
	servers   = map[string]string{".go": "gopls"}
	rootMarks = map[string]string{".go": "go.mod"}
	languages = map[string]string{".go": "go"}
)

// A document is sent no more often than this. Typing outruns any server, and
// every send it did not need is a package parsed again for nothing.
// Deliberately longer than termbox's own 100ms wait on an escape sequence, so
// that waking the loop cannot keep a lone Esc from resolving.
const throttle = 200 * time.Millisecond

var (
	client *Client
	wake   func()
	root   string
	docs   = map[string]*document{}

	// A file too large to send is left alone for the rest of the session rather
	// than rendered again on every change: at that size the rendering costs
	// more than the answers are worth, and a file rarely crosses back.
	refused = map[string]bool{}

	projectRoots = map[string]string{}

	// dialFor is how a server is started, which a test replaces with a pair of
	// pipes it drives the far end of.
	dialFor = Server

	// starts is the line index WriteLines builds as it goes, kept so that
	// rendering a document does not allocate one every time.
	starts []int64

	waking time.Time

	// A server that died is reported once. Nothing is restarted: a retry on
	// every frame is a process started sixty times a second.
	toldGone bool
)

// Gone is a server that died mid-session, so that what it had said can be
// dropped and the editor can say so. A server that was never installed is not
// reported: the editor going on doing what it does without one is the point of
// the fallback, not something to interrupt anybody about.
var Gone func(err error)

// Wake is where a goroutine off the loop asks for a frame, and nothing here
// does anything until it is set. That is what keeps a language server out of
// every test that drives the dispatcher directly instead of running the loop.
func Wake(ask func()) { wake = ask }

// Poll acts on everything the server has said since the last frame.
func Poll() {
	if client != nil {
		client.Poll()
	}
}

// Sync is one pass over what the editor holds open, against what the server was
// told. It is a reconciler rather than a hook on each edit because there are
// two dozen places text changes and half a dozen ways a buffer joins or leaves
// the list, and the one that got forgotten would leave a document silently,
// permanently stale — which shows up as diagnostics on the wrong lines.
func Sync(files []File) {
	if wake == nil {
		return
	}

	for _, file := range files {
		reconcile(file)
	}
	closeGone(files)
}

// Ready says whether there is a server up that holds this file, which is what a
// feature asks before using one instead of reading the text itself.
func Ready(path string) bool {
	if client == nil || !client.Ready() {
		return false
	}
	_, open := docs[FileURI(path)]

	return open
}

// PositionEncoding is how the running server counts columns, chosen during the
// handshake. With no server it is UTF-16, the protocol's own default, which is
// what anything left over would have been counted in anyway.
func PositionEncoding() Encoding {
	if client == nil {
		return UTF16
	}

	return client.Encoding()
}

// Stop is the editor going.
func Stop() {
	if client == nil {
		return
	}

	client.Stop()
	client, root = nil, ""
	clear(docs)
}

// Reset puts the package back to never having run, so that one test is not
// answered out of another one's server, paths or documents.
func Reset() {
	Stop()

	wake, waking, starts, toldGone = nil, time.Time{}, nil, false
	Gone, Published = nil, nil
	dialFor = Server
	clear(resolvedPaths)
	clear(fileURIs)
	clear(projectRoots)
	clear(refused)
	clear(found)
}

func reconcile(file File) {
	name, ok := servers[filepath.Ext(file.Path)]
	if !ok || refused[file.Path] {
		return
	}

	at := projectRoot(file.Path)
	if at == "" || !running(name, at) {
		return
	}

	uri := FileURI(file.Path)
	doc, open := docs[uri]
	if !open {
		opened(uri, file)

		return
	}

	if doc.revision != file.Buf.Revision() && !changed(doc, file) {
		return // held back by the throttle, or refused: the save waits with it
	}
	if doc.modified && !file.Modified {
		client.Notify(methodDidSave, identParams{TextDocument: ident{URI: doc.uri}})
	}
	doc.modified = file.Modified
}

func opened(uri string, file File) {
	body, ok := rendered(file)
	if !ok {
		return
	}

	doc := &document{
		uri:      uri,
		version:  1,
		revision: file.Buf.Revision(),
		modified: file.Modified,
		sentAt:   now(),
	}
	docs[uri] = doc
	client.Notify(methodDidOpen, doc.opened(languages[filepath.Ext(file.Path)], body))
}

func changed(doc *document, file File) bool {
	if waited := now().Sub(doc.sentAt); waited < throttle {
		armWake(throttle - waited)

		return false
	}

	body, ok := rendered(file)
	if !ok {
		delete(docs, doc.uri)
		client.Notify(methodDidClose, identParams{TextDocument: ident{URI: doc.uri}})

		return false
	}

	doc.version++
	doc.revision, doc.sentAt = file.Buf.Revision(), now()
	client.Notify(methodDidChange, doc.changed(body))

	return true
}

func rendered(file File) (string, bool) {
	body, err := text(file.Buf)
	if err != nil {
		refused[file.Path] = true

		return "", false
	}

	return body, true
}

// Every URI the buffer list no longer holds is closed, which covers ':bd',
// closing the others and quitting at once, with nothing to forget at any of
// them.
func closeGone(files []File) {
	for uri := range docs {
		if stillOpen(files, uri) {
			continue
		}

		delete(docs, uri)
		client.Notify(methodDidClose, identParams{TextDocument: ident{URI: uri}})
	}
}

func stillOpen(files []File, uri string) bool {
	for _, file := range files {
		if FileURI(file.Path) == uri {
			return true
		}
	}

	return false
}

// A server is started once. One that would not come up, or that died, is not
// tried again: a retry on every frame is a process started sixty times a second.
func running(name, at string) bool {
	if client == nil {
		root, client = at, New(dialFor(name), wake)
		client.Handler = handle
		if err := client.Start(at); err != nil {
			return false
		}
	}
	if err := client.Err(); err != nil {
		clear(docs)
		reportGone(err)

		return false
	}

	// One server, one root: a file outside it would be answered about the
	// module it is not in, which is worse than not being answered at all.
	return client.Ready() && at == root
}

func reportGone(err error) {
	if toldGone || Gone == nil || errors.Is(err, ErrNotInstalled) {
		return
	}

	toldGone = true
	Gone(err)
}

// A change the throttle held back needs a frame of its own to go out on, since
// the next one may be a keystroke away or an hour away.
func armWake(after time.Duration) {
	if wake == nil || now().Before(waking) {
		return
	}

	waking = now().Add(after)
	time.AfterFunc(after, wake)
}

func projectRoot(path string) string {
	if at, ok := projectRoots[path]; ok {
		return at
	}

	at := ""
	if mark, ok := rootMarks[filepath.Ext(path)]; ok {
		at = above(path, mark)
	}
	projectRoots[path] = at

	return at
}

func above(path, mark string) string {
	dir, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return ""
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, mark)); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
