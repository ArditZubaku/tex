package lsp

import (
	"encoding/json"
	"strings"
	"testing"
)

// asking is the package with a server up and one document open, which is as far
// as every lookup below starts from.
func asking(t *testing.T, name, content string) (*harness, string) {
	t.Helper()

	h := reconciling(t)
	path := project(t, name, content)
	b := opening(t, path)
	h.up(File{Path: path, Buf: b})
	openedIn(t, h.sent())

	return h, path
}

func TestALookupSaysWhereTheCursorIsInTheServersOwnUnits(t *testing.T) {
	h, path := asking(t, "wide.go", "😀😀ab\n")
	b := opening(t, path)

	Definition(path, b, 0, 3, func([]Location, error) {})

	asked := h.sent()
	if asked.Method != methodDefinition {
		t.Fatalf("the server was asked %q, want %q", asked.Method, methodDefinition)
	}

	var params positionParams
	if err := json.Unmarshal(asked.Params, &params); err != nil {
		t.Fatal(err)
	}
	if params.Position.Character != 5 {
		t.Errorf("rune column 3 went out as character %d, want 5", params.Position.Character)
	}
	if params.TextDocument.URI != FileURI(path) {
		t.Errorf("the request named %q, want %q", params.TextDocument.URI, FileURI(path))
	}
}

func TestADefinitionAnsweredAsOneLocationIsStillAList(t *testing.T) {
	h, path := asking(t, "main.go", "package main\n")
	b := opening(t, path)

	var got []Location
	Definition(path, b, 0, 8, func(found []Location, _ error) { got = found })

	h.server.answer(h.sent().ID, Location{
		URI:   FileURI(path),
		Range: Range{Start: Position{Line: 4, Character: 5}},
	})
	h.poll()

	if len(got) != 1 {
		t.Fatalf("one location came back as %d of them", len(got))
	}
	if got[0].Range.Start.Line != 4 {
		t.Errorf("the location is on line %d, want 4", got[0].Range.Start.Line)
	}
}

func TestNothingFoundIsNoLocationsAndNoComplaint(t *testing.T) {
	h, path := asking(t, "main.go", "package main\n")
	b := opening(t, path)

	var got []Location
	failed := ErrStopped
	Definition(path, b, 0, 8, func(found []Location, err error) { got, failed = found, err })

	h.server.answer(h.sent().ID, nil)
	h.poll()

	if failed != nil {
		t.Fatalf("a server that found nothing reported %v", failed)
	}
	if got != nil {
		t.Errorf("nothing found came back as %v", got)
	}
}

func TestReferencesAsksForTheDeclarationAmongTheMentions(t *testing.T) {
	h, path := asking(t, "main.go", "package main\n")
	b := opening(t, path)

	References(path, b, 0, 8, func([]Location, error) {})

	asked := h.sent()
	if asked.Method != methodReferences {
		t.Fatalf("the server was asked %q, want %q", asked.Method, methodReferences)
	}

	var params referenceParams
	if err := json.Unmarshal(asked.Params, &params); err != nil {
		t.Fatal(err)
	}
	if !params.Context.IncludeDeclaration {
		t.Error("the declaration was left out of the references asked for")
	}
}

func TestReferencesComeBackInTheOrderTheServerSentThem(t *testing.T) {
	h, path := asking(t, "main.go", "package main\n")
	b := opening(t, path)

	var got []Location
	References(path, b, 0, 8, func(found []Location, _ error) { got = found })

	h.server.answer(h.sent().ID, []Location{
		{URI: FileURI(path), Range: Range{Start: Position{Line: 2}}},
		{URI: FileURI(path), Range: Range{Start: Position{Line: 9}}},
	})
	h.poll()

	if len(got) != 2 || got[0].Range.Start.Line != 2 || got[1].Range.Start.Line != 9 {
		t.Fatalf("the references came back as %v", got)
	}
}

func TestNestedSymbolsAreFlattenedIntoRowsInTheOrderTheyCame(t *testing.T) {
	h, path := asking(t, "main.go", "package main\n")

	var got []Symbol
	DocumentSymbols(path, func(found []Symbol, _ error) { got = found })

	asked := h.sent()
	if asked.Method != methodDocumentSymbol {
		t.Fatalf("the server was asked %q, want %q", asked.Method, methodDocumentSymbol)
	}

	h.server.answer(asked.ID, []Symbol{{
		Name: "Palette",
		Kind: 23,
		Children: []Symbol{
			{Name: "Name", Kind: 8},
			{Name: "Background", Kind: 8},
		},
	}, {Name: "Default", Kind: 12}})
	h.poll()

	var names []string
	for _, one := range got {
		names = append(names, one.Name)
		if one.Children != nil {
			t.Errorf("%s still carries its children", one.Name)
		}
	}
	if want := "Palette Name Background Default"; strings.Join(names, " ") != want {
		t.Errorf("the symbols came back as %q, want %q", strings.Join(names, " "), want)
	}
}

func TestASymbolNamesItsOwnFileOnlyWhenTheServerGaveItOne(t *testing.T) {
	flat := Symbol{Location: Location{
		URI:   FileURI("/x/theme.go"),
		Range: Range{Start: Position{Line: 3, Character: 5}},
	}}
	if flat.At().Line != 3 || flat.In() == "" {
		t.Errorf("a symbolInformation answered %v in %q", flat.At(), flat.In())
	}

	nested := Symbol{SelectionRange: Range{Start: Position{Line: 7, Character: 2}}}
	if nested.At().Line != 7 || nested.In() != "" {
		t.Errorf("a documentSymbol answered %v in %q", nested.At(), nested.In())
	}
}

func TestTheProjectsSymbolsAreAskedForWithAQueryTheServerCanFilterOn(t *testing.T) {
	h, path := asking(t, "main.go", "package main\n")

	WorkspaceSymbols(path, "Pal", func([]Symbol, error) {})

	asked := h.sent()
	if asked.Method != methodWorkspaceSymbol {
		t.Fatalf("the server was asked %q, want %q", asked.Method, methodWorkspaceSymbol)
	}

	var params symbolQuery
	if err := json.Unmarshal(asked.Params, &params); err != nil {
		t.Fatal(err)
	}
	if params.Query != "Pal" {
		t.Errorf("the query went out as %q, want %q", params.Query, "Pal")
	}
}

func TestALookupWithNoServerIsRefusedRatherThanLeftWaiting(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	path := project(t, "main.go", "package main\n")
	b := opening(t, path)

	answered := false
	Definition(path, b, 0, 8, func(found []Location, err error) {
		answered = true
		if found != nil || err == nil {
			t.Errorf("with no server the lookup answered %v, %v", found, err)
		}
	})

	if !answered {
		t.Fatal("the lookup was never answered at all")
	}
}

func TestAHoverIsReadOutOfWhicheverShapeTheServerSentIt(t *testing.T) {
	for _, one := range []struct {
		shape    string
		contents any
		want     string
	}{
		{"a marked-up block", map[string]any{"kind": "markdown", "value": "func Run()"}, "func Run()"},
		{"a bare string", "func Run()", "func Run()"},
		{"a list of blocks", []any{
			map[string]any{"language": "go", "value": "func Run()"},
			"Run is the editor.",
		}, "func Run()\n\nRun is the editor."},
	} {
		t.Run(one.shape, func(t *testing.T) {
			h, path := asking(t, "main.go", "package main\n")
			b := opening(t, path)

			var got string
			Hover(path, b, 0, 8, func(text string, _ error) { got = text })

			asked := h.sent()
			if asked.Method != methodHover {
				t.Fatalf("the server was asked %q, want %q", asked.Method, methodHover)
			}

			h.server.answer(asked.ID, map[string]any{"contents": one.contents})
			h.poll()

			if got != one.want {
				t.Errorf("the hover read as %q, want %q", got, one.want)
			}
		})
	}
}

func TestAServerWithNothingToSayAboutTheCursorAnswersEmpty(t *testing.T) {
	h, path := asking(t, "main.go", "package main\n")
	b := opening(t, path)

	got, failed := "unset", ErrStopped
	Hover(path, b, 0, 8, func(text string, err error) { got, failed = text, err })

	h.server.answer(h.sent().ID, nil)
	h.poll()

	if got != "" || failed != nil {
		t.Errorf("nothing to say came back as %q, %v", got, failed)
	}
}
