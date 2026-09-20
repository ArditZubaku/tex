package hover

import (
	"fmt"
	"strconv"
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

// numberedLines is a fenced block, so paragraphs() keeps one line per number
// rather than joining them into prose — what a scroll test needs to tell which
// window it is looking at.
const numberedLineCount = 40

func numberedLines() string {
	var b strings.Builder
	b.WriteString("```\n")
	for i := range numberedLineCount {
		fmt.Fprintf(&b, "line%d\n", i)
	}
	b.WriteString("```\n")

	return b.String()
}

func TestAnAnswerLongerThanTheBoxShowsOnlyAWindowOfIt(t *testing.T) {
	var box Box
	box.Show(numberedLines(), layout.Rect{Rows: 24, Cols: screenful})

	got := strings.Split(box.Text(), "\n")
	if len(got) != maxRows-2 {
		t.Fatalf("the box holds %d lines, want %d", len(got), maxRows-2)
	}
	if got[0] != "line0" {
		t.Errorf("the box starts on %q, want %q", got[0], "line0")
	}
}

func TestScrollingMovesTheWindowDown(t *testing.T) {
	var box Box
	box.Show(numberedLines(), layout.Rect{Rows: 24, Cols: screenful})

	box.Scroll(3)

	got := strings.Split(box.Text(), "\n")
	if got[0] != "line3" {
		t.Errorf("the box starts on %q, want %q", got[0], "line3")
	}
}

func TestScrollingPastTheEndClampsOnTheLastLine(t *testing.T) {
	var box Box
	box.Show(numberedLines(), layout.Rect{Rows: 24, Cols: screenful})

	box.Scroll(1000)

	got := strings.Split(box.Text(), "\n")
	if want := "line" + strconv.Itoa(numberedLineCount-1); got[len(got)-1] != want {
		t.Errorf("the box ends on %q, want %q", got[len(got)-1], want)
	}
	if len(got) != maxRows-2 {
		t.Errorf("the box holds %d lines, want %d", len(got), maxRows-2)
	}
}

func TestScrollingUpPastTheTopClampsAtZero(t *testing.T) {
	var box Box
	box.Show(numberedLines(), layout.Rect{Rows: 24, Cols: screenful})

	box.Scroll(-1000)

	if got := strings.Split(box.Text(), "\n")[0]; got != "line0" {
		t.Errorf("the box starts on %q, want %q", got, "line0")
	}
}

func TestShowingANewAnswerResetsAnyScrollFromTheLastOne(t *testing.T) {
	var box Box
	box.Show(numberedLines(), layout.Rect{Rows: 24, Cols: screenful})
	box.Scroll(10)

	box.Show(numberedLines(), layout.Rect{Rows: 24, Cols: screenful})

	if got := strings.Split(box.Text(), "\n")[0]; got != "line0" {
		t.Errorf("the box starts on %q, want %q", got, "line0")
	}
}

func TestContainsIsTrueOverTheBoxAndFalseOutsideIt(t *testing.T) {
	var box Box
	within := layout.Rect{Row: 1, Rows: 24, Cols: screenful}
	box.Show("Run is the editor.", within)

	if !box.Contains(within, 5, 12, 6, 12) {
		t.Error("the box's own top-left corner reads as outside it")
	}
	if box.Contains(within, 5, 12, 0, 0) {
		t.Error("the top-left of the screen reads as inside the box")
	}
}

func TestContainsIsFalseWhenNothingIsShowing(t *testing.T) {
	var box Box
	within := layout.Rect{Row: 1, Rows: 24, Cols: screenful}

	if box.Contains(within, 5, 12, 6, 12) {
		t.Error("an empty box has somewhere the wheel can scroll it")
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
