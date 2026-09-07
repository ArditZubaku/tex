package diag_test

import (
	"path/filepath"
	"testing"

	"github.com/ArditZubaku/tex/internal/editor/diag"
	"github.com/ArditZubaku/tex/internal/lsp"
)

func fresh(t *testing.T) string {
	t.Helper()

	diag.Reset()
	lsp.Reset()
	t.Cleanup(func() {
		diag.Reset()
		lsp.Reset()
	})

	return filepath.Join(t.TempDir(), "main.go")
}

func TestAFileNothingWasSaidAboutHasNothingOnIt(t *testing.T) {
	path := fresh(t)

	if got := diag.Of(path); len(got) != 0 {
		t.Fatalf("a file nothing was said about has %v", got)
	}
	if got := diag.Of(path).Row(0).Worst(); got != diag.None {
		t.Errorf("its first row is marked %v", got)
	}
	if got := diag.Of(path).Row(0).Under(0); got != diag.None {
		t.Errorf("its first column is marked %v", got)
	}
}

func TestTheSameFileNamedTwoWaysIsOneEntry(t *testing.T) {
	path := fresh(t)
	diag.Set(path, diag.File{{Severity: diag.Error, Row: 2, Col: 0, EndRow: 2, EndCol: 4}})

	t.Chdir(filepath.Dir(path))

	if got := len(diag.Of("main.go")); got != 1 {
		t.Fatalf("the file named relative has %d notes, want the 1 named absolute", got)
	}
}

func TestAnEmptyPublishIsWhatTakesAnUnderlineOff(t *testing.T) {
	path := fresh(t)
	diag.Set(path, diag.File{{Severity: diag.Error, Row: 1, EndRow: 1, EndCol: 3}})

	diag.Set(path, nil)

	if got := diag.Of(path).Row(1).Worst(); got != diag.None {
		t.Errorf("the row is still marked %v after an empty publish", got)
	}
}

func TestARangeMarksEveryColumnItCoversAndNoOther(t *testing.T) {
	path := fresh(t)
	diag.Set(path, diag.File{{Severity: diag.Warning, Row: 3, Col: 4, EndRow: 3, EndCol: 7}})

	row := diag.Of(path).Row(3)
	for col, want := range map[int]diag.Severity{
		3: diag.None,
		4: diag.Warning,
		6: diag.Warning,
		7: diag.None,
	} {
		if got := row.Under(col); got != want {
			t.Errorf("column %d is marked %v, want %v", col, got, want)
		}
	}
	if got := diag.Of(path).Row(2).Worst(); got != diag.None {
		t.Errorf("the row above it is marked %v", got)
	}
}

func TestARangeOfNoWidthTakesTheRuneItPointsAt(t *testing.T) {
	path := fresh(t)
	diag.Set(path, diag.File{{Severity: diag.Error, Row: 0, Col: 5, EndRow: 0, EndCol: 5}})

	row := diag.Of(path).Row(0)
	if got := row.Under(5); got != diag.Error {
		t.Errorf("the rune it points at is marked %v, want %v", got, diag.Error)
	}
	if got := row.Under(4); got != diag.None {
		t.Errorf("the rune before it is marked %v", got)
	}
	if got := row.Under(6); got != diag.None {
		t.Errorf("the rune after it is marked %v", got)
	}
}

func TestARangeOverSeveralRowsTakesItsOwnColumnsAtEitherEnd(t *testing.T) {
	path := fresh(t)
	diag.Set(path, diag.File{{Severity: diag.Error, Row: 1, Col: 6, EndRow: 3, EndCol: 2}})

	file := diag.Of(path)
	if got := file.Row(1).Under(5); got != diag.None {
		t.Errorf("before the start column of the first row is marked %v", got)
	}
	if got := file.Row(1).Under(6); got != diag.Error {
		t.Errorf("the start column is marked %v, want %v", got, diag.Error)
	}
	if got := file.Row(1).Under(400); got != diag.Error {
		t.Errorf("the rest of the first row is marked %v, want %v", got, diag.Error)
	}
	if got := file.Row(2).Under(0); got != diag.Error {
		t.Errorf("a whole row in the middle is marked %v, want %v", got, diag.Error)
	}
	if got := file.Row(3).Under(1); got != diag.Error {
		t.Errorf("up to the end column of the last row is marked %v, want %v", got, diag.Error)
	}
	if got := file.Row(3).Under(2); got != diag.None {
		t.Errorf("the end column itself is marked %v, want it exclusive", got)
	}
}

// A range that ends at the start of a row ends where that row begins, so none
// of it is on it — which is how a server says "through to the end of the line
// above".
func TestARangeEndingAtTheStartOfARowDoesNotMarkThatRow(t *testing.T) {
	path := fresh(t)
	diag.Set(path, diag.File{{Severity: diag.Error, Row: 4, Col: 0, EndRow: 5, EndCol: 0}})

	file := diag.Of(path)
	if got := file.Row(4).Under(0); got != diag.Error {
		t.Errorf("the row it covers is marked %v, want %v", got, diag.Error)
	}
	if got := file.Row(5).Worst(); got != diag.None {
		t.Errorf("the row it ends at is marked %v", got)
	}
}

func TestARangeTheServerCouldNotPlaceOnlyMarksItsFirstRows(t *testing.T) {
	path := fresh(t)
	diag.Set(path, diag.File{{Severity: diag.Error, Row: 0, EndRow: 900, EndCol: 1}})

	file := diag.Of(path)
	if got := file.Row(7).Worst(); got != diag.Error {
		t.Errorf("row 7 is marked %v, want %v", got, diag.Error)
	}
	if got := file.Row(8).Worst(); got != diag.None {
		t.Errorf("row 8 is marked %v, want nothing", got)
	}
}

func TestTheWorstOfTwoThingsSaidAboutOneColumnIsWhatItIsMarked(t *testing.T) {
	path := fresh(t)
	diag.Set(path, diag.File{
		{Severity: diag.Hint, Row: 0, Col: 0, EndRow: 0, EndCol: 10},
		{Severity: diag.Error, Row: 0, Col: 3, EndRow: 0, EndCol: 5},
	})

	row := diag.Of(path).Row(0)
	if got := row.Under(4); got != diag.Error {
		t.Errorf("a column both cover is marked %v, want %v", got, diag.Error)
	}
	if got := row.Under(8); got != diag.Hint {
		t.Errorf("a column only the hint covers is marked %v, want %v", got, diag.Hint)
	}
	if got := row.Worst(); got != diag.Error {
		t.Errorf("the row is marked %v, want %v", got, diag.Error)
	}
}

func TestOnlyErrorsAndWarningsAreCounted(t *testing.T) {
	path := fresh(t)
	diag.Set(path, diag.File{
		{Severity: diag.Error, Row: 0},
		{Severity: diag.Error, Row: 1},
		{Severity: diag.Warning, Row: 2},
		{Severity: diag.Info, Row: 3},
		{Severity: diag.Hint, Row: 4},
	})

	errors, warnings := diag.Count(path)
	if errors != 2 || warnings != 1 {
		t.Errorf("counted %d errors and %d warnings, want 2 and 1", errors, warnings)
	}
}

func TestWhatWasSaidAboutAFileIsHeldInRowOrder(t *testing.T) {
	path := fresh(t)
	diag.Set(path, diag.File{
		{Severity: diag.Error, Row: 9, Col: 1},
		{Severity: diag.Error, Row: 2, Col: 8},
		{Severity: diag.Error, Row: 2, Col: 3},
	})

	want := [][2]int{{2, 3}, {2, 8}, {9, 1}}
	for i, note := range diag.Of(path) {
		if [2]int{note.Row, note.Col} != want[i] {
			t.Fatalf("note %d is at %d,%d, want %v", i, note.Row, note.Col, want[i])
		}
	}
}
