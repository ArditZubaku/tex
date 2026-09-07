package find_test

import (
	"strings"
	"testing"

	"github.com/nsf/termbox-go"

	"github.com/ArditZubaku/tex/internal/editor/command"
	"github.com/ArditZubaku/tex/internal/editor/diag"
	"github.com/ArditZubaku/tex/internal/editor/edtest"
	"github.com/ArditZubaku/tex/internal/editor/state"
)

func markedUp(t *testing.T, notes ...diag.Note) *state.Editor {
	t.Helper()

	e := state.New()
	edtest.InReadMode(t, e, strings.Repeat("some line of code\n", 20), 0, 0)
	edtest.SingleWindow(e, 10, 80)
	t.Cleanup(diag.Reset)

	diag.Set(e.SourceFile, notes)

	return e
}

func TestWalkingTheDiagnosticsOfAFileWrapsAtEitherEnd(t *testing.T) {
	e := markedUp(t,
		diag.Note{Severity: diag.Error, Row: 3, Col: 2},
		diag.Note{Severity: diag.Warning, Row: 3, Col: 9},
		diag.Note{Severity: diag.Error, Row: 11, Col: 0},
	)

	for _, want := range [][2]int{{3, 2}, {3, 9}, {11, 0}, {3, 2}} {
		edtest.Press(t, e, "]d")
		edtest.WantCursor(t, e, want[0], want[1])
	}

	for _, want := range [][2]int{{11, 0}, {3, 9}, {3, 2}, {11, 0}} {
		edtest.Press(t, e, "[d")
		edtest.WantCursor(t, e, want[0], want[1])
	}
}

func TestWalkingAFileWithNothingSaidAboutItReportsSo(t *testing.T) {
	e := markedUp(t)

	edtest.Press(t, e, "]d")

	edtest.WantCursor(t, e, 0, 0)
	if e.StatusMsg == "" {
		t.Error("walking a clean file said nothing")
	}
}

// The jump list is for jumps: a walk through one file is what Ctrl-O is meant
// to come back past, not into.
func TestWalkingTheDiagnosticsIsNotAJump(t *testing.T) {
	e := markedUp(t, diag.Note{Severity: diag.Error, Row: 14, Col: 0})

	edtest.Press(t, e, "]d")
	if _, ok := e.PopJump(); ok {
		t.Error("walking to a diagnostic remembered a jump")
	}
}

func TestADiagnosticPastTheEndOfTheFileLandsOnItsLastLine(t *testing.T) {
	e := markedUp(t, diag.Note{Severity: diag.Error, Row: 900, Col: 4})

	edtest.Press(t, e, "]d")

	if e.Row != e.Buf.LineCount()-1 {
		t.Errorf("cursor on line %d, want the last line %d", e.Row, e.Buf.LineCount()-1)
	}
}

func TestListingEveryDiagnosticPutsTheWorstFirst(t *testing.T) {
	e := markedUp(t,
		diag.Note{Severity: diag.Warning, Message: "unused variable", Row: 4},
		diag.Note{Severity: diag.Error, Message: "undefined: fooBar", Row: 9},
		diag.Note{Severity: diag.Hint, Message: "could be simplified", Row: 1},
	)

	command.Run(e, "diag")

	if e.Mode != state.PickerMode {
		t.Fatalf("':diag' left the editor in %v", e.Mode)
	}

	labels := matchedLabels(e)
	if len(labels) != 3 {
		t.Fatalf("':diag' listed %v, want three rows", labels)
	}
	for i, want := range []string{"E ", "W ", "H "} {
		if !strings.HasPrefix(labels[i], want) {
			t.Errorf("row %d is %q, want it to start %q", i, labels[i], want)
		}
	}
	if !strings.Contains(labels[0], "undefined: fooBar") {
		t.Errorf("the first row is %q, want the error's own message", labels[0])
	}
}

func TestListingWithNothingSaidAboutAnythingReportsSo(t *testing.T) {
	e := markedUp(t)

	command.Run(e, "diag")

	if e.Mode == state.PickerMode {
		t.Error("':diag' opened an empty popup")
	}
	if e.StatusMsg == "" {
		t.Error("':diag' said nothing at all")
	}
}

func TestChoosingADiagnosticLandsOnIt(t *testing.T) {
	e := markedUp(t, diag.Note{Severity: diag.Error, Message: "undefined: fooBar", Row: 7, Col: 3})

	command.Run(e, "diag")
	edtest.PressKey(t, e, termbox.KeyEnter)

	edtest.WantCursor(t, e, 7, 3)
	if e.Mode != state.ReadMode {
		t.Errorf("choosing a row left the editor in %v", e.Mode)
	}
}

// A server says several lines often enough — a type mismatch spelled out over
// three — and nothing the editor draws is more than one.
func TestAMessageOfSeveralLinesIsJoinedIntoOne(t *testing.T) {
	e := markedUp(t, diag.Note{
		Severity: diag.Error,
		Message:  "cannot use x\n\t(variable of type int)\n\tas string value",
		Row:      2,
	})

	if got := diag.Of(e.SourceFile)[0].Message; strings.ContainsAny(got, "\n\t") {
		t.Errorf("the message is %q, want it on one line", got)
	}
}
