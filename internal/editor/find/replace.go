package find

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/nsf/termbox-go"

	"github.com/ArditZubaku/tex/internal/decl"
	"github.com/ArditZubaku/tex/internal/editor/form"
	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/ArditZubaku/tex/internal/search"
)

// '<leader>fr' is find and replace: the two boxes it asks in, by the order the
// form holds them.
const (
	patternBox = iota
	withBox
)

const replaceHint = "Tab: next box · Enter: replace · Esc: cancel"

// OpenReplace opens the two boxes on the word under the cursor, or on the last
// pattern searched for when the cursor is not on a word, since either is more
// often the thing being replaced than an empty box is. What is typed into the
// first of them is the '/' search's own pattern, which is what lights every
// match of it on screen while the boxes are still up.
func OpenReplace(e *state.Editor) {
	pattern := string(e.SearchPat.Runes())
	if word, ok := decl.Under(e.Buf.Line(e.Row), e.Col); ok {
		pattern = word
	}

	e.Mode = state.FormMode
	e.Form.Show("Find and replace", replaceHint,
		form.NewField("Search", pattern),
		form.NewField("Replace", ""))
	litMatches(e)
}

func ReplaceKey(e *state.Editor, event termbox.Event) {
	switch e.Form.Key(event) {
	case form.Typed:
		litMatches(e)
	case form.Closed:
		closeReplace(e)
	case form.Submitted:
		pattern := slices.Clone(e.Form.Value(patternBox))
		with := slices.Clone(e.Form.Value(withBox))
		closeReplace(e)
		RunReplace(e, pattern, with)
	}
}

func DrawReplace(e *state.Editor) { e.Form.Draw(e.ScreenArea(), &e.Palette) }

// litMatches hands what the Search box holds to the '/' search, so that the
// renderer lights the matches of a pattern still being typed with no second
// idea of what a match is. The pattern outlives the box — a replacement
// cancelled still leaves 'n' and 'N' walking what was typed — so the box's own
// slice is copied rather than kept.
func litMatches(e *state.Editor) {
	e.SearchPat, e.HlSearch = search.New(slices.Clone(e.Form.Value(patternBox))), true
}

func closeReplace(e *state.Editor) {
	e.Mode = state.ReadMode
	e.Form.Close()
	e.ClampCol()
}

// RunReplace puts what the second box holds in place of every match of the
// first, in the file being edited and nowhere else: a replacement reaching the
// files beside it would be a rename, which '<leader>cr' already is. The whole
// of it is one change in the history, so a single 'u' puts it all back.
func RunReplace(e *state.Editor, pattern, with []rune) {
	if len(pattern) == 0 {
		e.StatusMsg = "E471: Argument required"
		return
	}

	e.BeginChange()
	changes, lines := replaceAll(e, search.New(pattern), with)
	e.EndChange()

	if changes == 0 {
		e.StatusMsg = "Pattern not found: " + string(pattern)
		return
	}

	e.Modified = true
	e.ClampCol()
	e.StatusMsg = fmt.Sprintf("%s on %s", tally(changes, "substitution"), tally(lines, "line"))
}

func replaceAll(e *state.Editor, pattern search.Pattern, with []rune) (changes, lines int) {
	var cols []int

	for row := range e.Buf.LineCount() {
		cols = pattern.MatchesIn(e.Buf, row, cols[:0])
		if len(cols) == 0 {
			continue
		}

		// The line is read before the history is told about it, since both go
		// through the buffer's own read window and the first of them aliases it.
		updated, hits := spliced(e.Buf.Line(row), cols, len(pattern.Runes()), with)
		if hits == 0 {
			continue
		}

		e.TouchLine(row)
		e.Buf.SetLine(row, updated)
		changes, lines = changes+hits, lines+1
	}

	return changes, lines
}

// spliced is one line with every match on it replaced. The columns are the
// search's own, which overlap where a pattern overlaps itself ("aa" twice over
// in "aaa"), and a replacement cannot: a match reaching back into one already
// taken is skipped, which is the left-to-right pass VIM's own ':s' makes.
//
// The result is a slice of its own rather than a scratch buffer reused down the
// file, since the buffer keeps whatever line it is handed.
func spliced(line []rune, cols []int, width int, with []rune) ([]rune, int) {
	taken := make([]int, 0, len(cols))
	at := 0
	for _, col := range cols {
		if col < at {
			continue
		}
		taken = append(taken, col)
		at = col + width
	}
	if len(taken) == 0 {
		return nil, 0
	}

	out := make([]rune, 0, len(line)+len(taken)*(len(with)-width))
	at = 0
	for _, col := range taken {
		out = append(out, line[at:col]...)
		out = append(out, with...)
		at = col + width
	}

	return append(out, line[at:]...), len(taken)
}

func tally(count int, what string) string {
	if count == 1 {
		return "1 " + what
	}

	return strconv.Itoa(count) + " " + what + "s"
}
