package main

import (
	"reflect"
	"testing"

	"github.com/nsf/termbox-go"
)

func TestThemeCommandSwitchesPalette(t *testing.T) {
	inReadMode(t, "package main\n", 0, 0)

	press(t, ":theme=2\n")

	if active.name != "gruvbox" {
		t.Fatalf("theme = %q, want gruvbox", active.name)
	}
	if statusMsg != "theme=2 (gruvbox)" {
		t.Errorf("statusMsg = %q", statusMsg)
	}

	press(t, ":theme=3\n")

	if active.name != "github-dark" {
		t.Errorf("theme = %q, want github-dark", active.name)
	}

	press(t, ":theme=1\n")

	if active.name != "default" {
		t.Errorf("theme = %q, want default", active.name)
	}
}

func TestThemeCommandTakesASpaceToo(t *testing.T) {
	inReadMode(t, "package main\n", 0, 0)

	press(t, ":colo 2\n")

	if active.name != "gruvbox" {
		t.Errorf("theme = %q, want gruvbox", active.name)
	}
}

func TestThemeCommandRejectsWhatIsNotAThemeNumber(t *testing.T) {
	for _, arg := range []string{"0", "4", "x", "-1"} {
		inReadMode(t, "package main\n", 0, 0)

		press(t, ":theme="+arg+"\n")

		if active.name != "default" {
			t.Errorf("theme=%s changed the palette to %q", arg, active.name)
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

// A theme that paints its own background cannot leave a colour at zero: that
// is the terminal's own, which is exactly what such a theme is replacing.
func TestThemesPaintingTheirOwnBackgroundAreComplete(t *testing.T) {
	for _, palette := range themes {
		if palette.background == termbox.ColorDefault {
			continue
		}

		value := reflect.ValueOf(palette)
		for i := range value.NumField() {
			field := value.Type().Field(i)
			if field.Type != reflect.TypeFor[termbox.Attribute]() {
				continue
			}
			if value.Field(i).Uint() == uint64(termbox.ColorDefault) {
				t.Errorf("theme %q leaves %s unset", palette.name, field.Name)
			}
		}
	}
}
