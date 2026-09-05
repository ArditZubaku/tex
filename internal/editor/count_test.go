package editor

import "testing"

func TestCountRepeatsAMotion(t *testing.T) {
	inReadMode(t, "a\nb\nc\nd\n", 0, 0)

	press(t, "3j")

	if currentRow != 3 {
		t.Errorf("currentRow = %d, want 3", currentRow)
	}
}

func TestCountedDeleteRune(t *testing.T) {
	b := inReadMode(t, "abcdef\n", 0, 0)

	press(t, "3x")

	wantLines(t, b, "def")
	if got := string(clipboard.lines[0]); got != "abc" {
		t.Errorf("register holds %q, want %q", got, "abc")
	}
}

func TestCountIsSpentByTheCommandItPrefixes(t *testing.T) {
	inReadMode(t, "a\nb\nc\nd\ne\n", 0, 0)

	press(t, "2jj")

	if currentRow != 3 {
		t.Errorf("currentRow = %d, want 3", currentRow)
	}
	if pendingCount != 0 {
		t.Errorf("pendingCount = %d, want it spent", pendingCount)
	}
}

func TestMultiDigitCountIsCapped(t *testing.T) {
	inReadMode(t, "a\n", 0, 0)

	press(t, "999999")

	if pendingCount != maxCount {
		t.Errorf("pendingCount = %d, want %d", pendingCount, maxCount)
	}
}

func TestLeadingZeroDoesNotStartACount(t *testing.T) {
	inReadMode(t, "abc\n", 0, 0)

	press(t, "0")

	if pendingCount != 0 {
		t.Errorf("pendingCount = %d, want 0", pendingCount)
	}
}

func TestEscCancelsAPendingCount(t *testing.T) {
	inReadMode(t, "a\nb\nc\nd\n", 0, 0)

	press(t, "3")
	esc()
	press(t, "j")

	if currentRow != 1 {
		t.Errorf("currentRow = %d, want 1", currentRow)
	}
}
