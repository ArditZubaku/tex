package editor

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/ArditZubaku/txi/internal/chars"
)

// 'gd' is VIM's jump to a declaration, read from the text rather than from a
// language server: the identifier under the cursor is looked for in the shapes
// a declaration takes across the languages the editor highlights, strongest
// first, and the plain first occurrence is what VIM's own 'gd' settles for.
var definitionForms = []string{
	`(?:func|fn|def|function|proc|sub)\s+(?:\([^)]*\)\s*)?(%s)\b`,
	`(?:type|class|struct|interface|enum|trait|record)\s+(%s)\b`,
	`(?:func|fn|def|function|proc|sub)\b[^(]*\((?:[^)]*[^\w.])?(%s)\b[^)]*\)`,
	`(?:var|let|const|val|static)\s+(%s)\b`,
	`(?:^|[^\w.])(%s)\s*(?::=|=[^=])`,
	`(?:^|[^\w.])(%s)\b`,
}

// A definition is worth looking for in the files beside the one being edited,
// but not at the cost of reading a tree of them whole: anything larger than
// this is left to a real index.
const maxDefinitionFileSize = 1 << 20

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

	forms, err := compileForms(word)
	if err != nil {
		statusMsg = "E486: Pattern not found: " + word
		return
	}

	if found, ok := definitionAbove(forms); ok {
		pushJump()
		currentRow, currentCol = found.row, found.col
		clampCol()
		centerIfOffScreen()
		return
	}

	if found, ok := definitionInBuffer(forms); ok {
		pushJump()
		currentRow, currentCol = found.row, found.col
		clampCol()
		centerIfOffScreen()
		return
	}

	found, ok := definitionInFiles(forms)
	if !ok {
		statusMsg = "E388: Couldn't find definition of " + word
		return
	}

	pushJump()
	openInBuffer(found.path)
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
	line := buf.Line(currentRow)
	if currentCol >= len(line) || chars.ClassOf(line[currentCol]) != chars.Word {
		return "", false
	}

	from := currentCol
	for from > 0 && chars.ClassOf(line[from-1]) == chars.Word {
		from--
	}
	to := currentCol
	for to < len(line) && chars.ClassOf(line[to]) == chars.Word {
		to++
	}

	return string(line[from:to]), true
}

func compileForms(word string) ([]*regexp.Regexp, error) {
	forms := make([]*regexp.Regexp, 0, len(definitionForms))
	for _, form := range definitionForms {
		compiled, err := regexp.Compile(fmt.Sprintf(form, regexp.QuoteMeta(word)))
		if err != nil {
			return nil, err
		}
		forms = append(forms, compiled)
	}

	return forms, nil
}

// definitionAbove is what VIM's 'gd' is for: the declaration nearest above the
// cursor and inside the same top-level construct, which is the one the
// identifier under the cursor is bound to — the parameter it was passed as, or
// the local it was assigned from, rather than a field of the same name three
// hundred lines away.
func definitionAbove(forms []*regexp.Regexp) (definition, bool) {
	for row := currentRow; row >= blockStart(currentRow); row-- {
		line := lineBytes(row)
		for rank := range len(forms) - 1 { // a mention on its own declares nothing
			at := forms[rank].FindSubmatchIndex(line)
			if at == nil {
				continue
			}

			col := utf8.RuneCount(line[:at[2]])
			if row == currentRow && col == currentCol {
				break // the cursor is on it: whatever this is, it is not a jump
			}

			return definition{path: sourceFile, row: row, col: col, rank: rank}, true
		}
	}

	return definition{}, false
}

// blockStart is where the top-level construct the row sits in begins: the
// nearest line above it that starts in the first column, which is where every
// language the editor highlights puts one.
func blockStart(row int) int {
	for at := row - 1; at > 0; at-- {
		line := lineBytes(at)
		if len(line) > 0 && line[0] != ' ' && line[0] != '\t' {
			return at
		}
	}

	return 0
}

// definitionInBuffer takes the strongest form any line matches, and the first
// line of that form: a declaration is what is wanted, and the file's own order
// breaks the tie between two of them.
func definitionInBuffer(forms []*regexp.Regexp) (definition, bool) {
	best := definition{rank: len(forms)}
	for row := range buf.LineCount() {
		line := lineBytes(row)
		for rank, form := range forms {
			if rank >= best.rank {
				break
			}
			if at := form.FindSubmatchIndex(line); at != nil {
				best = definition{path: sourceFile, row: row, col: utf8.RuneCount(line[:at[2]]), rank: rank}
				break
			}
		}
	}

	// the first occurrence of the word is where the cursor already is often
	// enough, and a jump to it is no jump at all
	if best.rank == len(forms) || (best.row == currentRow && best.rank == len(forms)-1) {
		return definition{}, false
	}

	return best, true
}

// definitionInFiles looks through the files beside the one being edited, of the
// same kind, and takes the strongest declaration any of them holds.
func definitionInFiles(forms []*regexp.Regexp) (definition, bool) {
	root := projectRoot()
	files, err := listFiles(root)
	if err != nil {
		return definition{}, false
	}

	ext := filepath.Ext(sourceFile)
	best := definition{rank: len(forms) - 1} // the plain occurrence is no reason to open a file
	for _, rel := range files {
		path := filepath.Join(root, rel)
		if filepath.Ext(path) != ext || sameFile(path, sourceFile) {
			continue
		}

		if found, ok := definitionInFile(path, forms, best.rank); ok {
			best = found
		}
	}

	if best.path == "" {
		return definition{}, false
	}

	return best, true
}

func definitionInFile(path string, forms []*regexp.Regexp, better int) (definition, bool) {
	info, err := os.Stat(path)
	if err != nil || info.Size() > maxDefinitionFileSize {
		return definition{}, false
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return definition{}, false
	}

	found := definition{}
	for row, line := range strings.Split(string(content), "\n") {
		for rank := range better {
			if at := forms[rank].FindStringSubmatchIndex(line); at != nil {
				found = definition{path: path, row: row, col: utf8.RuneCountInString(line[:at[2]]), rank: rank}
				better = rank
				break
			}
		}
		if better == 0 {
			break
		}
	}

	return found, found.path != ""
}

func sameFile(a, b string) bool {
	left, err := filepath.Abs(a)
	if err != nil {
		return false
	}
	right, err := filepath.Abs(b)
	if err != nil {
		return false
	}

	return left == right
}

// lineBytes is the line as it is held: the raw bytes of one still on disk, and
// the runes of one the overlay has edited.
func lineBytes(row int) []byte {
	if line, ok := buf.EditedLine(row); ok {
		return []byte(string(line))
	}

	return buf.Raw(row)
}
