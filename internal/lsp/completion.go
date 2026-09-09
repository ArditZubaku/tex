package lsp

import (
	"cmp"
	"encoding/json"
	"slices"

	"github.com/ArditZubaku/tex/internal/buffer"
)

const (
	methodCompletion = "textDocument/completion"
	methodResolve    = "completionItem/resolve"
)

// A Trigger is why a completion was asked for — the protocol's own
// CompletionTriggerKind, which a server reads rather than ignores: gopls offers
// the whole scope for one asked for outright, and only what can follow the
// character for one a character triggered.
type Trigger int

const (
	Invoked     Trigger = 1
	TriggerChar Trigger = 2
	Refining    Trigger = 3
)

// The protocol's InsertTextFormat for a snippet, which is a template with
// placeholders in it rather than text. The handshake says none are supported,
// so this is only ever a server that did not listen.
const snippet = 2

// A TextEdit is a stretch of a file replaced by some text, in the server's own
// coordinates. Completion sends them for the import a name needs, which is why
// they are carried through rather than dropped: a symbol taken out of a package
// the file does not import yet is a compile error the moment it lands.
type TextEdit struct {
	Range   Range  `json:"range"`
	NewText string `json:"newText"`
}

// An Item is one candidate. Text is what goes into the file and Edit what it
// replaces — nil where the server named no range at all, which leaves what the
// typing so far covered to the caller, who can see the line.
//
// Data is the server's own bookmark for the candidate, meaningless here and
// handed straight back to it: a server that held the import line back until the
// candidate was settled on has no other way of knowing which one it was.
type Item struct {
	Label  string
	Detail string
	Kind   int
	Text   string
	Edit   *Range
	Extra  []TextEdit
	Data   json.RawMessage
}

// A Completion is what a server offered. Incomplete is it saying the list was
// cut to the prefix it was asked about rather than being all there is, and so
// that a longer prefix is worth asking about again.
type Completion struct {
	Items      []Item
	Incomplete bool
}

type completionContext struct {
	TriggerKind      Trigger `json:"triggerKind"`
	TriggerCharacter string  `json:"triggerCharacter,omitempty"`
}

type completionParams struct {
	TextDocument ident             `json:"textDocument"`
	Position     Position          `json:"position"`
	Context      completionContext `json:"context"`
}

type completionList struct {
	IsIncomplete bool             `json:"isIncomplete"`
	Items        []completionItem `json:"items"`
}

// Everything but the label is omitted when it is empty, because this is the
// shape a resolve request goes out in as well as the shape an answer comes back
// in, and a candidate quoted back with "kind": 0 is a candidate naming a kind
// the protocol does not have.
type completionItem struct {
	Label               string          `json:"label"`
	Detail              string          `json:"detail,omitempty"`
	Kind                int             `json:"kind,omitempty"`
	SortText            string          `json:"sortText,omitempty"`
	InsertText          string          `json:"insertText,omitempty"`
	InsertTextFormat    int             `json:"insertTextFormat,omitempty"`
	TextEdit            *TextEdit       `json:"textEdit,omitempty"`
	AdditionalTextEdits []TextEdit      `json:"additionalTextEdits,omitempty"`
	Data                json.RawMessage `json:"data,omitempty"`
}

// Complete is what the server offers for the word at row and col. Unlike every
// other lookup it sends the document first: the reconciler holds a change back
// for as long as a fifth of a second, and a server answering out of the text as
// it was two keystrokes ago offers what the word used to be the start of.
func Complete(
	path string, b *buffer.Buffer, row, col int,
	trigger Trigger, ch string, answer func(Completion, error),
) {
	doc := held(path)
	if !Completing(path) || !freshen(doc, b) {
		answer(Completion{}, ErrStopped)

		return
	}

	params := completionParams{
		TextDocument: ident{URI: doc.uri},
		Position:     doc.srv.client.Encoding().Pos(b, row, col),
		Context:      completionContext{TriggerKind: trigger, TriggerCharacter: ch},
	}
	err := doc.srv.client.Request(methodCompletion, params, func(result json.RawMessage, err error) {
		if err != nil {
			answer(Completion{}, err)

			return
		}
		answer(completionFrom(result))
	})
	if err != nil {
		answer(Completion{}, err)
	}
}

// The two shapes an answer comes in are a list that says whether it was cut and
// a bare array that cannot, and which of them arrives is the server's choice
// rather than the request's.
func completionFrom(result json.RawMessage) (Completion, error) {
	if empty(result) {
		return Completion{}, nil
	}

	var list completionList
	if err := json.Unmarshal(result, &list); err == nil {
		return Completion{Items: itemsFrom(list.Items), Incomplete: list.IsIncomplete}, nil
	}

	var many []completionItem
	if err := json.Unmarshal(result, &many); err != nil {
		return Completion{}, err
	}

	return Completion{Items: itemsFrom(many)}, nil
}

// The order is the server's own ranking, which is what sortText is for: it is
// how a server says that the field of the receiver comes before the package of
// the same first letter, and the answer is not in that order on the wire.
func itemsFrom(raw []completionItem) []Item {
	slices.SortStableFunc(raw, func(a, b completionItem) int {
		return cmp.Compare(a.order(), b.order())
	})

	out := make([]Item, 0, len(raw))
	for _, one := range raw {
		out = append(out, one.item())
	}

	return out
}

func (c completionItem) order() string {
	if c.SortText != "" {
		return c.SortText
	}

	return c.Label
}

func (c completionItem) item() Item {
	got := Item{
		Label:  c.Label,
		Detail: c.Detail,
		Kind:   c.Kind,
		Text:   c.plain(),
		Extra:  c.AdditionalTextEdits,
		Data:   c.Data,
	}
	if c.TextEdit != nil {
		at := c.TextEdit.Range
		got.Edit = &at
	}

	return got
}

// A server that sends a snippet after being told none are supported is sending
// placeholders there is nothing here to expand, so the label goes in instead:
// it is the plain name in every server that does this.
func (c completionItem) plain() string {
	switch {
	case c.InsertTextFormat == snippet:
		return c.Label
	case c.TextEdit != nil && c.TextEdit.NewText != "":
		return c.TextEdit.NewText
	case c.InsertText != "":
		return c.InsertText
	default:
		return c.Label
	}
}

// Resolves says whether the server holding this file answers a second question
// about one candidate, which is what a candidate with no edits of its own and a
// bookmark to quote back is waiting on.
func Resolves(path string) bool {
	doc := held(path)

	return doc != nil && doc.srv.client.Ready() && doc.srv.client.Resolves()
}

// Resolve is the rest of one candidate: the import line a server held back
// until it knew which candidate was being settled on. It is asked after the
// candidate has already gone in rather than before, since a round trip with the
// keyboard held is a round trip felt.
func Resolve(path string, item Item, answer func([]TextEdit, error)) {
	doc := held(path)
	if doc == nil || !doc.srv.client.Ready() || !doc.srv.client.Resolves() {
		answer(nil, ErrStopped)

		return
	}

	// The protocol has the candidate itself go back, and a server reads its own
	// bookmark out of it. Nothing else here is worth the memory of holding a
	// thousand candidates' raw JSON on the chance one is settled on.
	asking := completionItem{Label: item.Label, Kind: item.Kind, Data: item.Data}
	err := doc.srv.client.Request(methodResolve, asking, func(result json.RawMessage, err error) {
		if err != nil {
			answer(nil, err)

			return
		}
		answer(resolvedFrom(result))
	})
	if err != nil {
		answer(nil, err)
	}
}

func resolvedFrom(result json.RawMessage) ([]TextEdit, error) {
	if empty(result) {
		return nil, nil
	}

	var got completionItem
	if err := json.Unmarshal(result, &got); err != nil {
		return nil, err
	}

	return got.AdditionalTextEdits, nil
}
