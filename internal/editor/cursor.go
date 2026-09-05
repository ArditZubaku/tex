package editor

import (
	"log/slog"
	"os"
)

// ANSI DECSCUSR escape sequences for cursor styles
const (
	CursorDefault       = "\033[0 q" // Terminal default
	CursorBlinkingBlock = "\033[1 q"
	CursorSteadyBlock   = "\033[2 q"
	CursorBlinkingUnder = "\033[3 q"
	CursorSteadyUnder   = "\033[4 q"
	CursorBlinkingBar   = "\033[5 q" // Blinking vertical line
	CursorSteadyBar     = "\033[6 q" // Steady vertical line
)

func setCursorShape(shape string) {
	_, err := os.Stdout.WriteString(shape)
	if err != nil {
		slog.Error("Failed to update cursor's shape", "error", err)
	}
}
