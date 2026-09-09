package lsp

import "time"

// A document is what one file was last said to be, and which server it was said
// to. The version is that server's own count, which must only ever go up; the
// revision is the buffer's, and the two differing is the whole of "this needs
// sending".
type document struct {
	uri      string
	srv      *server
	version  int
	revision int
	modified bool
	sentAt   time.Time
}

// close is the server told to forget the file and the editor forgetting it was
// ever told, which has to happen together or the next open would be refused as
// one already open.
func (d *document) close() {
	delete(docs, d.uri)
	d.srv.client.Notify(methodDidClose, identParams{TextDocument: ident{URI: d.uri}})
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
