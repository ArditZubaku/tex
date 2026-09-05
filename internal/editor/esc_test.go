package editor

import (
	"testing"

	"github.com/ArditZubaku/tex/internal/editor/edtest"
	"github.com/ArditZubaku/tex/internal/editor/state"
)

func TestEscStepsOffTheGap(t *testing.T) {
	edtest.AtCursor(t, ed, "package main\n", 0, 12)
	ed.Mode = state.EditMode

	esc()

	if ed.Mode != state.ReadMode || ed.Col != 11 {
		t.Errorf("mode %v, currentCol = %d, want ReadMode, 11", ed.Mode, ed.Col)
	}
}
