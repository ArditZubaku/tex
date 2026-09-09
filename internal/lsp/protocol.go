package lsp

import (
	"net/url"
	"path/filepath"
	"strings"
)

// The methods the editor uses. They are named rather than spelled out at each
// call so that a typo is a build error instead of a request no server answers.
const (
	methodInitialize           = "initialize"
	methodInitialized          = "initialized"
	methodShutdown             = "shutdown"
	methodExit                 = "exit"
	methodRegisterCapability   = "client/registerCapability"
	methodUnregisterCapability = "client/unregisterCapability"
	methodDidOpen              = "textDocument/didOpen"
	methodDidChange            = "textDocument/didChange"
	methodDidSave              = "textDocument/didSave"
	methodDidClose             = "textDocument/didClose"
	methodPublishDiagnostics   = "textDocument/publishDiagnostics"
)

// A path is turned into a URI on every frame's reconciling, once per open
// buffer, and again on every key a completion is weighed against. Escaping one
// allocates, so the answers are kept beside the resolved paths they are built
// from.
var fileURIs = map[string]string{}

// FileURI is a path as a server names it. Symlinks are resolved because a
// server does resolve them — on macOS /tmp is /private/tmp and a test's own
// temporary directory is under /private/var — and a URI that does not match
// leaves every answer attached to a document the editor does not have open.
func FileURI(path string) string {
	if uri, ok := fileURIs[path]; ok {
		return uri
	}

	uri := "file://" + (&url.URL{Path: resolved(path)}).EscapedPath()
	fileURIs[path] = uri

	return uri
}

// Path is FileURI back, for the URIs a server sends.
func Path(uri string) string {
	trimmed := strings.TrimPrefix(uri, "file://")
	if trimmed == uri {
		return uri // not a file URI at all: a server's own scheme, left alone
	}

	unescaped, err := url.PathUnescape(trimmed)
	if err != nil {
		return trimmed
	}

	return unescaped
}

// Resolving a path costs a stat per directory of it, and the same handful of
// paths are asked for on every frame, so the answers are kept.
var resolvedPaths = map[string]string{}

func resolved(path string) string {
	if was, ok := resolvedPaths[path]; ok {
		return was
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	// A file being created has no symlinks to resolve yet, and the directory
	// holding it is where the resolving that matters happens anyway.
	dir, name := filepath.Split(abs)
	if resolvedDir, err := filepath.EvalSymlinks(filepath.Clean(dir)); err == nil {
		abs = filepath.Join(resolvedDir, name)
	}
	resolvedPaths[path] = abs

	return abs
}
