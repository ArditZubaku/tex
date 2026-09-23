// The tests sit outside the package because the harness they share with the
// rest of the editor imports it.
package state_test

import (
	"testing"

	"github.com/ArditZubaku/tex/internal/editor/state"
)

func TestNotifyAreaUsesTheRealWidthNotTheFlooredOne(t *testing.T) {
	e := state.New()
	e.ScreenRows, e.ScreenCols, e.RealScreenCols = 24, 80, 45

	if got := e.NotifyArea().Cols; got != 45 {
		t.Errorf("NotifyArea().Cols = %d, want the real 45", got)
	}
	if got := e.ScreenArea().Cols; got != 80 {
		t.Errorf("ScreenArea().Cols = %d, want the floored 80, unaffected by NotifyArea", got)
	}
}
