// The tests sit outside the package because the harness they share with the
// rest of the editor imports it.
package keys_test

import (
	"testing"

	"github.com/ArditZubaku/tex/internal/editor/edtest"
	"github.com/ArditZubaku/tex/internal/editor/state"
)

func TestCountRepeatsAMotion(t *testing.T) {
	e := state.New()

	edtest.InReadMode(t, e, "a\nb\nc\nd\n", 0, 0)

	edtest.Press(t, e, "3j")

	if e.Row != 3 {
		t.Errorf("currentRow = %d, want 3", e.Row)
	}
}

func TestCountedDeleteRune(t *testing.T) {
	e := state.New()

	b := edtest.InReadMode(t, e, "abcdef\n", 0, 0)

	edtest.Press(t, e, "3x")

	edtest.WantLines(t, b, "def")
	if got := string(e.Clip.Content()[0]); got != "abc" {
		t.Errorf("register holds %q, want %q", got, "abc")
	}
}

func TestCountIsSpentByTheCommandItPrefixes(t *testing.T) {
	e := state.New()

	edtest.InReadMode(t, e, "a\nb\nc\nd\ne\n", 0, 0)

	edtest.Press(t, e, "2jj")

	if e.Row != 3 {
		t.Errorf("currentRow = %d, want 3", e.Row)
	}
	if e.PendingCount != 0 {
		t.Errorf("pendingCount = %d, want it spent", e.PendingCount)
	}
}

func TestMultiDigitCountIsCapped(t *testing.T) {
	e := state.New()

	edtest.InReadMode(t, e, "a\n", 0, 0)

	edtest.Press(t, e, "999999")

	if e.PendingCount != state.MaxCount {
		t.Errorf("pendingCount = %d, want %d", e.PendingCount, state.MaxCount)
	}
}

func TestLeadingZeroDoesNotStartACount(t *testing.T) {
	e := state.New()

	edtest.InReadMode(t, e, "abc\n", 0, 0)

	edtest.Press(t, e, "0")

	if e.PendingCount != 0 {
		t.Errorf("pendingCount = %d, want 0", e.PendingCount)
	}
}

func TestEscCancelsAPendingCount(t *testing.T) {
	e := state.New()

	edtest.InReadMode(t, e, "a\nb\nc\nd\n", 0, 0)

	edtest.Press(t, e, "3")
	edtest.Esc(t, e)
	edtest.Press(t, e, "j")

	if e.Row != 1 {
		t.Errorf("currentRow = %d, want 1", e.Row)
	}
}
