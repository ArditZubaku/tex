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

// jumps is what Ctrl-O steps back through: where the cursor was before a jump
// took it somewhere else, the file it was in included.
var jumps []definition

func goToDefinition() {
	word, ok := wordUnderCursor()
	if !ok {
		statusMsg = "E349: No identifier under the cursor"
		return
	}

	forms, err := decl.Forms(word)
	if err != nil {
		statusMsg = "E486: Pattern not found: " + word
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
		statusMsg = "E388: Couldn't find definition of " + word
		return
	}

	jumpTo(found)
}

func jumpTo(found definition) {
	pushJump()
	if found.path != sourceFile {
		openInBuffer(found.path)
	}
	currentRow, currentCol = found.row, found.col
	clampCol()
	centerIfOffScreen()
}

// jumpBack is Ctrl-O: the cursor goes back to where the last jump left from,
// switching buffers when the jump crossed files.
func jumpBack() {
	if len(jumps) == 0 {
		statusMsg = "E664: Jump list is empty"
		return
	}

	back := jumps[len(jumps)-1]
	jumps = jumps[:len(jumps)-1]

	if back.path != sourceFile {
		openInBuffer(back.path)
	}
	currentRow, currentCol = min(back.row, buf.LineCount()-1), back.col
	clampCol()
	centerIfOffScreen()
}

func pushJump() {
	jumps = append(jumps, definition{path: sourceFile, row: currentRow, col: currentCol})
}

func centerIfOffScreen() {
	if currentRow < offsetRow || currentRow >= offsetRow+ROWS {
		centerView()
	}
}

func wordUnderCursor() (string, bool) {
	return decl.Under(buf.Line(currentRow), currentCol)
}

func definitionAbove(forms []*regexp.Regexp) (definition, bool) {
	at, ok := decl.Above(bufLines(), forms, currentRow, currentCol)
	if !ok {
		return definition{}, false
	}

	return definition{path: sourceFile, row: at.Row, col: at.Col, rank: at.Rank}, true
}

func definitionInBuffer(forms []*regexp.Regexp) (definition, bool) {
	at, ok := decl.First(bufLines(), forms, len(forms))

	// the first occurrence of the word is where the cursor already is often
	// enough, and a jump to it is no jump at all
	if !ok || (at.Row == currentRow && at.Rank == len(forms)-1) {
		return definition{}, false
	}

	return definition{path: sourceFile, row: at.Row, col: at.Col, rank: at.Rank}, true
}

// definitionInFiles looks through the files beside the one being edited and
// takes the strongest declaration any of them holds.
func definitionInFiles(forms []*regexp.Regexp) (definition, bool) {
	best := definition{rank: len(forms) - 1} // the plain occurrence is no reason to open a file
	for path, content := range project.Siblings(sourceFile) {
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
	return decl.Lines{Count: buf.LineCount(), At: lineBytes}
}

// lineBytes is the line as it is held: the raw bytes of one still on disk, and
// the runes of one the overlay has edited.
func lineBytes(row int) []byte {
	if line, ok := buf.EditedLine(row); ok {
		return []byte(string(line))
	}

	return buf.Raw(row)
}
