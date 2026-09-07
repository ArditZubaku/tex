package find

import (
	"regexp"

	"github.com/ArditZubaku/tex/internal/decl"
	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/ArditZubaku/tex/internal/editor/view"
	"github.com/ArditZubaku/tex/internal/lsp"
	"github.com/ArditZubaku/tex/internal/project"
)

// 'gd' is VIM's jump to a declaration. A language server knows the answer
// properly and is asked when there is one; failing that the identifier under
// the cursor is looked for in the text, in the shapes a declaration takes,
// strongest first and nearest above the cursor first of all.

type definition struct {
	path string
	row  int
	col  int
	rank int
}

func GoToDefinition(e *state.Editor) {
	if lsp.Ready(e.SourceFile) {
		askDefinition(e)

		return
	}

	definitionInText(e)
}

// A server that finds nothing, refuses, or never answers at all leaves the
// text search to answer instead, which is what the editor does when there is
// no server: a hung one degrades to that after a couple of seconds rather than
// to nothing at all.
func askDefinition(e *state.Editor) {
	token, from := ask(e)
	lsp.Definition(e.SourceFile, e.Buf, e.Row, e.Col, func(found []lsp.Location, err error) {
		if !stale(e, token, from) {
			goToFirst(e, found, err)
		}
	})
}

func goToFirst(e *state.Editor, found []lsp.Location, err error) {
	if err != nil || len(found) == 0 {
		definitionInText(e)

		return
	}

	path, row, col, ok := place(found[0])
	if !ok {
		definitionInText(e)

		return
	}

	view.Goto(e, path, row, col)
}

func definitionInText(e *state.Editor) {
	word, ok := wordUnderCursor(e)
	if !ok {
		e.StatusMsg = "E349: No identifier under the cursor"
		return
	}

	forms, err := decl.Forms(word)
	if err != nil {
		e.StatusMsg = "E486: Pattern not found: " + word
		return
	}

	if found, ok := definitionAbove(e, forms); ok {
		view.Goto(e, found.path, found.row, found.col)
		return
	}

	if found, ok := definitionInBuffer(e, forms); ok {
		view.Goto(e, found.path, found.row, found.col)
		return
	}

	found, ok := definitionInFiles(e, forms)
	if !ok {
		e.StatusMsg = "E388: Couldn't find definition of " + word
		return
	}

	view.Goto(e, found.path, found.row, found.col)
}

// JumpBack is Ctrl-O: the cursor goes back to where the last jump left from,
// switching buffers when the jump crossed files.
func JumpBack(e *state.Editor) {
	back, ok := e.PopJump()
	if !ok {
		e.StatusMsg = "E664: Jump list is empty"
		return
	}

	if back.Path != e.SourceFile {
		view.Open(e, back.Path)
	}
	e.Row, e.Col = min(back.Row, e.Buf.LineCount()-1), back.Col
	e.ClampCol()
	e.CenterIfOffScreen()
}

func wordUnderCursor(e *state.Editor) (string, bool) {
	return decl.Under(e.Buf.Line(e.Row), e.Col)
}

func definitionAbove(e *state.Editor, forms []*regexp.Regexp) (definition, bool) {
	at, ok := decl.Above(bufLines(e), forms, e.Row, e.Col)
	if !ok {
		return definition{}, false
	}

	return definition{path: e.SourceFile, row: at.Row, col: at.Col, rank: at.Rank}, true
}

func definitionInBuffer(e *state.Editor, forms []*regexp.Regexp) (definition, bool) {
	at, ok := decl.First(bufLines(e), forms, len(forms))

	// the first occurrence of the word is where the cursor already is often
	// enough, and a jump to it is no jump at all
	if !ok || (at.Row == e.Row && at.Rank == len(forms)-1) {
		return definition{}, false
	}

	return definition{path: e.SourceFile, row: at.Row, col: at.Col, rank: at.Rank}, true
}

// definitionInFiles looks through the files beside the one being edited and
// takes the strongest declaration any of them holds.
func definitionInFiles(e *state.Editor, forms []*regexp.Regexp) (definition, bool) {
	best := definition{rank: len(forms) - 1} // the plain occurrence is no reason to open a file
	for path, content := range project.Siblings(e.SourceFile) {
		if at, ok := decl.First(decl.Of(content), forms, best.rank); ok {
			best = definition{path: path, row: at.Row, col: at.Col, rank: at.Rank}
		}
	}

	if best.path == "" {
		return definition{}, false
	}

	return best, true
}

func bufLines(e *state.Editor) decl.Lines {
	return decl.Lines{Count: e.Buf.LineCount(), At: e.LineBytes}
}
