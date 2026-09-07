// The status line's own tests sit outside the package, since the harness they
// share with the rest of the editor imports it.
package render_test

import (
	"strings"
	"testing"

	"github.com/ArditZubaku/tex/internal/editor/diag"
	"github.com/ArditZubaku/tex/internal/editor/edtest"
	"github.com/ArditZubaku/tex/internal/editor/render"
	"github.com/ArditZubaku/tex/internal/editor/screen"
	"github.com/ArditZubaku/tex/internal/editor/state"
)

func onLine(t *testing.T, row int, notes ...diag.Note) *state.Editor {
	t.Helper()

	e := state.New()
	edtest.InReadMode(t, e, strings.Repeat("some line of code\n", 12), row, 0)
	edtest.SingleWindow(e, 20, 80)
	t.Cleanup(diag.Reset)

	if len(notes) > 0 {
		diag.Set(e.SourceFile, notes)
	}

	return e
}

func TestTheStatusLineCountsWhatAServerSaid(t *testing.T) {
	for _, tc := range []struct {
		name  string
		notes []diag.Note
		want  string
	}{
		{
			name: "errors and warnings",
			notes: []diag.Note{
				{Severity: diag.Error, Row: 0},
				{Severity: diag.Error, Row: 1},
				{Severity: diag.Warning, Row: 2},
			},
			want: "[2E 1W]",
		},
		{
			name:  "warnings alone",
			notes: []diag.Note{{Severity: diag.Warning, Row: 4}},
			want:  "[1W]",
		},
		{
			name:  "errors alone",
			notes: []diag.Note{{Severity: diag.Error, Row: 4}},
			want:  "[1E]",
		},
		{
			name:  "nothing worth a flag",
			notes: []diag.Note{{Severity: diag.Hint, Row: 4}},
			want:  "",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := onLine(t, 8, tc.notes...)

			got := render.Status(e)
			switch {
			case tc.want == "" && (strings.Contains(got, "E]") || strings.Contains(got, "W]")):
				t.Errorf("status line %q carries a count", got)
			case tc.want != "" && !strings.Contains(got, tc.want):
				t.Errorf("status line %q, want it to carry %q", got, tc.want)
			}
		})
	}
}

func TestTheStatusLineReportsTheWorstThingSaidAboutTheCursorsLine(t *testing.T) {
	e := onLine(t, 3,
		diag.Note{Severity: diag.Hint, Message: "this is only a hint", Row: 3, EndRow: 3, EndCol: 4},
		diag.Note{Severity: diag.Error, Message: "undefined: fooBar", Row: 3, EndRow: 3, EndCol: 9},
	)

	got := render.Status(e)
	if !strings.Contains(got, "undefined: fooBar") {
		t.Errorf("status line %q, want the error on the cursor's line", got)
	}
	if strings.Contains(got, "only a hint") {
		t.Errorf("status line %q carries the milder of the two", got)
	}
}

func TestTheMessageGoesTheMomentTheCursorLeavesTheLine(t *testing.T) {
	e := onLine(t, 3, diag.Note{
		Severity: diag.Error, Message: "undefined: fooBar", Row: 3, EndRow: 3, EndCol: 9,
	})

	if !strings.Contains(render.Status(e), "undefined: fooBar") {
		t.Fatal("the message was not there to leave")
	}

	e.Down()

	if got := render.Status(e); strings.Contains(got, "undefined: fooBar") {
		t.Errorf("status line %q still carries it a line down", got)
	}
}

func TestWhatACommandReportsStillTakesTheWholeStatusLine(t *testing.T) {
	e := onLine(t, 3, diag.Note{
		Severity: diag.Error, Message: "undefined: fooBar", Row: 3, EndRow: 3, EndCol: 9,
	})
	e.StatusMsg = "E486: Pattern not found: qqq"

	got := render.Status(e)
	if strings.TrimSpace(got) != e.StatusMsg {
		t.Errorf("status line %q, want only what the command reported", got)
	}
}

func TestALongMessageIsCutRatherThanPushingTheRowAndColumnOff(t *testing.T) {
	e := onLine(t, 3, diag.Note{
		Severity: diag.Error,
		Message:  strings.Repeat("something the server had a great deal to say about ", 8),
		Row:      3, EndRow: 3, EndCol: 9,
	})

	got := render.Status(e)
	if screen.Width(got) != e.ScreenCols {
		t.Errorf("status line is %d columns wide, want %d: %q", screen.Width(got), e.ScreenCols, got)
	}
	if !strings.Contains(got, "Row 4, Col 1") {
		t.Errorf("status line %q, want it still to say where the cursor is", got)
	}
}

// A message is not worth the two words that would fit in what a narrow screen
// leaves once everything else has had its width.
func TestANarrowScreenKeepsWhatTheBarAlreadySaid(t *testing.T) {
	e := onLine(t, 3, diag.Note{
		Severity: diag.Error, Message: "undefined: fooBar", Row: 3, EndRow: 3, EndCol: 9,
	})
	e.ScreenCols = 46

	if got := render.Status(e); strings.Contains(got, "undefi") {
		t.Errorf("status line %q, want no room found for a message", got)
	}
}
