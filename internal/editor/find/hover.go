package find

import (
	"path/filepath"

	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/ArditZubaku/tex/internal/lsp"
)

// 'K' is what a language server knows about the identifier under the cursor —
// its signature, its type, the doc comment above it — in a box beside it, which
// is VIM's own 'K' with the server where the man page was. There is nothing
// behind it in the text: a doc comment read out of the file is what 'gd' is
// for, and it goes to the file rather than quoting it back.
func ShowHover(e *state.Editor) {
	if !lsp.Ready(e.SourceFile) {
		e.StatusMsg = "no language server for " + filepath.Base(e.SourceFile)

		return
	}

	token, from := ask(e)
	lsp.Hover(e.SourceFile, e.Buf, e.Row, e.Col, func(text string, err error) {
		if !stale(e, token, from) {
			showHover(e, text, err)
		}
	})
}

func showHover(e *state.Editor, text string, err error) {
	if err != nil {
		e.StatusMsg = "the language server did not answer"

		return
	}

	e.Hov.Show(text, e.ScreenArea())
	if !e.Hov.Showing() {
		e.StatusMsg = "nothing known about what is under the cursor"
	}
}
