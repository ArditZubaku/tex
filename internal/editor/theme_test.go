package editor

import (
	"testing"
)

func TestThemeCommandSwitchesPalette(t *testing.T) {
	inReadMode(t, "package main\n", 0, 0)

	press(t, ":theme=2\n")

	if ed.Palette.Name != "gruvbox" {
		t.Fatalf("theme = %q, want gruvbox", ed.Palette.Name)
	}
	if ed.StatusMsg != "theme=2 (gruvbox)" {
		t.Errorf("statusMsg = %q", ed.StatusMsg)
	}

	press(t, ":theme=3\n")

	if ed.Palette.Name != "github-dark" {
		t.Errorf("theme = %q, want github-dark", ed.Palette.Name)
	}

	press(t, ":theme=1\n")

	if ed.Palette.Name != "default" {
		t.Errorf("theme = %q, want default", ed.Palette.Name)
	}
}

func TestThemeCommandTakesASpaceToo(t *testing.T) {
	inReadMode(t, "package main\n", 0, 0)

	press(t, ":colo 2\n")

	if ed.Palette.Name != "gruvbox" {
		t.Errorf("theme = %q, want gruvbox", ed.Palette.Name)
	}
}

func TestThemeCommandRejectsWhatIsNotAThemeNumber(t *testing.T) {
	for _, arg := range []string{"0", "4", "x", "-1"} {
		inReadMode(t, "package main\n", 0, 0)

		press(t, ":theme="+arg+"\n")

		if ed.Palette.Name != "default" {
			t.Errorf("theme=%s changed the palette to %q", arg, ed.Palette.Name)
		}
		if want := "E474: Invalid argument: theme=" + arg; ed.StatusMsg != want {
			t.Errorf("statusMsg = %q, want %q", ed.StatusMsg, want)
		}
	}
}

func TestBareThemeCommandNamesWhatIsInUse(t *testing.T) {
	inReadMode(t, "package main\n", 0, 0)

	press(t, ":theme\n")

	if want := "theme=1 (1=default, 2=gruvbox, 3=github-dark)"; ed.StatusMsg != want {
		t.Errorf("statusMsg = %q, want %q", ed.StatusMsg, want)
	}
}
