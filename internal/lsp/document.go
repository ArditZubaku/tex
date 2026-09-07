package lsp

import "time"

// A document is what one file was last said to be. The version is the
// server's own count, which must only ever go up; the revision is the buffer's,
// and the two differing is the whole of "this needs sending".
type document struct {
	uri      string
	version  int
	revision int
	modified bool
	sentAt   time.Time
}

type textDocumentItem struct {
	URI        string `json:"uri"`
	LanguageID string `json:"languageId"`
	Version    int    `json:"version"`
	Text       string `json:"text"`
}

type didOpenParams struct {
	TextDocument textDocumentItem `json:"textDocument"`
}

type versionedIdent struct {
	URI     string `json:"uri"`
	Version int    `json:"version"`
}

// Only the whole text is ever sent, which is what makes the reconciler possible
// at all: an incremental change has to be told what the last one was, and there
// is no such thing here to be told.
type contentChange struct {
	Text string `json:"text"`
}

type didChangeParams struct {
	TextDocument   versionedIdent  `json:"textDocument"`
	ContentChanges []contentChange `json:"contentChanges"`
}

type ident struct {
	URI string `json:"uri"`
}

type identParams struct {
	TextDocument ident `json:"textDocument"`
}

func (d *document) opened(languageID, body string) didOpenParams {
	return didOpenParams{TextDocument: textDocumentItem{
		URI:        d.uri,
		LanguageID: languageID,
		Version:    d.version,
		Text:       body,
	}}
}

func (d *document) changed(body string) didChangeParams {
	return didChangeParams{
		TextDocument:   versionedIdent{URI: d.uri, Version: d.version},
		ContentChanges: []contentChange{{Text: body}},
	}
}
