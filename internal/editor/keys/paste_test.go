// The tests sit outside the package because the harness they share with the
// rest of the editor imports it.
package keys_test

import (
	"testing"

	"github.com/nsf/termbox-go"

	"github.com/ArditZubaku/tex/internal/editor/edtest"
	"github.com/ArditZubaku/tex/internal/editor/state"
)

// With no bracketed-paste mode, a pasted Unix line ending arrives as a bare
// LF (Ctrl-J) rather than Enter's CR, and has to split the line the same way.
func TestPastedUnixLineEndingSplitsTheLine(t *testing.T) {
	e := state.New()

	b := edtest.InReadMode(t, e, "\n", 0, 0)
	edtest.Press(t, e, "i")
	edtest.Press(t, e, "ab")
	edtest.PressKey(t, e, termbox.KeyCtrlJ)
	edtest.Press(t, e, "cd")

	edtest.WantLines(t, b, "ab", " cd")
}

// A pasted CRLF line ending arrives as Enter's CR followed immediately by the
// LF Ctrl-J shares with a bare paste; the LF has to be dropped rather than
// splitting the line a second time.
func TestPastedCRLFLineEndingSplitsOnlyOnce(t *testing.T) {
	e := state.New()

	b := edtest.InReadMode(t, e, "\n", 0, 0)
	edtest.Press(t, e, "i")
	edtest.Press(t, e, "ab")
	edtest.PressKey(t, e, termbox.KeyEnter)
	edtest.PressKey(t, e, termbox.KeyCtrlJ)
	edtest.Press(t, e, "cd")

	edtest.WantLines(t, b, "ab", " cd")
}
