package form

import (
	"strings"
	"testing"

	"github.com/nsf/termbox-go"

	"github.com/ArditZubaku/tex/internal/layout"
	"github.com/ArditZubaku/tex/internal/theme"
)

func twoBoxes(t *testing.T, pattern string) *Form {
	t.Helper()

	var f Form
	f.Show("Find and replace", "hint", NewField("Search", pattern), NewField("Replace", ""))

	return &f
}

func typeInto(t *testing.T, f *Form, txt string) {
	t.Helper()

	for _, ch := range txt {
		event := termbox.Event{Ch: ch}
		if ch == ' ' {
			event = termbox.Event{Key: termbox.KeySpace}
		}
		if action := f.Key(event); action != Typed {
			t.Fatalf("typing %q = %v, want Typed", ch, action)
		}
	}
}

func press(t *testing.T, f *Form, key termbox.Key) Action {
	t.Helper()

	return f.Key(termbox.Event{Key: key})
}

func TestAFieldOpensOnTheTextItWasGiven(t *testing.T) {
	f := twoBoxes(t, "needle")

	if got := string(f.Value(0)); got != "needle" {
		t.Errorf("the first box holds %q, want %q", got, "needle")
	}
	if got := string(f.Value(1)); got != "" {
		t.Errorf("the second box holds %q, want it empty", got)
	}
}

func TestTypingGoesIntoTheBoxTheKeysAreIn(t *testing.T) {
	f := twoBoxes(t, "")

	typeInto(t, f, "one")
	if action := press(t, f, termbox.KeyTab); action != Handled {
		t.Fatalf("Tab = %v, want Handled", action)
	}
	typeInto(t, f, "two")

	if got, want := string(f.Value(0)), "one"; got != want {
		t.Errorf("the first box holds %q, want %q", got, want)
	}
	if got, want := string(f.Value(1)), "two"; got != want {
		t.Errorf("the second box holds %q, want %q", got, want)
	}
}

func TestTabWrapsBackToTheFirstBox(t *testing.T) {
	f := twoBoxes(t, "")

	press(t, f, termbox.KeyTab)
	press(t, f, termbox.KeyTab)

	if got := f.Focus(); got != 0 {
		t.Errorf("focus = %d after wrapping, want 0", got)
	}
}

func TestTheArrowsMoveBetweenTheBoxesTheWayTabDoes(t *testing.T) {
	f := twoBoxes(t, "")

	press(t, f, termbox.KeyArrowDown)
	if got := f.Focus(); got != 1 {
		t.Fatalf("focus = %d after ArrowDown, want 1", got)
	}

	press(t, f, termbox.KeyArrowUp)
	if got := f.Focus(); got != 0 {
		t.Errorf("focus = %d after ArrowUp, want 0", got)
	}
}

func TestBackspaceEditsTheBoxRatherThanClosingTheForm(t *testing.T) {
	f := twoBoxes(t, "ab")

	press(t, f, termbox.KeyBackspace)
	if got := string(f.Value(0)); got != "a" {
		t.Errorf("the box holds %q, want %q", got, "a")
	}

	press(t, f, termbox.KeyBackspace)
	if action := press(t, f, termbox.KeyBackspace); action != Handled {
		t.Errorf("backspace on an empty box = %v, want Handled", action)
	}
	if !f.Open() {
		t.Error("backspacing an empty box closed the form")
	}
}

func TestTheCursorWalksTheTextItIsTyping(t *testing.T) {
	f := twoBoxes(t, "")

	typeInto(t, f, "ac")
	press(t, f, termbox.KeyArrowLeft)
	typeInto(t, f, "b")

	if got, want := string(f.Value(0)), "abc"; got != want {
		t.Errorf("the box holds %q, want %q", got, want)
	}
}

func TestCtrlUClearsBackToTheStartOfTheBox(t *testing.T) {
	f := twoBoxes(t, "")

	typeInto(t, f, "stale")
	press(t, f, termbox.KeyCtrlU)

	if got := string(f.Value(0)); got != "" {
		t.Errorf("the box holds %q, want it cleared", got)
	}
}

func TestEscAndEnterAreTheFormsOwnAnswers(t *testing.T) {
	f := twoBoxes(t, "needle")

	if action := press(t, f, termbox.KeyEnter); action != Submitted {
		t.Errorf("Enter = %v, want Submitted", action)
	}
	if action := press(t, f, termbox.KeyEsc); action != Closed {
		t.Errorf("Esc = %v, want Closed", action)
	}
}

func TestAClosedFormHoldsNothing(t *testing.T) {
	f := twoBoxes(t, "needle")
	f.Close()

	if f.Open() {
		t.Error("the form is still open")
	}
	if got := f.Value(0); got != nil {
		t.Errorf("a closed form holds %q, want nothing", string(got))
	}
}

// The window ends at the cursor, so a pattern longer than its box is shown by
// its tail rather than running off the end of the frame. The last column of the
// box is the cursor's own, which is why four columns show three characters.
func TestALongFieldIsShownByItsTail(t *testing.T) {
	f := twoBoxes(t, "")
	typeInto(t, f, "abcdefghij")

	shown, before := f.fields[0].view(4)
	if shown != "hij" {
		t.Errorf("shown = %q, want %q", shown, "hij")
	}
	if before != 3 {
		t.Errorf("the cursor sits %d columns in, want 3", before)
	}
}

// termbox is never initialised under test, so a cell drawn into it goes
// nowhere; what this covers is the geometry on the way there, which is where a
// frame too small for what it holds would come apart.
func TestDrawingSurvivesTheRoomItIsGiven(t *testing.T) {
	f := twoBoxes(t, strings.Repeat("needle ", 20))
	palette := theme.Default()

	for _, within := range []layout.Rect{
		{Row: 1, Rows: 23, Cols: 80},
		{Row: 1, Rows: 9, Cols: 80},
		{Row: 1, Rows: 4, Cols: 80},
		{Row: 1, Rows: 1, Cols: 80},
		{Row: 1, Rows: 23, Cols: 10},
		{Row: 0, Rows: 0, Cols: 0},
	} {
		f.Draw(within, &palette)
		f.CursorRow(within)
		f.CursorCol(within)
	}
}

// The boxes are centred in the room below the buffer line, and the cursor sits
// inside the one being typed into.
func TestTheCursorSitsInsideTheBoxBeingTypedInto(t *testing.T) {
	f := twoBoxes(t, "needle")
	within := layout.Rect{Row: 1, Rows: 23, Cols: 80}

	frame := f.frame(within)
	for at := range f.fields {
		f.focus = at

		row, col := f.CursorRow(within), f.CursorCol(within)
		if row <= frame.Row || row >= frame.Row+frame.Rows-1 {
			t.Errorf("the cursor is on row %d, outside the frame's own %d..%d", row, frame.Row, frame.Row+frame.Rows-1)
		}
		if col <= frame.Col+2 || col >= frame.Col+frame.Cols-2 {
			t.Errorf("the cursor is at column %d, outside the box's own %d..%d", col, frame.Col+2, frame.Col+frame.Cols-2)
		}
	}
}
