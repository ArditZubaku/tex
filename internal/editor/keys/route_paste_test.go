// The tests sit outside the package because the harness they share with the
// rest of the editor imports it.
package keys_test

import (
	"strings"
	"testing"
	"time"

	"github.com/ArditZubaku/tex/internal/editor/edtest"
	"github.com/ArditZubaku/tex/internal/editor/keys"
	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/ArditZubaku/tex/internal/editor/terminal"
	"github.com/ArditZubaku/tex/internal/editor/view"
)

// A paste in Edit mode is spliced into the buffer verbatim: no indent copied
// onto the new lines, no bracket the paste already closed closed again.
func TestPasteInEditModeInsertsVerbatim(t *testing.T) {
	e := state.New()

	b := edtest.InReadMode(t, e, "\n", 0, 0)
	e.Mode = state.EditMode

	keys.Paste(e, "fn main() {\n    let city = String::new();\n}")

	edtest.WantLines(t, b, "fn main() {", "    let city = String::new();", "}")
	edtest.WantCursor(t, e, 2, 1)
}

// A paste elsewhere is read back through Dispatch a character at a time, so
// it keeps meaning whatever it would as typed keys — a count here, not text.
func TestPasteInReadModeReplaysAsOrdinaryKeys(t *testing.T) {
	e := state.New()

	b := edtest.InReadMode(t, e, "abcdef\n", 0, 0)

	keys.Paste(e, "3x")

	edtest.WantLines(t, b, "def")
}

// A '\n' inside such a paste has to arrive as Enter, not a rune ReadMode has
// no notion of, so it means what Enter already means there: a line down.
func TestPasteInReadModeTranslatesNewlinesToEnter(t *testing.T) {
	e := state.New()

	edtest.InReadMode(t, e, "a\nb\nc\n", 0, 0)

	keys.Paste(e, "\n")

	edtest.WantCursor(t, e, 1, 0)
}

func snapshotContains(cells [][]terminal.Cell, want string) bool {
	for _, row := range cells {
		var line []rune
		for _, c := range row {
			line = append(line, c.Ch)
		}
		if strings.Contains(string(line), want) {
			return true
		}
	}

	return false
}

func TestPasteOnAFocusedTerminalForwardsItAsOneWrite(t *testing.T) {
	e := state.New()

	edtest.InReadMode(t, e, "\n", 0, 0)
	edtest.SingleWindow(e, 20, 80)

	edtest.Press(t, e, " ft")
	term := view.Focused()
	t.Cleanup(term.Entry.Terminal.Kill)

	keys.Paste(e, "echo paste-came-through\n")

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if snapshotContains(term.Entry.Terminal.Snapshot(), "paste-came-through") {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("the shell never saw the pasted command")
}
