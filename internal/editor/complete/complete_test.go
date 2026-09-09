package complete

import (
	"strings"
	"testing"

	"github.com/ArditZubaku/tex/internal/layout"
	"github.com/ArditZubaku/tex/internal/theme"
)

func offered(names ...string) []Item {
	items := make([]Item, 0, len(names))
	for _, name := range names {
		items = append(items, Item{Label: name, Text: name})
	}

	return items
}

func listed(m *Menu) string { return strings.Join(m.Labels(), ",") }

func TestTheServersOwnRankingStandsUntilSomethingIsTypedAgainstIt(t *testing.T) {
	var m Menu
	m.Show(offered("Println", "Print", "Printf"), 0, 0, nil, false)

	if got := listed(&m); got != "Println,Print,Printf" {
		t.Errorf("the menu lists %q, want the order it was given", got)
	}
}

func TestTypingNarrowsTheCandidatesWithoutAskingAgain(t *testing.T) {
	var m Menu
	m.Show(offered("Println", "Fprintln", "Sscan"), 0, 0, nil, false)

	if !m.Retype([]rune("pr")) {
		t.Fatal("the menu closed on a query two of its candidates match")
	}
	if got := listed(&m); got != "Println,Fprintln" {
		t.Errorf("the menu lists %q, want the two matching candidates", got)
	}
}

func TestAQueryNothingMatchesClosesTheMenuRatherThanLeavingItEmpty(t *testing.T) {
	var m Menu
	m.Show(offered("Println"), 0, 0, nil, false)

	if m.Retype([]rune("zzz")) {
		t.Error("a menu with nothing left in it stayed up")
	}
	if m.Open() {
		t.Error("the menu is still open")
	}
}

func TestCandidatesNoneOfWhichMatchNeverOpenTheMenu(t *testing.T) {
	var m Menu

	if m.Show(offered("Println"), 0, 0, []rune("zzz"), false) {
		t.Error("a menu opened on candidates the typing had already ruled out")
	}
}

func TestTheSelectionWrapsBothWays(t *testing.T) {
	var m Menu
	m.Show(offered("one", "two", "three"), 0, 0, nil, false)

	m.Prev()
	if item, _ := m.Selected(); item.Label != "three" {
		t.Errorf("stepping back from the first lands on %q, want the last", item.Label)
	}

	m.Next()
	if item, _ := m.Selected(); item.Label != "one" {
		t.Errorf("stepping on from the last lands on %q, want the first", item.Label)
	}
}

func TestClosingTheMenuForgetsWhatItHeld(t *testing.T) {
	var m Menu
	m.Show(offered("Println"), 3, 7, nil, true)
	m.Close()

	if m.Open() || m.Count() != 0 || m.Incomplete() {
		t.Error("the menu is still holding what it was showing")
	}
	if _, ok := m.Selected(); ok {
		t.Error("a closed menu still has a candidate selected")
	}
}

func TestTheMenuKnowsWhichWordItIsAbout(t *testing.T) {
	var m Menu
	m.Show(offered("Println"), 3, 7, []rune("Pr"), false)

	if m.Row() != 3 || m.Start() != 7 {
		t.Errorf("the menu is about %d,%d, want 3,7", m.Row(), m.Start())
	}
	if string(m.Query()) != "Pr" {
		t.Errorf("the menu was typed %q against, want %q", string(m.Query()), "Pr")
	}
}

// A menu drawn from the cursor would put the candidates one word to the right
// of what they replace.
func TestTheMenuIsDrawnFromTheStartOfTheWordRatherThanTheCursor(t *testing.T) {
	var m Menu
	m.Show(offered("Println"), 0, 4, []rune("Pri"), false)

	frame := m.frame(layout.Rect{Row: 1, Rows: 20, Cols: 80}, 5, 20)
	if frame.Col != 17 {
		t.Errorf("the menu is drawn at column %d, want three back from the cursor", frame.Col)
	}
	if frame.Row != 6 {
		t.Errorf("the menu is drawn on row %d, want the one under the cursor", frame.Row)
	}
}

func TestAMenuWithNoRoomUnderTheCursorIsDrawnAboveIt(t *testing.T) {
	var m Menu
	m.Show(offered("one", "two", "three"), 0, 0, nil, false)

	frame := m.frame(layout.Rect{Row: 1, Rows: 20, Cols: 80}, 19, 0)
	if frame.Row != 16 {
		t.Errorf("the menu is drawn on row %d, want the three rows above the cursor", frame.Row)
	}
}

// A signature is what tells two candidates of the same name apart, and the name
// is what is being chosen between: the signature is cut first.
func TestADetailTooLongForTheRowIsCutBeforeTheNameIs(t *testing.T) {
	var m Menu
	m.Show([]Item{{Label: "Println", Detail: "func(a ...any) (n int, err error)"}}, 0, 0, nil, false)

	row := m.rowText(m.items[0], 24)
	if !strings.Contains(row, "Println") {
		t.Errorf("the row reads %q, without the name", row)
	}
	if len([]rune(row)) != 24 {
		t.Errorf("the row is %d columns, want the 24 it was given", len([]rune(row)))
	}
}

func TestADetailWithNoRoomLeftForItGoesRatherThanTheName(t *testing.T) {
	var m Menu
	m.Show([]Item{{Label: "Println", Detail: "func(a ...any)"}}, 0, 0, nil, false)

	if row := m.rowText(m.items[0], 12); strings.TrimSpace(row) != "Println" {
		t.Errorf("the row reads %q, want the name on its own", row)
	}
}

func TestNothingIsDrawnForAMenuThatIsNotOpen(t *testing.T) {
	var m Menu
	palette := theme.Default()

	m.Draw(layout.Rect{Row: 1, Rows: 20, Cols: 80}, 5, 5, &palette)
}
