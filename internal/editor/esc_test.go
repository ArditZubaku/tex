package editor

import (
	"testing"

	"github.com/ArditZubaku/tex/internal/editor/edtest"
	"github.com/ArditZubaku/tex/internal/editor/state"
)

func TestEscStepsOffTheGap(t *testing.T) {
	e := state.New()

	edtest.AtCursor(t, e, "package main\n", 0, 12)
	e.Mode = state.EditMode

	edtest.Esc(t, e)

	if e.Mode != state.ReadMode || e.Col != 11 {
		t.Errorf("mode %v, currentCol = %d, want ReadMode, 11", e.Mode, e.Col)
	}
}
