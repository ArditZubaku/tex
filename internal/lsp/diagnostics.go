package lsp

import "encoding/json"

// A Diagnostic is one thing a server said about a file, in the protocol's own
// coordinates. Turning those into rows and rune columns needs the text they
// were counted in, and nothing here may read a buffer, so it is left to the
// caller.
type Diagnostic struct {
	Range    Range  `json:"range"`
	Severity int    `json:"severity"`
	Source   string `json:"source"`
	Message  string `json:"message"`
}

type publishDiagnosticsParams struct {
	URI         string       `json:"uri"`
	Version     int          `json:"version"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// Published is where a server's diagnostics go, and is nil until something
// wants them. It runs on the loop's goroutine, out of Poll, and an empty list
// is honoured rather than dropped: it is the only way an underline ever comes
// off a line that has been fixed.
var Published func(path string, notes []Diagnostic)

func published(params json.RawMessage) {
	var got publishDiagnosticsParams
	if err := json.Unmarshal(params, &got); err != nil {
		return
	}

	// A payload that crossed a newer document on the wire is about text that is
	// already gone, and drawing it would put every underline a line out. A
	// version of zero is a server that did not say, since ours start at one.
	if doc, open := docs[got.URI]; open && got.Version != 0 && got.Version < doc.version {
		return
	}

	Published(Path(got.URI), got.Diagnostics)
}

func handle(method string, params json.RawMessage) {
	if method == methodPublishDiagnostics && Published != nil {
		published(params)
	}
}
