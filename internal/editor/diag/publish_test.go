package diag_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ArditZubaku/tex/internal/editor/diag"
	"github.com/ArditZubaku/tex/internal/lsp"
)

// The columns a server counts are UTF-16 code units by default, and the editor
// counts runes: a line of plain ASCII cannot tell the two apart, and every
// other line can.
const mixed = "héllo ünïcödé 😀 x"

func at(line, character int) lsp.Position {
	return lsp.Position{Line: line, Character: character}
}

func TestColumnsAreCountedTheWayTheServerCountsThem(t *testing.T) {
	path := fresh(t)
	if err := os.WriteFile(path, []byte(mixed+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	// units 16 through 18 are the space after the emoji and the 'x' past it,
	// which are rune columns 15 and 16
	diag.Publish(path, []lsp.Diagnostic{{
		Range:    lsp.Range{Start: at(0, 16), End: at(0, 18)},
		Severity: 1,
		Message:  "past the emoji",
	}})

	notes := diag.Of(path)
	if len(notes) != 1 {
		t.Fatalf("published %v", notes)
	}
	if notes[0].Col != 15 || notes[0].EndCol != 17 {
		t.Errorf("range is columns %d to %d, want 15 to 17", notes[0].Col, notes[0].EndCol)
	}
}

func TestAServerThatDidNotSayHowBadItIsMeansItToBeSeen(t *testing.T) {
	path := fresh(t)
	if err := os.WriteFile(path, []byte("package main\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	diag.Publish(path, []lsp.Diagnostic{{
		Range:   lsp.Range{Start: at(0, 0), End: at(0, 7)},
		Message: "no severity at all",
	}})

	if got := diag.Of(path)[0].Severity; got != diag.Error {
		t.Errorf("severity %v, want %v", got, diag.Error)
	}
}

func TestAFileThatIsNotThereIsPublishedAsNothing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gone.go")
	diag.Reset()
	lsp.Reset()
	t.Cleanup(func() {
		diag.Reset()
		lsp.Reset()
	})

	diag.Publish(path, []lsp.Diagnostic{{
		Range:   lsp.Range{Start: at(0, 0), End: at(0, 4)},
		Message: "about a file that has gone",
	}})

	if got := diag.Of(path); len(got) != 0 {
		t.Errorf("a file that is not there has %v", got)
	}
}

func TestAnEmptyPublishClearsWhatTheFileHad(t *testing.T) {
	path := fresh(t)
	diag.Set(path, diag.File{{Severity: diag.Error, Row: 1, EndRow: 1, EndCol: 2}})

	diag.Publish(path, nil)

	if got := diag.Of(path); len(got) != 0 {
		t.Errorf("the file still has %v", got)
	}
}

// A server naming a line the file no longer has is one working from text it was
// told about before the last change landed.
func TestALineTheFileNoLongerHasIsClampedOntoIt(t *testing.T) {
	path := fresh(t)
	if err := os.WriteFile(path, []byte("package main\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	diag.Publish(path, []lsp.Diagnostic{{
		Range:    lsp.Range{Start: at(400, 0), End: at(400, 4)},
		Severity: 1,
		Message:  "off the end",
	}})

	if got := diag.Of(path)[0].Row; got != 0 {
		t.Errorf("landed on row %d, want the last row the file has", got)
	}
}
