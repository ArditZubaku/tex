package find

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/nsf/termbox-go"

	"github.com/ArditZubaku/tex/internal/editor/complete"
	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/ArditZubaku/tex/internal/lsp"
)

// These are the answers a server gives, acted on without one: what crosses the
// wire is the language server package's own business, and what is left here is
// deciding whether an answer still belongs to the word being typed.

func typing(t *testing.T, content string, col int) (*state.Editor, string) {
	t.Helper()

	e, path := answered(t, "main.go", content)
	e.Mode, e.Col = state.EditMode, col
	asking = wordAsked{path: path, at: now()}

	return e, path
}

func candidate(name string) lsp.Item { return lsp.Item{Label: name, Text: name} }

func TestAWordIsCompletedFromWhereItStarts(t *testing.T) {
	e, _ := typing(t, "\tfmt.Prin\n", 9)

	start, ok := wordStart(e)
	if !ok || start != 5 {
		t.Errorf("the word starts at %d (%v), want 5", start, ok)
	}
}

// A '.' just typed is a whole scope's worth of candidates and no word at all,
// which is exactly when a menu is most wanted.
func TestACursorOnNoWordAtAllStillCompletes(t *testing.T) {
	e, _ := typing(t, "\tfmt.\n", 5)

	start, ok := wordStart(e)
	if !ok || start != 5 {
		t.Errorf("the word starts at %d (%v), want 5", start, ok)
	}
}

// What went in would land in front of the rest of the word.
func TestACursorInTheMiddleOfAWordCompletesNothing(t *testing.T) {
	e, _ := typing(t, "\tPrintln\n", 4)

	if _, ok := wordStart(e); ok {
		t.Error("a cursor inside a word was taken for one completing it")
	}
}

func TestAnAnswerForAWordTheTypingHasLeftIsDropped(t *testing.T) {
	e, _ := typing(t, "\tfmt.Prin\n", 9)
	asking.start = 5
	e.Row, e.Col = 0, 0

	offer(e, lsp.Completion{Items: []lsp.Item{candidate("Println")}}, nil)

	if e.Comp.Open() {
		t.Error("a menu went up for a word the cursor had already left")
	}
}

func TestAnAnswerForAWordStillBeingTypedIsNarrowedToIt(t *testing.T) {
	e, _ := typing(t, "\tfmt.Prin\n", 9)
	asking.start = 5

	offer(e, lsp.Completion{Items: []lsp.Item{
		candidate("Println"), candidate("Sscan"),
	}}, nil)

	if !e.Comp.Open() {
		t.Fatal("no menu went up for the word being typed")
	}
	if e.Comp.Count() != 1 {
		t.Errorf("the menu offers %d candidates, want only the one the typing matches", e.Comp.Count())
	}
}

// Asking again would be asking the same question of the same text.
func TestAWordTheServerHadNothingForIsNotAskedAboutAgain(t *testing.T) {
	e, _ := typing(t, "\tfmt.Prin\n", 9)
	asking.start = 5

	offer(e, lsp.Completion{}, nil)

	if !asking.hushed {
		t.Fatal("a word answered with nothing is still worth asking about")
	}
	if worthAsking(e, 5) {
		t.Error("the same word is being asked about again")
	}
	if !worthAsking(e, 4) {
		t.Error("a longer word than the one answered is not being asked about")
	}
}

// A second request while the first is still out doubles what the first costs
// and answers the same thing.
func TestAWordAlreadyAskedAboutIsNotAskedAgainWithinTheThrottle(t *testing.T) {
	e, _ := typing(t, "\tfmt.Prin\n", 9)
	asking.start = 5

	at := time.Now()
	now = func() time.Time { return at }
	t.Cleanup(func() { now = time.Now })

	asking.at = at
	if worthAsking(e, 5) {
		t.Error("the word was asked about twice in the same instant")
	}

	now = func() time.Time { return at.Add(refresh) }
	if !worthAsking(e, 5) {
		t.Error("the word is still not being asked about once the throttle has come round")
	}
}

func TestAServerThatDidNotAnswerLeavesNoMenuBehind(t *testing.T) {
	e, _ := typing(t, "\tfmt.Prin\n", 9)
	asking.start = 5
	e.Comp.Show([]complete.Item{{Label: "stale"}}, 0, 5, nil, false)

	offer(e, lsp.Completion{}, lsp.ErrNoAnswer)

	if e.Comp.Open() {
		t.Error("the menu from before the failed request is still up")
	}
}

// The server knows that a Rust '::' is part of the path being completed and a
// Go '.' is not, so where it names a range that is what a candidate replaces.
func TestTheServersOwnRangeIsWhatACandidateReplaces(t *testing.T) {
	e, _ := typing(t, "\tstd::ve\n", 8)

	at := lsp.Range{Start: lsp.Position{Line: 0, Character: 1}}
	items := candidates(e, []lsp.Item{{Label: "std::vec", Text: "std::vec", Edit: &at}}, 6)

	if items[0].From != 1 {
		t.Errorf("the candidate replaces from column %d, want the 1 the server named", items[0].From)
	}
}

// A range starting later than the word would leave half of what was typed
// behind it.
func TestARangeShorterThanTheWordIsWidenedToIt(t *testing.T) {
	e, _ := typing(t, "\tfmt.Prin\n", 9)

	at := lsp.Range{Start: lsp.Position{Line: 0, Character: 7}}
	items := candidates(e, []lsp.Item{{Label: "Println", Text: "Println", Edit: &at}}, 5)

	if items[0].From != 5 {
		t.Errorf("the candidate replaces from column %d, want the start of the word", items[0].From)
	}
}

func TestACandidateWithNothingToSayForItselfIsLabelledWithItsKind(t *testing.T) {
	e, _ := typing(t, "\tra\n", 3)

	items := candidates(e, []lsp.Item{{Label: "range", Kind: 14}}, 1)
	if items[0].Detail != "Keyword" {
		t.Errorf("the candidate reads %q, want the kind it is", items[0].Detail)
	}
}

func TestOnlyTheCandidatesThatAreInvokedAreCalled(t *testing.T) {
	e, _ := typing(t, "\tra\n", 3)

	for kind, want := range map[int]bool{2: true, 3: true, 4: true, 5: false, 6: false, 14: false} {
		items := candidates(e, []lsp.Item{{Label: "x", Kind: kind}}, 1)
		if items[0].Call != want {
			t.Errorf("kind %d called %v, want %v", kind, items[0].Call, want)
		}
	}
}

func TestTabPutsTheSelectedCandidateIn(t *testing.T) {
	e, _ := typing(t, "\tfmt.Prin\n", 9)
	e.Comp.Show([]complete.Item{{Label: "Println", Text: "Println", From: 5}}, 0, 5, nil, false)

	if !CompletionKey(e, termbox.Event{Key: termbox.KeyTab}) {
		t.Fatal("Tab was left to insert spaces with a menu up")
	}
	if got := string(e.Buf.Line(0)); got != "\tfmt.Println" {
		t.Errorf("the line reads %q, want the candidate in it", got)
	}
	if e.Comp.Open() {
		t.Error("the menu is still up after settling on a candidate")
	}
}

func TestEnterSettlesOnTheSelectedCandidate(t *testing.T) {
	e, _ := typing(t, "\tfmt.Prin\n", 9)
	e.Comp.Show([]complete.Item{{Label: "Println", Text: "Println", From: 5}}, 0, 5, nil, false)

	if !CompletionKey(e, termbox.Event{Key: termbox.KeyEnter}) {
		t.Fatal("Enter split the line instead of settling on a candidate")
	}
	if got := string(e.Buf.Line(0)); got != "\tfmt.Println" {
		t.Errorf("the line reads %q, want the candidate in it", got)
	}
}

// A pasted Unix line ending arrives as Ctrl-J rather than Enter's CR, which is
// the only thing telling a paste from typing here: taking it for an accept
// would rewrite every line ending of a paste as whatever the menu had selected.
func TestAPastedLineEndingStillSplitsTheLine(t *testing.T) {
	e, _ := typing(t, "\tfmt.Prin\n", 9)
	e.Comp.Show([]complete.Item{{Label: "Println", Text: "Println", From: 5}}, 0, 5, nil, false)

	if CompletionKey(e, termbox.Event{Key: termbox.KeyCtrlJ}) {
		t.Error("a pasted line ending settled on a candidate")
	}
}

func TestEscapeClosesTheMenuAndLeavesTheTypingWhereItWas(t *testing.T) {
	e, _ := typing(t, "\tfmt.Prin\n", 9)
	e.Comp.Show([]complete.Item{{Label: "Println", Text: "Println", From: 5}}, 0, 5, nil, false)

	if !CompletionKey(e, termbox.Event{Key: termbox.KeyEsc}) {
		t.Fatal("Escape went past the menu to leave Edit mode")
	}
	if e.Comp.Open() {
		t.Error("the menu is still up")
	}
	if e.Mode != state.EditMode {
		t.Error("Escape left Edit mode as well as closing the menu")
	}
}

func TestLeavingEditModeTakesTheMenuWithIt(t *testing.T) {
	e, _ := typing(t, "\tfmt.Prin\n", 9)
	e.Comp.Show([]complete.Item{{Label: "Println", Text: "Println", From: 5}}, 0, 5, nil, false)
	e.Mode = state.ReadMode

	AfterKey(e, termbox.Event{Key: termbox.KeyEsc}, true)

	if e.Comp.Open() {
		t.Error("the menu outlived the mode it belongs to")
	}
}

// The menu follows the word it was opened on and no further.
func TestTheCursorLeavingTheWordClosesTheMenu(t *testing.T) {
	e, _ := typing(t, "\tfmt.Prin\n\tx\n", 9)
	e.Comp.Show([]complete.Item{{Label: "Println", Text: "Println", From: 5}}, 0, 5, nil, false)
	e.Row, e.Col = 1, 2

	AfterKey(e, termbox.Event{Ch: 'x'}, true)

	if e.Comp.Open() {
		t.Error("the menu is still up on another row entirely")
	}
}

func TestTypingOnNarrowsTheMenuWithoutAskingAgain(t *testing.T) {
	e, _ := typing(t, "\tfmt.Prin\n", 9)
	e.Comp.Show([]complete.Item{
		{Label: "Println", Text: "Println", From: 5},
		{Label: "Sscan", Text: "Sscan", From: 5},
	}, 0, 5, nil, false)

	AfterKey(e, termbox.Event{Ch: 'n'}, true)

	if !e.Comp.Open() {
		t.Fatal("the menu closed on a word one of its candidates still matches")
	}
	if e.Comp.Count() != 1 {
		t.Errorf("the menu offers %d candidates, want the one the typing left", e.Comp.Count())
	}
}

// A candidate whose edits the server held back still goes in when the second
// question cannot be asked: the name typed is what was wanted, and an import
// that never arrives is what happens with no server at all.
func TestACandidateWaitingOnASecondQuestionStillGoesIn(t *testing.T) {
	e, _ := typing(t, "\tfmt.Prin\n", 9)
	e.Comp.Show([]complete.Item{{
		Label: "Println", Text: "Println", From: 5,
		Ask: true, Data: json.RawMessage(`{"entry":1}`),
	}}, 0, 5, nil, false)

	if !CompletionKey(e, termbox.Event{Key: termbox.KeyTab}) {
		t.Fatal("Tab did not settle on the candidate")
	}
	if got := string(e.Buf.Line(0)); got != "\tfmt.Println" {
		t.Errorf("the line reads %q, want the candidate in it", got)
	}
}

// These answer for a file with no server, the buffer's own words standing in
// for one — none of them ever cross the "server" answered/typing helpers set
// up, which is what makes those the right harness for this path too.

func TestSuggestWithNoServerOffersWhatUsuallyFollowedTheWordBefore(t *testing.T) {
	e, _ := typing(t, "quick fox\nquick brown\nquick brown\nquick \n", 6)
	e.Row = 3

	Suggest(e)

	if !e.Comp.Open() {
		t.Fatal("no menu went up with no server and words seen before in the buffer")
	}
	if got := e.Comp.Labels(); len(got) == 0 || got[0] != "brown" {
		t.Errorf("Labels() = %v, want brown first (it followed quick more often than fox did)", got)
	}
}

// Typing never asks on its own with no server: nothing here has a trigger
// character or a minimum length to hold it back, so leaving it to answer
// every keystroke would mean the first Escape after almost any word closed a
// menu nobody asked for rather than leaving Edit mode. See suggestLocal.
func TestAfterKeyWithNoServerNeverAsksOnItsOwn(t *testing.T) {
	e, _ := typing(t, "quick brown\nb", 1)
	e.Row = 1

	AfterKey(e, termbox.Event{Ch: 'b'}, true)

	if e.Comp.Open() {
		t.Error("a menu went up from typing alone, with no server and nothing asked outright")
	}
}

func TestSuggestLocalShowsCandidatesAnchoredAtTheWordsStart(t *testing.T) {
	e, _ := typing(t, "quick brown\nquick \n", 6)
	e.Row = 1

	suggestLocal(e)

	if !e.Comp.Open() {
		t.Fatal("suggestLocal found a candidate but did not show it")
	}
	if e.Comp.Row() != 1 || e.Comp.Start() != 6 {
		t.Errorf("menu anchored at row %d col %d, want row 1 col 6", e.Comp.Row(), e.Comp.Start())
	}

	item, ok := e.Comp.Selected()
	if !ok || item.Label != "brown" || item.Text != "brown" || item.From != 6 {
		t.Errorf("top candidate = %+v, want brown with From at the word's start", item)
	}
}

func TestSuggestLocalOpensNoMenuOverAnEmptyBuffer(t *testing.T) {
	e, _ := typing(t, "", 0)

	suggestLocal(e)

	if e.Comp.Open() {
		t.Error("a menu went up over a buffer with no words in it at all")
	}
}

func TestPrevWordFindsTheWordOnItsOwnLineFirst(t *testing.T) {
	e, _ := typing(t, "zero\nalpha bravo\n", 11)
	e.Row = 1

	if got := prevWord(e, 6); got != "alpha" {
		t.Errorf("prevWord() = %q, want alpha, without reaching back to zero on the line before", got)
	}
}

// The same lookup Build's own bigrams use: a word wrapped by the buffer rather
// than the author still carries into what follows it.
func TestPrevWordCrossesALineBreakToFindTheWordBefore(t *testing.T) {
	e, _ := typing(t, "zero alpha\nbravo\n", 0)
	e.Row = 1

	if got := prevWord(e, 0); got != "alpha" {
		t.Errorf("prevWord() = %q, want alpha (nothing word-shaped precedes it on its own line)", got)
	}
}
