package find

import (
	"strconv"
	"time"

	"github.com/nsf/termbox-go"

	"github.com/ArditZubaku/tex/internal/chars"
	"github.com/ArditZubaku/tex/internal/editor/complete"
	"github.com/ArditZubaku/tex/internal/editor/edit"
	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/ArditZubaku/tex/internal/lsp"
	"github.com/ArditZubaku/tex/internal/ngram"
)

// Completion is what a language server offers for the word being typed, in a
// menu under it. Where no server is running, the buffer's own words stand in
// for one — internal/ngram's ranking of what has usually followed the word
// before — since the editor's own answer to "what could this be" is otherwise
// 'gd' and the symbol listings, which read declarations rather than guess at
// half a word.
//
// The local fallback only answers when asked outright, never as a side effect
// of typing: it has no trigger character and no minimum word length to hold it
// back, so leaving it to fire on every keystroke would mean the first Escape
// after almost any word closed a menu nobody asked for instead of leaving Edit
// mode — every plain-text file paying a language server's asking price with
// none of a language server's restraint about when to ask.

const (
	// A menu is wanted from the first letter on rather than held back until a
	// word is unambiguous: the narrowing past this point happens locally (see
	// AfterKey and refine), so there is nothing saved by waiting for more of
	// the word before asking.
	minPrefix = 1

	// A ceiling rather than a shortlist. The narrowing happens locally, over
	// everything the server sent, and it has to: typescript-language-server
	// answers a bare prefix with a thousand candidates and ranks the ones that
	// need an import *last*, so a list cut to the best few hundred is a list
	// with every auto-import cut out of it.
	maxItems = 2000

	// A signature past this is cut long before it is drawn, so the rest of it
	// is memory held for nothing across every candidate of every word.
	maxDetail = 96

	// Deliberately the reconciler's own throttle: a completion sends the
	// document before it asks, so asking more often than the editor already
	// tells the server about typing would be paying for the same send twice.
	refresh = 200 * time.Millisecond

	// The local model is already ranked by relevance rather than dumped in
	// server order, so there is no thousand-candidate flood to cut down here —
	// just enough to fill the menu's own maxRows several times over.
	maxLocalItems = 50
)

// asking is the word the newest request was made about. An answer naming
// anything else belongs to a word already typed past, and putting a menu up
// from it would offer the last word's candidates against this one. Its token
// comes from the same counter every other lookup uses, so that a completion
// asked for in Edit mode drops the hover whose mode has been left.
var asking wordAsked

type wordAsked struct {
	token int
	path  string
	row   int
	start int
	at    time.Time

	// hushed is a word the server had nothing for, so that the next keystroke
	// does not ask about the same word again to be told the same thing.
	hushed bool
}

var now = time.Now

// resetCompletion is one test not being answered out of another one's requests.
func resetCompletion() { asking = wordAsked{} }

// Suggest is the candidates for the word under the cursor, asked for outright
// rather than waited for: Ctrl-Space, and Ctrl-N with no menu up. Stepping
// through a menu already showing is the key handler's own, the way VIM's Ctrl-N
// is once the list is there.
func Suggest(e *state.Editor) {
	if !lsp.Completing(e.SourceFile) {
		suggestLocal(e)

		return
	}

	request(e, lsp.Invoked, "")
}

// CompletionKey is the keys the menu owns while it is up, and Ctrl-Space,
// Ctrl-N and Ctrl-P when it is not. Everything else goes on to be typed, which
// is what the result says.
func CompletionKey(e *state.Editor, event termbox.Event) bool {
	if e.Mode != state.EditMode || event.Ch != 0 {
		return false
	}

	// Ctrl-Space asks outright whether or not a menu is up, which is what makes
	// it the key for a place no typing has opened one: inside a literal's
	// braces, in front of a closing bracket. The zero Key is the key itself
	// rather than an event with none — a rune arrives with its rune, and was
	// turned away above.
	if event.Key == termbox.KeyCtrlSpace {
		Suggest(e)

		return true
	}

	if !e.Comp.Open() {
		if event.Key == termbox.KeyCtrlN || event.Key == termbox.KeyCtrlP {
			Suggest(e)

			return true
		}

		return false
	}

	switch event.Key {
	case termbox.KeyCtrlN, termbox.KeyArrowDown:
		e.Comp.Next()
	case termbox.KeyCtrlP, termbox.KeyArrowUp:
		e.Comp.Prev()
	// Ctrl-J is not among these, and that is what keeps a paste from settling
	// on candidates: with no bracketed paste to tell typing apart, a pasted
	// Unix line ending is the only thing that arrives as one, while the Enter
	// key itself arrives as the CR below.
	case termbox.KeyEnter, termbox.KeyTab, termbox.KeyCtrlY:
		accept(e)
	case termbox.KeyCtrlE, termbox.KeyEsc:
		e.Comp.Close()
	default:
		return false
	}

	return true
}

// AfterKey is the menu kept in step with the text, run once the key has done
// whatever it does. A menu narrows to the word as it now stands, and a word
// that has grown long enough to be worth asking about is asked about.
//
// typing is whether the key arrived in Edit mode rather than the one that
// entered it: 'a' pressed at the end of a name is not somebody asking what the
// name could be, and a request for it would be a menu nobody opened.
func AfterKey(e *state.Editor, event termbox.Event, typing bool) {
	if e.Mode != state.EditMode {
		e.Comp.Close()

		return
	}
	if !typing {
		return
	}

	if e.Comp.Open() {
		refine(e)

		return
	}
	if !lsp.Completing(e.SourceFile) {
		return
	}

	start, ok := wordStart(e)
	if !ok {
		return
	}

	switch {
	case lsp.TriggerRune(e.SourceFile, event.Ch):
		request(e, lsp.TriggerChar, string(event.Ch))
	case e.Col-start >= minPrefix && worthAsking(e, start):
		request(e, lsp.Invoked, "")
	}
}

// suggestLocal is Suggest's own answer for a file with no language server:
// the buffer's own words, ranked by what has usually followed the word before
// the one being typed. There is no request in flight to await, so the menu
// goes up in the same call rather than a callback away.
func suggestLocal(e *state.Editor) {
	start, ok := wordStart(e)
	if !ok {
		return
	}

	prefix := string(e.Buf.Line(e.Row)[start:e.Col])
	words := ngram.Build(e.Buf).Rank(prevWord(e, start), prefix, maxLocalItems)
	if len(words) == 0 {
		return
	}

	items := make([]complete.Item, len(words))
	for i, word := range words {
		items[i] = complete.Item{Label: word, Text: word, From: start}
	}

	e.Comp.Show(items, e.Row, start, []rune(prefix), false)
}

// prevWord is the word immediately before start on row, which is the bigram
// context Rank narrows by. A start with nothing word-shaped in front of it on
// its own line looks up to the line before, the same way Build's own bigrams
// carry a word across the break rather than stopping at it.
func prevWord(e *state.Editor, start int) string {
	line := e.Buf.Line(e.Row)[:start]

	for row := e.Row; ; {
		end := len(line)
		for end > 0 && !chars.IsWord(line[end-1]) {
			end--
		}

		if end > 0 {
			begin := end
			for begin > 0 && chars.IsWord(line[begin-1]) {
				begin--
			}

			return string(line[begin:end])
		}

		row--
		if row < 0 {
			return ""
		}
		line = e.Buf.Line(row)
	}
}

// worthAsking keeps one word from being asked about over and over: a server
// that answered it with nothing has nothing more to give, and while a request
// for it is still out a second one only doubles what the first costs.
func worthAsking(e *state.Editor, start int) bool {
	if e.SourceFile != asking.path || e.Row != asking.row || start != asking.start {
		return true
	}

	return !asking.hushed && now().Sub(asking.at) >= refresh
}

// A menu follows the word it was opened on and no further: the cursor leaving
// it — onto another row, or back behind where it started — closes the menu
// rather than filtering against a word it was never about.
func refine(e *state.Editor) {
	start, ok := wordStart(e)
	if !ok || e.Row != e.Comp.Row() || start != e.Comp.Start() {
		e.Comp.Close()

		return
	}

	typed := e.Buf.Line(e.Row)[start:e.Col]
	if e.Comp.Retype(typed) && e.Comp.Incomplete() && now().Sub(asking.at) >= refresh {
		request(e, lsp.Refining, "")
	}
}

// A menu already up is left up while the answer is awaited, so that a refresh
// does not blink the candidates off screen and back.
func request(e *state.Editor, trigger lsp.Trigger, ch string) {
	start, ok := wordStart(e)
	if !ok {
		return
	}

	asked++
	asking.token, asking.path, asking.row, asking.start = asked, e.SourceFile, e.Row, start
	asking.at, asking.hushed = now(), false

	token := asked
	lsp.Complete(e.SourceFile, e.Buf, e.Row, e.Col, trigger, ch,
		func(got lsp.Completion, err error) {
			if token == asking.token {
				offer(e, got, err)
			}
		})
}

func offer(e *state.Editor, got lsp.Completion, err error) {
	if err != nil || !stillTyping(e) {
		e.Comp.Close()

		return
	}
	if len(got.Items) == 0 {
		asking.hushed = true
		e.Comp.Close()

		return
	}

	items := candidates(e, got.Items[:min(len(got.Items), maxItems)], asking.start)
	typed := e.Buf.Line(e.Row)[asking.start:e.Col]
	if !e.Comp.Show(items, asking.row, asking.start, typed, got.Incomplete) {
		asking.hushed = true
	}
}

// An answer is still wanted while the cursor is where it was left or further
// along the same word: typing does not stop for a request, and everything the
// server sent is still about the word being extended.
func stillTyping(e *state.Editor) bool {
	return e.Mode == state.EditMode &&
		e.SourceFile == asking.path && e.Row == asking.row &&
		e.Col >= asking.start && e.Col <= e.Buf.RuneLen(e.Row) &&
		wordStartAt(e.Buf.Line(e.Row), e.Col) == asking.start
}

// candidates are the server's items as the menu's own rows. What each replaces
// is the server's to say — it knows that a Rust '::' is part of the path and a
// Go '.' is not — and only where it says nothing does the word under the cursor
// answer for it.
func candidates(e *state.Editor, found []lsp.Item, start int) []complete.Item {
	encoding := lsp.PositionEncoding(e.SourceFile)
	resolves := lsp.Resolves(e.SourceFile)
	items := make([]complete.Item, 0, len(found))

	for _, one := range found {
		from := start
		if one.Edit != nil && one.Edit.Start.Line == e.Row {
			from = min(encoding.Column(e.Buf.Line(e.Row), one.Edit.Start.Character), start)
		}

		items = append(items, complete.Item{
			Label:  one.Label,
			Detail: detailOf(one),
			Text:   one.Text,
			From:   from,
			Call:   called(one.Kind),
			Extra:  edits(e, encoding, one.Extra),
			Ask:    len(one.Extra) == 0 && one.Data != nil && resolves,
			Data:   one.Data,
		})
	}

	return items
}

// A candidate with nothing to say for itself is labelled with what kind of
// thing it is, which is the only thing left that tells a keyword from a local.
func detailOf(one lsp.Item) string {
	if one.Detail == "" {
		return completionKind(one.Kind)
	}

	if runes := []rune(one.Detail); len(runes) > maxDetail {
		return string(runes[:maxDetail])
	}

	return one.Detail
}

func edits(e *state.Editor, encoding lsp.Encoding, extra []lsp.TextEdit) []complete.Edit {
	if len(extra) == 0 {
		return nil
	}

	out := make([]complete.Edit, 0, len(extra))
	for _, one := range extra {
		row, col := encoding.RowCol(e.Buf, one.Range.Start)
		endRow, endCol := encoding.RowCol(e.Buf, one.Range.End)
		out = append(out, complete.Edit{
			Row: row, Col: col, EndRow: endRow, EndCol: endCol, Text: one.NewText,
		})
	}

	return out
}

func accept(e *state.Editor) {
	item, ok := e.Comp.Selected()
	e.Comp.Close()
	if !ok {
		return
	}

	edit.Accept(e, item)
	if item.Ask {
		resolve(e, item)
	}
}

// resolve is the second question about a candidate already settled on: what
// else has to change for it, which a server may hold back until it knows which
// one it was. The name is already in — a round trip with the keyboard held is a
// round trip felt — so the import lands a frame or two behind it.
func resolve(e *state.Editor, item complete.Item) {
	asked++
	token, path, lines := asked, e.SourceFile, e.Buf.LineCount()

	lsp.Resolve(path, lsp.Item{Label: item.Label, Data: item.Data},
		func(extra []lsp.TextEdit, err error) {
			// Anything that has since added or dropped a line has moved the
			// rows the server named, and an import written into the wrong one
			// is worse than no import at all.
			if err != nil || token != asked ||
				e.SourceFile != path || e.Buf.LineCount() != lines {
				return
			}
			edit.Elsewhere(e, edits(e, lsp.PositionEncoding(path), extra))
		})
}

// wordStart is where the word the cursor is at the end of begins. There may be
// none of it typed yet — a '.' just typed is a whole scope's worth of
// candidates and no word at all — so an empty one is an answer. What is not is
// a cursor in the middle of a word: what went in would land in front of the
// rest of it.
func wordStart(e *state.Editor) (int, bool) {
	line := e.Buf.Line(e.Row)
	if e.Col > len(line) {
		return 0, false
	}
	if e.Col < len(line) && chars.IsWord(line[e.Col]) {
		return 0, false
	}

	return wordStartAt(line, e.Col), true
}

func wordStartAt(line []rune, col int) int {
	start := min(col, len(line))
	for start > 0 && chars.IsWord(line[start-1]) {
		start--
	}

	return start
}

// The protocol's own CompletionItemKind, which is numbered differently from the
// SymbolKind the listings use and so cannot share its table.
var completionKindNames = [...]string{
	1: "Text", 2: "Method", 3: "Function", 4: "New", 5: "Field",
	6: "Variable", 7: "Class", 8: "Interface", 9: "Module", 10: "Property",
	11: "Unit", 12: "Value", 13: "Enum", 14: "Keyword", 15: "Snippet",
	16: "Color", 17: "File", 18: "Reference", 19: "Folder", 20: "EnumValue",
	21: "Constant", 22: "Struct", 23: "Event", 24: "Operator", 25: "TypeParam",
}

// The three kinds that are invoked rather than named, which is what a
// candidate carrying its own parentheses in is.
const (
	kindMethod      = 2
	kindFunction    = 3
	kindConstructor = 4
)

func called(kind int) bool {
	return kind == kindMethod || kind == kindFunction || kind == kindConstructor
}

func completionKind(kind int) string {
	if kind < 1 || kind >= len(completionKindNames) {
		return strconv.Itoa(kind)
	}

	return completionKindNames[kind]
}
