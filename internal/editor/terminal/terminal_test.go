package terminal

import (
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/hinshun/vt10x"
	"github.com/nsf/termbox-go"
)

func waitUntil(t *testing.T, cond func() bool) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out waiting for condition")
}

func snapshotText(s *Session) string {
	var runes []rune
	for _, row := range s.Snapshot() {
		for _, c := range row {
			runes = append(runes, c.Ch)
		}
	}

	return string(runes)
}

func TestSessionRunsAShellAndReadsWhatItPrints(t *testing.T) {
	s, err := start(exec.Command("/bin/sh", "-c", "printf hello"), 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Kill)

	waitUntil(t, func() bool { return strings.Contains(snapshotText(s), "hello") })
}

func TestWriteSendsBytesToTheShell(t *testing.T) {
	s, err := start(exec.Command("/bin/sh", "-c", "cat"), 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Kill)

	s.Write([]byte("abc\n"))

	waitUntil(t, func() bool { return strings.Contains(snapshotText(s), "abc") })
}

func TestSessionMarksExitedOnceTheShellExits(t *testing.T) {
	s, err := start(exec.Command("/bin/sh", "-c", "exit 0"), 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Kill)

	waitUntil(t, s.Exited)
}

func TestKillMarksExitedForAShellThatWouldNotOtherwiseStop(t *testing.T) {
	s, err := start(exec.Command("/bin/sh", "-c", "sleep 30"), 80, 24)
	if err != nil {
		t.Fatal(err)
	}

	s.Kill()

	waitUntil(t, s.Exited)
}

func TestResizeChangesTheSessionsOwnIdeaOfItsSize(t *testing.T) {
	s, err := start(exec.Command("/bin/sh", "-c", "sleep 30"), 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Kill)

	s.Resize(40, 12)

	if cols, rows := s.Size(); cols != 40 || rows != 12 {
		t.Errorf("size = %dx%d, want 40x12", cols, rows)
	}
}

func TestAppCursorReflectsDECCKM(t *testing.T) {
	s, err := start(exec.Command("/bin/sh", "-c", "printf '\\033[?1h'; sleep 30"), 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Kill)

	waitUntil(t, s.AppCursor)
}

func TestEncodeSendsARuneAsItself(t *testing.T) {
	if got := string(Encode(termbox.Event{Ch: 'a'}, false)); got != "a" {
		t.Errorf("Encode = %q, want %q", got, "a")
	}
}

func TestEncodeSendsALiteralByteKeyAsThatByte(t *testing.T) {
	got := Encode(termbox.Event{Key: termbox.KeyCtrlA}, false)
	if len(got) != 1 || got[0] != 0x01 {
		t.Errorf("Encode = %v, want [0x01]", got)
	}
}

func TestEncodeSendsArrowsAsCSIByDefault(t *testing.T) {
	if got := string(Encode(termbox.Event{Key: termbox.KeyArrowUp}, false)); got != "\x1b[A" {
		t.Errorf("Encode = %q, want CSI Up", got)
	}
}

func TestEncodeSendsArrowsAsSS3UnderDECCKM(t *testing.T) {
	if got := string(Encode(termbox.Event{Key: termbox.KeyArrowUp}, true)); got != "\x1bOA" {
		t.Errorf("Encode = %q, want SS3 Up", got)
	}
}

func TestEncodeSendsPageAndFunctionKeys(t *testing.T) {
	cases := map[termbox.Key]string{
		termbox.KeyPgup:   "\x1b[5~",
		termbox.KeyPgdn:   "\x1b[6~",
		termbox.KeyInsert: "\x1b[2~",
		termbox.KeyDelete: "\x1b[3~",
		termbox.KeyF1:     "\x1bOP",
		termbox.KeyF12:    "\x1b[24~",
	}
	for key, want := range cases {
		if got := string(Encode(termbox.Event{Key: key}, false)); got != want {
			t.Errorf("Encode(%v) = %q, want %q", key, got, want)
		}
	}
}

func TestColorOfDefaultIsZero(t *testing.T) {
	if got := colorOf(vt10x.DefaultFG); got != 0 {
		t.Errorf("colorOf(DefaultFG) = %d, want 0", got)
	}
}

func TestColorOfAPaletteIndexIsOneAboveIt(t *testing.T) {
	if got := colorOf(vt10x.Color(5)); got != 6 {
		t.Errorf("colorOf(5) = %d, want 6", got)
	}
}

func TestColorOfTruecolorQuantizesToTheNearestCubeStep(t *testing.T) {
	if got, want := colorOf(vt10x.Color(255<<16)), termbox.Attribute(197); got != want {
		t.Errorf("colorOf(pure red) = %d, want %d", got, want)
	}
}
