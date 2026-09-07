package hover

import (
	"strings"
	"testing"

	"github.com/ArditZubaku/tex/internal/layout"
)

const screenful = 80

func lines(t *testing.T, markup string, width int) string {
	t.Helper()

	return strings.Join(Lines(markup, width), "|")
}

func TestASignatureKeepsTheLineBreaksItWasWrittenWith(t *testing.T) {
	markup := "```go\nfunc Wrap(text string, width int) []string\n```"

	if got := lines(t, markup, screenful); got != "func Wrap(text string, width int) []string" {
		t.Errorf("the block reads %q", got)
	}
}

func TestProseIsJoinedAndWrappedToTheBox(t *testing.T) {
	markup := "Wrap breaks text\nonto lines that fit\na width."

	want := "Wrap breaks text onto|lines that fit a|width."
	if got := lines(t, markup, 21); got != want {
		t.Errorf("the prose reads %q, want %q", got, want)
	}
}

func TestASignatureAndItsDocCommentAreSeparatedByOneBlankLine(t *testing.T) {
	markup := "```go\nfunc Run(args []string)\n```\n\nRun is the editor.\n"

	want := "func Run(args []string)||Run is the editor."
	if got := lines(t, markup, screenful); got != want {
		t.Errorf("the box reads %q, want %q", got, want)
	}
}

func TestALineTooLongForTheBoxIsWrappedRatherThanLost(t *testing.T) {
	got := Lines("aaaa bbbb cccc dddd", 9)

	want := []string{"aaaa bbbb", "cccc dddd"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("the box reads %q, want %q", got, want)
	}
}

func TestAnAnswerLongerThanTheBoxIsCutRatherThanPushedOffTheScreen(t *testing.T) {
	var box Box
	box.Show(strings.Repeat("word ", 400), layout.Rect{Rows: 24, Cols: screenful})

	if got := len(strings.Split(box.Text(), "\n")); got != maxRows-2 {
		t.Errorf("the box holds %d lines, want %d", got, maxRows-2)
	}
}

func TestTheBoxSitsUnderTheCursorsLineWhenThereIsRoom(t *testing.T) {
	var box Box
	within := layout.Rect{Row: 1, Rows: 24, Cols: screenful}
	box.Show("Run is the editor.", within)

	frame := box.frame(within, 5, 12)
	if frame.Row != 6 {
		t.Errorf("the box starts on row %d, want 6", frame.Row)
	}
	if frame.Col != 12 {
		t.Errorf("the box starts at column %d, want 12", frame.Col)
	}
}

func TestTheBoxGoesAboveTheCursorWhenThereIsNoRoomBelow(t *testing.T) {
	var box Box
	within := layout.Rect{Row: 1, Rows: 10, Cols: screenful}
	box.Show("Run is the editor.", within)

	if frame := box.frame(within, 9, 0); frame.Row != 6 {
		t.Errorf("the box starts on row %d, want 6", frame.Row)
	}
}

func TestTheBoxIsPulledLeftToStayOnTheScreen(t *testing.T) {
	var box Box
	within := layout.Rect{Row: 1, Rows: 24, Cols: 40}
	box.Show("Run is the editor.", within)

	frame := box.frame(within, 2, 38)
	if frame.Col+frame.Cols > 40 {
		t.Errorf("the box runs from column %d for %d, past the screen's 40", frame.Col, frame.Cols)
	}
}

func TestAnAnswerOfNothingIsNoBox(t *testing.T) {
	var box Box
	box.Show("", layout.Rect{Rows: 24, Cols: screenful})

	if box.Showing() {
		t.Errorf("an empty answer put %q on the screen", box.Text())
	}
}
