package editor

import (
	"testing"
)

func TestThemeCommandSwitchesPalette(t *testing.T) {
	inReadMode(t, "package main\n", 0, 0)

	press(t, ":theme=2\n")

	if active.Name != "gruvbox" {
		t.Fatalf("theme = %q, want gruvbox", active.Name)
	}
	if statusMsg != "theme=2 (gruvbox)" {
		t.Errorf("statusMsg = %q", statusMsg)
	}

	press(t, ":theme=3\n")

	if active.Name != "github-dark" {
		t.Errorf("theme = %q, want github-dark", active.Name)
	}

	press(t, ":theme=1\n")

	if active.Name != "default" {
		t.Errorf("theme = %q, want default", active.Name)
	}
}

func TestThemeCommandTakesASpaceToo(t *testing.T) {
	inReadMode(t, "package main\n", 0, 0)

	press(t, ":colo 2\n")

	if active.Name != "gruvbox" {
		t.Errorf("theme = %q, want gruvbox", active.Name)
	}
}

func TestThemeCommandRejectsWhatIsNotAThemeNumber(t *testing.T) {
	for _, arg := range []string{"0", "4", "x", "-1"} {
		inReadMode(t, "package main\n", 0, 0)

		press(t, ":theme="+arg+"\n")

		if active.Name != "default" {
			t.Errorf("theme=%s changed the palette to %q", arg, active.Name)
		}
		if want := "E474: Invalid argument: theme=" + arg; statusMsg != want {
			t.Errorf("statusMsg = %q, want %q", statusMsg, want)
		}
	}
}

func TestBareThemeCommandNamesWhatIsInUse(t *testing.T) {
	inReadMode(t, "package main\n", 0, 0)

	press(t, ":theme\n")

	if want := "theme=1 (1=default, 2=gruvbox, 3=github-dark)"; statusMsg != want {
		t.Errorf("statusMsg = %q, want %q", statusMsg, want)
	}
}
