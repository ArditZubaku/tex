package edit_test

import (
	"testing"

	"github.com/ArditZubaku/tex/internal/editor/complete"
	"github.com/ArditZubaku/tex/internal/editor/edit"
	"github.com/ArditZubaku/tex/internal/editor/edtest"
	"github.com/ArditZubaku/tex/internal/editor/state"
)

func TestACandidateReplacesWhatHasBeenTypedOfTheWord(t *testing.T) {
	e := state.New()
	b := edtest.AtCursor(t, e, "\tfmt.Prin\n", 0, 9)
	e.Mode = state.EditMode

	edit.Accept(e, complete.Item{Label: "Println", Text: "Println", From: 5})

	edtest.WantLines(t, b, "\tfmt.Println")
	edtest.WantCursor(t, e, 0, 12)
}

func TestACandidateGoesInWhereThereIsNothingTypedYet(t *testing.T) {
	e := state.New()
	b := edtest.AtCursor(t, e, "\tfmt.\n", 0, 5)
	e.Mode = state.EditMode

	edit.Accept(e, complete.Item{Label: "Println", Text: "Println", From: 5})

	edtest.WantLines(t, b, "\tfmt.Println")
	edtest.WantCursor(t, e, 0, 12)
}

func TestTheRestOfTheLineIsLeftAloneAroundACandidate(t *testing.T) {
	e := state.New()
	b := edtest.AtCursor(t, e, "\tx := fmt.Prin(a, b)\n", 0, 14)
	e.Mode = state.EditMode

	edit.Accept(e, complete.Item{Label: "Println", Text: "Println", From: 10})

	edtest.WantLines(t, b, "\tx := fmt.Println(a, b)")
}

func TestACandidateThatIsCalledBringsItsParenthesesWithIt(t *testing.T) {
	e := state.New()
	b := edtest.AtCursor(t, e, "\tfmt.Prin\n", 0, 9)
	e.Mode = state.EditMode

	edit.Accept(e, complete.Item{Label: "Println", Text: "Println", From: 5, Call: true})

	edtest.WantLines(t, b, "\tfmt.Println()")
	edtest.WantCursor(t, e, 0, 13)
}

func TestACandidateThatIsNamedRatherThanCalledDoesNot(t *testing.T) {
	e := state.New()
	b := edtest.AtCursor(t, e, "\tx := hand\n", 0, 10)
	e.Mode = state.EditMode

	edit.Accept(e, complete.Item{Label: "handler", Text: "handler", From: 6})

	edtest.WantLines(t, b, "\tx := handler")
	edtest.WantCursor(t, e, 0, 13)
}

func TestACallAlreadyWrittenIsNotGivenASecondPair(t *testing.T) {
	e := state.New()
	b := edtest.AtCursor(t, e, "\tx := fmt.Prin(a, b)\n", 0, 14)
	e.Mode = state.EditMode

	edit.Accept(e, complete.Item{Label: "Println", Text: "Println", From: 10, Call: true})

	edtest.WantLines(t, b, "\tx := fmt.Println(a, b)")
	edtest.WantCursor(t, e, 0, 17)
}

func TestAServerThatSentTheParenthesesItselfIsLeftAlone(t *testing.T) {
	e := state.New()
	b := edtest.AtCursor(t, e, "\tPrin\n", 0, 5)
	e.Mode = state.EditMode

	edit.Accept(e, complete.Item{Label: "Println", Text: "Println()", From: 1, Call: true})

	edtest.WantLines(t, b, "\tPrintln()")
}

// The edits a server sends alongside a candidate are the import the name it
// just wrote needs; without them the completion is a compile error.
func TestTheImportACandidateNeedsGoesInWithIt(t *testing.T) {
	e := state.New()
	b := edtest.AtCursor(t, e, "package main\n\nfunc main() {\n\tPrin\n}\n", 3, 5)
	e.Mode = state.EditMode

	edit.Accept(e, complete.Item{
		Label: "fmt.Println",
		Text:  "fmt.Println",
		From:  1,
		Extra: []complete.Edit{{
			Row: 1, Col: 0, EndRow: 1, EndCol: 0,
			Text: "\nimport \"fmt\"\n",
		}},
	})

	edtest.WantLines(t, b,
		"package main", "", "import \"fmt\"", "", "func main() {", "\tfmt.Println", "}")
}

// An import going in above the cursor moves the line being typed on down with
// it, and the cursor has to follow or the next keystroke lands elsewhere.
func TestTheCursorFollowsTheLineAnImportPushedDown(t *testing.T) {
	e := state.New()
	edtest.AtCursor(t, e, "package main\n\nfunc main() {\n\tPrin\n}\n", 3, 5)
	e.Mode = state.EditMode

	edit.Accept(e, complete.Item{
		Label: "fmt.Println",
		Text:  "fmt.Println",
		From:  1,
		Extra: []complete.Edit{{Row: 1, EndRow: 1, Text: "\nimport \"fmt\"\n"}},
	})

	edtest.WantCursor(t, e, 5, 12)
}

func TestAnEditBelowTheCursorLeavesTheCursorWhereItIs(t *testing.T) {
	e := state.New()
	b := edtest.AtCursor(t, e, "\tPrin\n\n// tail\n", 0, 5)
	e.Mode = state.EditMode

	edit.Accept(e, complete.Item{
		Label: "Println",
		Text:  "Println",
		From:  1,
		Extra: []complete.Edit{{Row: 2, Col: 0, EndRow: 2, EndCol: 7, Text: "// done"}},
	})

	edtest.WantLines(t, b, "\tPrintln", "", "// done")
	edtest.WantCursor(t, e, 0, 8)
}

// An edit spanning lines is what replacing a whole import block looks like, and
// it has to leave what was either side of it in place.
func TestAnEditAcrossLinesJoinsWhatWasEitherSideOfIt(t *testing.T) {
	e := state.New()
	b := edtest.AtCursor(t, e, "import (\n\t\"os\"\n)\n\tPrin\n", 3, 5)
	e.Mode = state.EditMode

	edit.Accept(e, complete.Item{
		Label: "Println",
		Text:  "Println",
		From:  1,
		Extra: []complete.Edit{{Row: 0, Col: 8, EndRow: 2, EndCol: 0, Text: "\n\t\"fmt\"\n\t\"os\"\n"}},
	})

	edtest.WantLines(t, b, "import (", "\t\"fmt\"", "\t\"os\"", ")", "\tPrintln")
	edtest.WantCursor(t, e, 4, 8)
}

func TestAcceptingACandidateIsOneUndo(t *testing.T) {
	e := state.New()
	b := edtest.InReadMode(t, e, "\tfmt.Prin\n", 0, 9)
	e.Mode = state.EditMode

	e.BeginChange()
	edit.Accept(e, complete.Item{Label: "Println", Text: "Println", From: 5})
	e.Mode = state.ReadMode
	e.EndChange()
	e.Undo()

	edtest.WantLines(t, b, "\tfmt.Prin")
}

// The two edits gopls sends to import a package into a file that has a single
// unparenthesised import: one opens the block around what is there, the other
// adds the new line and closes it. Both land above the word being completed,
// and between them they move it four lines down.
func TestAnImportThatOpensABlockAroundTheOneAlreadyThere(t *testing.T) {
	e := state.New()
	b := edtest.AtCursor(t, e,
		"package main\n\nimport \"os\"\n\nfunc probe() {\n\tedit\n\t_ = os.Args\n}\n", 5, 5)
	e.Mode = state.EditMode

	edit.Accept(e, complete.Item{
		Label: "editor",
		Text:  "editor",
		From:  1,
		Extra: []complete.Edit{
			{Row: 2, Col: 7, EndRow: 2, EndCol: 7, Text: "(\n\t"},
			{
				Row: 3, Col: 0, EndRow: 3, EndCol: 0,
				Text: "\n\t\"github.com/ArditZubaku/tex/internal/editor\"\n)\n",
			},
		},
	})

	edtest.WantLines(t, b,
		"package main",
		"",
		"import (",
		"\t\"os\"",
		"",
		"\t\"github.com/ArditZubaku/tex/internal/editor\"",
		")",
		"",
		"func probe() {",
		"\teditor",
		"\t_ = os.Args",
		"}")
	edtest.WantCursor(t, e, 9, 7)
}

// The import typescript-language-server sends back for a candidate in a
// one-line file goes in at the very start of the line being typed on, which is
// the cursor's own row: it has to apply, and it has to carry the cursor with it.
func TestAnImportOnTheCursorsOwnRowCarriesTheCursorDown(t *testing.T) {
	e := state.New()
	b := edtest.AtCursor(t, e, "const x = computeTotal\n", 0, 22)
	e.Mode = state.EditMode

	edit.Elsewhere(e, []complete.Edit{{
		Row: 0, Col: 0, EndRow: 0, EndCol: 0,
		Text: "import { computeTotal } from \"./helper\";\n\n",
	}})

	edtest.WantLines(t, b, "import { computeTotal } from \"./helper\";", "", "const x = computeTotal")
	edtest.WantCursor(t, e, 2, 22)
}

func TestAnEditInFrontOfTheCursorOnItsOwnLineMovesItAlong(t *testing.T) {
	e := state.New()
	b := edtest.AtCursor(t, e, "let total\n", 0, 9)
	e.Mode = state.EditMode

	edit.Elsewhere(e, []complete.Edit{{Row: 0, Col: 0, EndRow: 0, EndCol: 3, Text: "const"}})

	edtest.WantLines(t, b, "const total")
	edtest.WantCursor(t, e, 0, 11)
}

func TestAnEditBehindTheCursorOnItsOwnLineLeavesTheColumnAlone(t *testing.T) {
	e := state.New()
	b := edtest.AtCursor(t, e, "let total = x\n", 0, 4)
	e.Mode = state.EditMode

	edit.Elsewhere(e, []complete.Edit{{Row: 0, Col: 12, EndRow: 0, EndCol: 13, Text: "yy"}})

	edtest.WantLines(t, b, "let total = yy")
	edtest.WantCursor(t, e, 0, 4)
}

// Two answers landing on top of each other is worse than one being dropped.
func TestAnEditStraddlingTheCursorIsDroppedRatherThanApplied(t *testing.T) {
	e := state.New()
	b := edtest.AtCursor(t, e, "const x = compute\n", 0, 13)
	e.Mode = state.EditMode

	edit.Elsewhere(e, []complete.Edit{{Row: 0, Col: 10, EndRow: 0, EndCol: 17, Text: "nonsense"}})

	edtest.WantLines(t, b, "const x = compute")
	edtest.WantCursor(t, e, 0, 13)
}
