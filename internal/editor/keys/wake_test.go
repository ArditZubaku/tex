// The tests sit outside the package because the harness they share with the
// rest of the editor imports it.
package keys_test

import (
	"testing"

	"github.com/nsf/termbox-go"

	"github.com/ArditZubaku/tex/internal/editor/edtest"
	"github.com/ArditZubaku/tex/internal/editor/keys"
	"github.com/ArditZubaku/tex/internal/editor/state"
)

func wake(t *testing.T, e *state.Editor) {
	t.Helper()

	keys.Handle(e, termbox.Event{Type: termbox.EventInterrupt}, false)
}

func resize(t *testing.T, e *state.Editor) {
	t.Helper()

	keys.Handle(e, termbox.Event{Type: termbox.EventResize, Width: 100, Height: 40}, false)
}

func TestAChordSurvivesTheLoopBeingWokenBetweenItsKeys(t *testing.T) {
	e := state.New()

	b := edtest.InReadMode(t, e, "a\nb\nc\n", 1, 0)

	edtest.Press(t, e, "d")
	wake(t, e)
	edtest.Press(t, e, "d")

	edtest.WantLines(t, b, "a", "c")
}

func TestAChordSurvivesAResizeBetweenItsKeys(t *testing.T) {
	e := state.New()

	b := edtest.InReadMode(t, e, "a\nb\nc\n", 1, 0)

	edtest.Press(t, e, "d")
	resize(t, e)
	edtest.Press(t, e, "d")

	edtest.WantLines(t, b, "a", "c")
}

func TestACountSurvivesTheLoopBeingWoken(t *testing.T) {
	e := state.New()

	edtest.InReadMode(t, e, "a\nb\nc\nd\ne\n", 0, 0)

	edtest.Press(t, e, "3")
	wake(t, e)
	edtest.Press(t, e, "j")

	if e.Row != 3 {
		t.Errorf("currentRow = %d, want 3", e.Row)
	}
}

func TestWhatTheLastCommandReportedSurvivesTheLoopBeingWoken(t *testing.T) {
	e := state.New()

	edtest.InReadMode(t, e, "a\n", 0, 0)
	e.StatusMsg = "E492: Not an editor command: nope"

	wake(t, e)

	if e.StatusMsg == "" {
		t.Error("StatusMsg was cleared by a wake-up, want it left to be redrawn")
	}
}

func TestTheBoxAServerAnswerLandedInSurvivesTheWakeUpThatBroughtIt(t *testing.T) {
	e := state.New()

	edtest.InReadMode(t, e, "a\n", 0, 0)
	edtest.SingleWindow(e, 20, 80)
	e.Hov.Show("Run is the editor.", e.ScreenArea())

	wake(t, e)

	if !e.Hov.Showing() {
		t.Error("the box was cleared by the wake-up that put it there")
	}
}

func TestTheNextKeyTakesTheBoxDown(t *testing.T) {
	e := state.New()

	edtest.InReadMode(t, e, "a\nb\n", 0, 0)
	edtest.SingleWindow(e, 20, 80)
	e.Hov.Show("Run is the editor.", e.ScreenArea())

	keys.Handle(e, termbox.Event{Ch: 'j'}, true)

	if e.Hov.Showing() {
		t.Errorf("the box is still showing %q", e.Hov.Text())
	}
}
