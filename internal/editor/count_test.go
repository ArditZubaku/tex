package editor

import (
	"testing"

	"github.com/ArditZubaku/tex/internal/editor/state"
)

func TestCountRepeatsAMotion(t *testing.T) {
	inReadMode(t, "a\nb\nc\nd\n", 0, 0)

	press(t, "3j")

	if ed.Row != 3 {
		t.Errorf("currentRow = %d, want 3", ed.Row)
	}
}

func TestCountedDeleteRune(t *testing.T) {
	b := inReadMode(t, "abcdef\n", 0, 0)

	press(t, "3x")

	wantLines(t, b, "def")
	if got := string(ed.Clip.Content()[0]); got != "abc" {
		t.Errorf("register holds %q, want %q", got, "abc")
	}
}

func TestCountIsSpentByTheCommandItPrefixes(t *testing.T) {
	inReadMode(t, "a\nb\nc\nd\ne\n", 0, 0)

	press(t, "2jj")

	if ed.Row != 3 {
		t.Errorf("currentRow = %d, want 3", ed.Row)
	}
	if ed.PendingCount != 0 {
		t.Errorf("pendingCount = %d, want it spent", ed.PendingCount)
	}
}

func TestMultiDigitCountIsCapped(t *testing.T) {
	inReadMode(t, "a\n", 0, 0)

	press(t, "999999")

	if ed.PendingCount != state.MaxCount {
		t.Errorf("pendingCount = %d, want %d", ed.PendingCount, state.MaxCount)
	}
}

func TestLeadingZeroDoesNotStartACount(t *testing.T) {
	inReadMode(t, "abc\n", 0, 0)

	press(t, "0")

	if ed.PendingCount != 0 {
		t.Errorf("pendingCount = %d, want 0", ed.PendingCount)
	}
}

func TestEscCancelsAPendingCount(t *testing.T) {
	inReadMode(t, "a\nb\nc\nd\n", 0, 0)

	press(t, "3")
	esc()
	press(t, "j")

	if ed.Row != 1 {
		t.Errorf("currentRow = %d, want 1", ed.Row)
	}
}
