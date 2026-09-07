package screen

import (
	"strings"
	"testing"
)

func TestWrapBreaksOnSpaces(t *testing.T) {
	got := Wrap("a.go:2:6: expected declaration", 12)

	want := []string{"a.go:2:6:", "expected", "declaration"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("Wrap = %q, want %q", got, want)
	}
}

func TestWrapBreaksAWordTooLongForTheBox(t *testing.T) {
	got := Wrap("aa bbbbbbbb", 4)

	want := []string{"aa", "bbbb", "bbbb"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("Wrap = %q, want %q", got, want)
	}
}

func TestWrapKeepsWhatFitsOnOneLine(t *testing.T) {
	if got := Wrap("expected declaration", 40); len(got) != 1 || got[0] != "expected declaration" {
		t.Errorf("Wrap = %q, want it left whole", got)
	}
}
