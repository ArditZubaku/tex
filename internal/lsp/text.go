package lsp

import (
	"errors"
	"strings"

	"github.com/ArditZubaku/tex/internal/buffer"
)

// A document past this is not sent. The editor holds a 23MB file in 8.8MB
// because it never loads one, and handing a server the whole of it would spend
// more than every answer about it is worth.
const maxDocument = 1 << 20

var errTooLarge = errors.New("lsp: document too large to send")

// text is the whole of a buffer, rendered the way a save renders it. It goes
// through the buffer's own writing rather than reading the file back, because
// the file is what the buffer has not written yet — and because a second
// renderer is a second chance to disagree about a trailing newline, which would
// put every answer a line out with nothing on screen to say why.
func text(b *buffer.Buffer) (string, error) {
	out := &capped{limit: maxDocument}

	written, err := b.WriteLines(out, starts)
	starts = written.Starts
	if err != nil {
		return "", err
	}

	return out.built.String(), nil
}

// capped refuses rather than grows. The refusal reaches the caller through the
// buffered writer WriteLines flushes, whose error is sticky.
type capped struct {
	built strings.Builder
	limit int
}

func (c *capped) Write(p []byte) (int, error) {
	if c.built.Len()+len(p) > c.limit {
		return 0, errTooLarge
	}

	return c.built.Write(p)
}
