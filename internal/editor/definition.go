package editor

import (
	"regexp"

	"github.com/ArditZubaku/tex/internal/decl"
	"github.com/ArditZubaku/tex/internal/project"
)

// 'gd' is VIM's jump to a declaration, read from the text rather than from a
// language server: the identifier under the cursor is looked for in the shapes
// a declaration takes, strongest first, nearest above the cursor first of all.

type definition struct {
	path string
	row  int
	col  int
	rank int
}

func goToDefinition() {
	word, ok := wordUnderCursor()
	if !ok {
		ed.StatusMsg = "E349: No identifier under the cursor"
		return
	}

	forms, err := decl.Forms(word)
	if err != nil {
		ed.StatusMsg = "E486: Pattern not found: " + word
		return
	}

	if found, ok := definitionAbove(forms); ok {
		jumpTo(found)
		return
	}

	if found, ok := definitionInBuffer(forms); ok {
		jumpTo(found)
		return
	}

	found, ok := definitionInFiles(forms)
	if !ok {
		ed.StatusMsg = "E388: Couldn't find definition of " + word
		return
	}

	jumpTo(found)
}

func jumpTo(found definition) {
	ed.PushJump()
	if found.path != ed.SourceFile {
		openInBuffer(found.path)
	}
	ed.Row, ed.Col = found.row, found.col
	ed.ClampCol()
	ed.CenterIfOffScreen()
}

// jumpBack is Ctrl-O: the cursor goes back to where the last jump left from,
// switching buffers when the jump crossed files.
func jumpBack() {
	back, ok := ed.PopJump()
	if !ok {
		ed.StatusMsg = "E664: Jump list is empty"
		return
	}

	if back.Path != ed.SourceFile {
		openInBuffer(back.Path)
	}
	ed.Row, ed.Col = min(back.Row, ed.Buf.LineCount()-1), back.Col
	ed.ClampCol()
	ed.CenterIfOffScreen()
}

func wordUnderCursor() (string, bool) {
	return decl.Under(ed.Buf.Line(ed.Row), ed.Col)
}

func definitionAbove(forms []*regexp.Regexp) (definition, bool) {
	at, ok := decl.Above(bufLines(), forms, ed.Row, ed.Col)
	if !ok {
		return definition{}, false
	}

	return definition{path: ed.SourceFile, row: at.Row, col: at.Col, rank: at.Rank}, true
}

func definitionInBuffer(forms []*regexp.Regexp) (definition, bool) {
	at, ok := decl.First(bufLines(), forms, len(forms))

	// the first occurrence of the word is where the cursor already is often
	// enough, and a jump to it is no jump at all
	if !ok || (at.Row == ed.Row && at.Rank == len(forms)-1) {
		return definition{}, false
	}

	return definition{path: ed.SourceFile, row: at.Row, col: at.Col, rank: at.Rank}, true
}

// definitionInFiles looks through the files beside the one being edited and
// takes the strongest declaration any of them holds.
func definitionInFiles(forms []*regexp.Regexp) (definition, bool) {
	best := definition{rank: len(forms) - 1} // the plain occurrence is no reason to open a file
	for path, content := range project.Siblings(ed.SourceFile) {
		if at, ok := decl.First(decl.Of(content), forms, best.rank); ok {
			best = definition{path: path, row: at.Row, col: at.Col, rank: at.Rank}
		}
	}

	if best.path == "" {
		return definition{}, false
	}

	return best, true
}

func bufLines() decl.Lines {
	return decl.Lines{Count: ed.Buf.LineCount(), At: ed.LineBytes}
}
