package state

import (
	"encoding/base64"
	"log/slog"
	"os"

	"github.com/ArditZubaku/tex/internal/editor/register"
)

// SetClip is the one path every yank and delete writes the register through,
// so the system clipboard stays in step with it without every call site
// having to remember to say so.
func (e *Editor) SetClip(r register.Register) {
	e.Clip = r
	CopyToClipboard(r.Text())
}

// CopyToClipboard puts text on the system clipboard through OSC 52, the
// escape sequence a terminal reads for it directly, rather than a tool like
// pbcopy that would need installing and cannot reach past an SSH session.
// A terminal that does not understand OSC 52 just discards it, the same
// trade SetCursorShape already makes for its own escape sequence.
func CopyToClipboard(text string) {
	if text == "" {
		return
	}

	encoded := base64.StdEncoding.EncodeToString([]byte(text))
	if _, err := os.Stdout.WriteString("\033]52;c;" + encoded + "\a"); err != nil {
		slog.Error("Failed to copy to system clipboard", "error", err)
	}
}
