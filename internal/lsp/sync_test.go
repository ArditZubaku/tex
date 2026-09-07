package lsp

import (
	"bufio"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ArditZubaku/tex/internal/buffer"
)

// project is a directory a server would be started in: what marks a project of
// that kind above the file, without which nothing is tracked at all.
func project(t *testing.T, name, content string) string {
	t.Helper()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	return path
}

func loose(t *testing.T, name, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	return path
}

func opening(t *testing.T, path string) *buffer.Buffer {
	t.Helper()

	b := buffer.Open(path)
	t.Cleanup(b.Close)

	return b
}

// reconciling is the package driven the way the editor's own loop drives it,
// with a pair of pipes where the server would be.
func reconciling(t *testing.T) *harness {
	t.Helper()

	Reset()
	t.Cleanup(Reset)

	near, far := net.Pipe()
	if err := far.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = far.Close() })

	h := &harness{
		t:      t,
		server: &server{t: t, conn: far, wire: bufio.NewReader(far)},
		woken:  make(chan struct{}, 1),
	}
	dialFor = func(string) Dial {
		return func(string) (Transport, error) { return pipe{near}, nil }
	}
	Wake(h.wake)

	return h
}

// up is the first pass, which finds the server rather than telling it anything,
// plus the handshake it takes before it will be told.
func (h *harness) up(files ...File) {
	h.t.Helper()

	Sync(files)
	if client == nil {
		h.t.Fatal("nothing started a server")
	}

	h.client = client
	h.shake("")
	Sync(files)
}

func (h *harness) sent() Message {
	h.t.Helper()

	return h.server.next()
}

func openedIn(t *testing.T, msg Message) textDocumentItem {
	t.Helper()

	if msg.Method != methodDidOpen {
		t.Fatalf("the server was sent %q, want %q", msg.Method, methodDidOpen)
	}

	var params didOpenParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		t.Fatal(err)
	}

	return params.TextDocument
}

func TestOpeningABufferTellsTheServerTheWholeOfIt(t *testing.T) {
	h := reconciling(t)
	path := project(t, "main.go", "package main\n\nfunc main() {}\n")
	b := opening(t, path)

	h.up(File{Path: path, Buf: b})

	item := openedIn(t, h.sent())
	if item.URI != FileURI(path) {
		t.Errorf("opened %q, want %q", item.URI, FileURI(path))
	}
	if item.LanguageID != "go" || item.Version != 1 {
		t.Errorf("opened as %q version %d, want go version 1", item.LanguageID, item.Version)
	}
	if item.Text != "package main\n\nfunc main() {}\n" {
		t.Errorf("opened with %q, want the file as it is on disk", item.Text)
	}
}

func TestATrackedFileWithNoProjectAboveItIsLeftAlone(t *testing.T) {
	reconciling(t)
	path := loose(t, "main.go", "package main\n")

	Sync([]File{{Path: path, Buf: opening(t, path)}})

	if client != nil {
		t.Fatal("a server was started for a file in no project")
	}
}

func TestAFileNoServerAnswersForIsLeftAlone(t *testing.T) {
	reconciling(t)
	path := project(t, "notes.txt", "nothing to see\n")

	Sync([]File{{Path: path, Buf: opening(t, path)}})

	if client != nil {
		t.Fatal("a server was started for a file it knows nothing about")
	}
}

func TestNothingHappensUntilTheLoopSaysWhereToWakeIt(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	dialFor = func(string) Dial {
		return func(string) (Transport, error) {
			t.Fatal("a server was started with no loop to wake")

			return nil, nil
		}
	}

	path := project(t, "main.go", "package main\n")
	Sync([]File{{Path: path, Buf: opening(t, path)}})
}

func TestAChangeGoesOutOnlyOnceTheThrottleHasPassed(t *testing.T) {
	pass := held(t)
	h := reconciling(t)
	path := project(t, "main.go", "package main\n")
	b := opening(t, path)

	files := []File{{Path: path, Buf: b}}
	h.up(files...)
	openedIn(t, h.sent())

	b.SetLine(0, []rune("package other"))
	Sync(files)

	pass(throttle)
	Sync(files)

	msg := h.sent()
	if msg.Method != methodDidChange {
		t.Fatalf("the server was sent %q, want %q", msg.Method, methodDidChange)
	}

	var params didChangeParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		t.Fatal(err)
	}
	if params.TextDocument.Version != 2 {
		t.Errorf("changed to version %d, want 2", params.TextDocument.Version)
	}
	if len(params.ContentChanges) != 1 || params.ContentChanges[0].Text != "package other\n" {
		t.Errorf("changed to %+v, want the whole text", params.ContentChanges)
	}
}

// A save points the buffer at the file it just wrote and a reread replaces it
// outright, and a version that started again from nothing would read as a copy
// already current — leaving the server's idea of the file stale for as long as
// it is open.
func TestTheVersionOnlyEverGoesUpAcrossASaveAndAReread(t *testing.T) {
	pass := held(t)
	h := reconciling(t)
	path := project(t, "main.go", "package main\n")
	b := opening(t, path)

	files := []File{{Path: path, Buf: b}}
	h.up(files...)
	openedIn(t, h.sent())

	was := 1
	for _, change := range []func(){
		func() { b.SetLine(0, []rune("package one")) },
		func() { _ = b.Save(path) },
		func() { b.Reload(path) },
		func() { b.SetLine(0, []rune("package two")) },
	} {
		change()
		pass(throttle)
		Sync(files)

		msg := h.sent()
		var params didChangeParams
		if err := json.Unmarshal(msg.Params, &params); err != nil {
			t.Fatal(err)
		}
		if params.TextDocument.Version <= was {
			t.Fatalf("version went from %d to %d", was, params.TextDocument.Version)
		}
		was = params.TextDocument.Version
	}
}

func TestASaveIsToldOnlyOnceTheTextItBelongsToHasGone(t *testing.T) {
	pass := held(t)
	h := reconciling(t)
	path := project(t, "main.go", "package main\n")
	b := opening(t, path)

	h.up(File{Path: path, Buf: b, Modified: true})
	openedIn(t, h.sent())

	b.SetLine(0, []rune("package saved"))
	if err := b.Save(path); err != nil {
		t.Fatal(err)
	}

	pass(throttle)
	Sync([]File{{Path: path, Buf: b, Modified: false}})

	if got := h.sent().Method; got != methodDidChange {
		t.Fatalf("the server was sent %q first, want %q", got, methodDidChange)
	}
	if got := h.sent().Method; got != methodDidSave {
		t.Fatalf("the server was then sent %q, want %q", got, methodDidSave)
	}
}

func TestABufferTheEditorNoLongerHoldsIsClosed(t *testing.T) {
	h := reconciling(t)
	path := project(t, "main.go", "package main\n")
	b := opening(t, path)

	h.up(File{Path: path, Buf: b})
	openedIn(t, h.sent())

	Sync(nil)

	msg := h.sent()
	if msg.Method != methodDidClose {
		t.Fatalf("the server was sent %q, want %q", msg.Method, methodDidClose)
	}

	var params identParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		t.Fatal(err)
	}
	if params.TextDocument.URI != FileURI(path) {
		t.Errorf("closed %q, want %q", params.TextDocument.URI, FileURI(path))
	}
	if Ready(path) {
		t.Error("a file the editor had closed was still worth asking about")
	}
}

func TestADocumentTooLargeToSendIsNotRenderedAgain(t *testing.T) {
	h := reconciling(t)
	path := project(t, "main.go", "package main\n"+strings.Repeat("// x\n", maxDocument/5))
	b := opening(t, path)

	files := []File{{Path: path, Buf: b}}
	h.up(files...)

	if !refused[path] {
		t.Fatal("a file too large to send was not refused")
	}
	if Ready(path) {
		t.Error("a file that was never sent was still worth asking about")
	}

	Sync(files)
	if len(docs) != 0 {
		t.Errorf("the server was told about %d documents, want none", len(docs))
	}
}

func TestOnlyAServerThatHoldsTheFileIsWorthAsking(t *testing.T) {
	h := reconciling(t)
	path := project(t, "main.go", "package main\n")
	b := opening(t, path)

	if Ready(path) {
		t.Fatal("a file was worth asking about before a server was running")
	}

	h.up(File{Path: path, Buf: b})
	openedIn(t, h.sent())

	if !Ready(path) {
		t.Error("a file the server holds was not worth asking about")
	}
	if Ready(filepath.Join(filepath.Dir(path), "other.go")) {
		t.Error("a file the server was never told about was worth asking about")
	}
}
