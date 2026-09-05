package theme

import (
	"reflect"
	"testing"

	"github.com/nsf/termbox-go"
)

// A theme that paints its own background cannot leave a colour at zero: that
// is the terminal's own, which is exactly what such a theme is replacing.
func TestThemesPaintingTheirOwnBackgroundAreComplete(t *testing.T) {
	for _, palette := range Themes {
		if palette.Background == termbox.ColorDefault {
			continue
		}

		value := reflect.ValueOf(palette)
		for i := range value.NumField() {
			field := value.Type().Field(i)
			if field.Type != reflect.TypeFor[termbox.Attribute]() {
				continue
			}
			if value.Field(i).Uint() == uint64(termbox.ColorDefault) {
				t.Errorf("theme %q leaves %s unset", palette.Name, field.Name)
			}
		}
	}
}
