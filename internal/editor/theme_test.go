package editor

import (
	"testing"

	"github.com/ArditZubaku/tex/internal/editor/edtest"
	"github.com/ArditZubaku/tex/internal/editor/state"
)

func TestThemeCommandSwitchesPalette(t *testing.T) {
	e := state.New()

	edtest.InReadMode(t, e, "package main\n", 0, 0)

	edtest.Press(t, e, ":theme=2\n")

	if e.Palette.Name != "gruvbox" {
		t.Fatalf("theme = %q, want gruvbox", e.Palette.Name)
	}
	if e.StatusMsg != "theme=2 (gruvbox)" {
		t.Errorf("statusMsg = %q", e.StatusMsg)
	}

	edtest.Press(t, e, ":theme=3\n")

	if e.Palette.Name != "github-dark" {
		t.Errorf("theme = %q, want github-dark", e.Palette.Name)
	}

	edtest.Press(t, e, ":theme=1\n")

	if e.Palette.Name != "default" {
		t.Errorf("theme = %q, want default", e.Palette.Name)
	}
}

func TestThemeCommandTakesASpaceToo(t *testing.T) {
	e := state.New()

	edtest.InReadMode(t, e, "package main\n", 0, 0)

	edtest.Press(t, e, ":colo 2\n")

	if e.Palette.Name != "gruvbox" {
		t.Errorf("theme = %q, want gruvbox", e.Palette.Name)
	}
}

func TestThemeCommandRejectsWhatIsNotAThemeNumber(t *testing.T) {
	e := state.New()

	for _, arg := range []string{"0", "4", "x", "-1"} {
		edtest.InReadMode(t, e, "package main\n", 0, 0)

		edtest.Press(t, e, ":theme="+arg+"\n")

		if e.Palette.Name != "default" {
			t.Errorf("theme=%s changed the palette to %q", arg, e.Palette.Name)
		}
		if want := "E474: Invalid argument: theme=" + arg; e.StatusMsg != want {
			t.Errorf("statusMsg = %q, want %q", e.StatusMsg, want)
		}
	}
}

func TestBareThemeCommandNamesWhatIsInUse(t *testing.T) {
	e := state.New()

	edtest.InReadMode(t, e, "package main\n", 0, 0)

	edtest.Press(t, e, ":theme\n")

	if want := "theme=1 (1=default, 2=gruvbox, 3=github-dark)"; e.StatusMsg != want {
		t.Errorf("statusMsg = %q, want %q", e.StatusMsg, want)
	}
}
