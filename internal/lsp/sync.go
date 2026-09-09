package lsp

import (
	"errors"
	"fmt"
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

// A language is everything the editor knows about having a server for one: the
// program to run and what it wants on its command line, what marks the root of
// a project of that kind, and what the protocol calls the language. Adding one
// is an entry in the table below and nothing else — everything above this is
// the protocol rather than the language, the completion trigger characters
// included, since those are the server's own.
type language struct {
	server []string
	marks  []string
	name   string
}

var languages = map[string]language{
	".go": {server: []string{"gopls"}, marks: []string{"go.mod"}, name: "go"},

	".rs": {server: []string{"rust-analyzer"}, marks: []string{"Cargo.toml"}, name: "rust"},

	".ts":  web("typescript"),
	".mts": web("typescript"),
	".cts": web("typescript"),
	".tsx": web("typescriptreact"),
	".js":  web("javascript"),
	".mjs": web("javascript"),
	".cjs": web("javascript"),
	".jsx": web("javascriptreact"),
}

// One server answers for JavaScript as readily as for TypeScript, and the
// languageId is what tells it which — and whether JSX is parsed. It speaks over
// its standard input only when told to.
func web(name string) language {
	return language{
		server: []string{"typescript-language-server", "--stdio"},
		// The nearest of these is the project: a package inside a monorepo is
		// its own root, which is where its own tsconfig applies.
		marks: []string{"tsconfig.json", "jsconfig.json", "package.json"},
		name:  name,
	}
}

// A document is sent no more often than this. Typing outruns any server, and
// every send it did not need is a package parsed again for nothing.
// Deliberately longer than termbox's own 100ms wait on an escape sequence, so
// that waking the loop cannot keep a lone Esc from resolving.
const throttle = 200 * time.Millisecond

// A server is one language server the editor has running. There is one per
// program rather than one in all: a Go backend and the TypeScript frontend
// beside it are two servers, and neither is asked about the other's files.
type server struct {
	client *Client
	name   string
	root   string

	// told marks a server whose death has been dealt with, so that what it had
	// said is dropped once and it is not started again. A retry on every frame
	// is a process started sixty times a second.
	told bool
}

var (
	servers = map[string]*server{}
	wake    func()
	docs    = map[string]*document{}

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

// Poll acts on everything the servers have said since the last frame.
func Poll() {
	for _, srv := range servers {
		srv.client.Poll()
	}
}

// Sync is one pass over what the editor holds open, against what the servers
// were told. It is a reconciler rather than a hook on each edit because there
// are two dozen places text changes and half a dozen ways a buffer joins or
// leaves the list, and the one that got forgotten would leave a document
// silently, permanently stale — which shows up as diagnostics on the wrong
// lines.
func Sync(files []File) {
	if wake == nil {
		return
	}

	for _, file := range files {
		reconcile(file)
	}
	closeGone(files)
}

// held is the document one path is open as, and nil where the file is not one
// any server was told about. Everything a feature asks about a file goes
// through it, since which server answers for a file is the document's to know.
func held(path string) *document { return docs[FileURI(path)] }

// Ready says whether there is a server up that holds this file, which is what a
// feature asks before using one instead of reading the text itself.
func Ready(path string) bool {
	doc := held(path)

	return doc != nil && doc.srv.client.Ready()
}

// Completing says whether the server holding this file offers completion, which
// is asked before a keystroke is allowed to turn into a request.
func Completing(path string) bool {
	doc := held(path)

	return doc != nil && doc.srv.client.Ready() && doc.srv.client.Completes()
}

// TriggerRune is a character the server holding this file asked to be woken on
// — the '.' that starts a selector, and whatever else the language has. It is
// checked against every rune typed, so it is a scan of a handful of runes
// rather than anything that allocates.
func TriggerRune(path string, ch rune) bool {
	doc := held(path)
	if doc == nil {
		return false
	}

	for _, trigger := range doc.srv.client.Triggers() {
		if trigger == ch {
			return true
		}
	}

	return false
}

// PositionEncoding is how the server holding this file counts columns, chosen
// during its own handshake — two servers may well have chosen differently. With
// no server it is UTF-16, the protocol's own default, which is what anything
// left over would have been counted in anyway.
func PositionEncoding(path string) Encoding {
	doc := held(path)
	if doc == nil {
		return UTF16
	}

	return doc.srv.client.Encoding()
}

// freshen is the document sent now rather than whenever the throttle next lets
// it: a completion is about the word as it stands this keystroke, and the
// reconciler is deliberately a fifth of a second behind that. A document that
// cannot be rendered is closed and refused, which is what reconcile would do
// with it on its own next pass.
func freshen(doc *document, b *buffer.Buffer) bool {
	if doc.revision == b.Revision() {
		return true
	}

	body, err := text(b)
	if err != nil {
		refused[Path(doc.uri)] = true
		doc.close()

		return false
	}

	doc.version++
	doc.revision, doc.sentAt = b.Revision(), now()
	doc.srv.client.Notify(methodDidChange, doc.changed(body))

	return true
}

// Stop is the editor going.
func Stop() {
	for name, srv := range servers {
		srv.client.Stop()
		delete(servers, name)
	}
	clear(docs)
}

// Reset puts the package back to never having run, so that one test is not
// answered out of another one's servers, paths or documents.
func Reset() {
	Stop()

	wake, waking, starts = nil, time.Time{}, nil
	Gone, Published = nil, nil
	dialFor = Server
	clear(resolvedPaths)
	clear(fileURIs)
	clear(projectRoots)
	clear(refused)
	clear(found)
}

func reconcile(file File) {
	lang, ok := languages[filepath.Ext(file.Path)]
	if !ok || refused[file.Path] {
		return
	}

	at := projectRoot(file.Path, lang.marks)
	if at == "" {
		return
	}

	srv := running(lang.server, at)
	if srv == nil {
		return
	}

	uri := FileURI(file.Path)
	doc, open := docs[uri]
	if !open {
		opened(uri, srv, lang.name, file)

		return
	}

	if doc.revision != file.Buf.Revision() && !changed(doc, file) {
		return // held back by the throttle, or refused: the save waits with it
	}
	if doc.modified && !file.Modified {
		doc.srv.client.Notify(methodDidSave, identParams{TextDocument: ident{URI: doc.uri}})
	}
	doc.modified = file.Modified
}

func opened(uri string, srv *server, name string, file File) {
	body, ok := rendered(file)
	if !ok {
		return
	}

	doc := &document{
		uri:      uri,
		srv:      srv,
		version:  1,
		revision: file.Buf.Revision(),
		modified: file.Modified,
		sentAt:   now(),
	}
	docs[uri] = doc
	srv.client.Notify(methodDidOpen, doc.opened(name, body))
}

func changed(doc *document, file File) bool {
	if waited := now().Sub(doc.sentAt); waited < throttle {
		armWake(throttle - waited)

		return false
	}

	body, ok := rendered(file)
	if !ok {
		doc.close()

		return false
	}

	doc.version++
	doc.revision, doc.sentAt = file.Buf.Revision(), now()
	doc.srv.client.Notify(methodDidChange, doc.changed(body))

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
	for uri, doc := range docs {
		if stillOpen(files, uri) {
			continue
		}
		doc.close()
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

// A server is started once, and pinned to the root it was started in. A file
// outside that root would be answered about the project it is not in, and a
// second server for it is another few hundred megabytes of the same language's
// type information — which is what 'gd' into the standard library would cost if
// the root were not pinned.
func running(argv []string, at string) *server {
	name := argv[0]

	srv, up := servers[name]
	if !up {
		srv = &server{client: New(dialFor(argv), wake), name: name, root: at}
		srv.client.Handler = handle
		servers[name] = srv

		if err := srv.client.Start(at); err != nil {
			return nil
		}
	}

	if err := srv.client.Err(); err != nil {
		srv.gone(err)

		return nil
	}
	if !srv.client.Ready() || at != srv.root {
		return nil
	}

	return srv
}

// gone is a server's death dealt with once: what it had said goes, and the
// editor is told which of them it was.
func (s *server) gone(err error) {
	if s.told {
		return
	}
	s.told = true

	for uri, doc := range docs {
		if doc.srv == s {
			delete(docs, uri)
		}
	}

	if Gone == nil || errors.Is(err, ErrNotInstalled) {
		return
	}
	Gone(fmt.Errorf("%s: %w", s.name, err))
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

func projectRoot(path string, marks []string) string {
	if at, ok := projectRoots[path]; ok {
		return at
	}

	at := above(path, marks)
	projectRoots[path] = at

	return at
}

// The nearest directory holding any of the marks is the project. Without a root
// above the file every scratch .go file is answered with "no packages found for
// open file", drawn as an error across its first line.
func above(path string, marks []string) string {
	dir, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return ""
	}

	for {
		for _, mark := range marks {
			if _, err := os.Stat(filepath.Join(dir, mark)); err == nil {
				return dir
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
